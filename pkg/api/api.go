package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	restful "github.com/emicklei/go-restful/v3"
	promModels "github.com/kimicao1010/alert-webhook/pkg/models"
	"github.com/kimicao1010/alert-webhook/pkg/senders"
	models "github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/utils"
	"github.com/kr/pretty"
)

type Controller struct {
	signature       string
	debug           bool
	logger          *slog.Logger
	authToken       string
	dataDir         string
	webEnabled      bool
	channelStore    store.ChannelStore
	tmplStore       store.TemplateStore
	sendStore       store.SendStore
	customTmplStore store.CustomTemplateStore
}

func NewController(signature string) *Controller {
	return &Controller{
		signature: signature,
		logger:    slog.Default(),
		dataDir:   "/data",
	}
}

func (c *Controller) WithLogger(l *slog.Logger) *Controller {
	if l != nil {
		c.logger = l
	}
	return c
}

func (c *Controller) WithAuthToken(token string) *Controller {
	c.authToken = token
	return c
}

func (c *Controller) WithDataDir(dir string) *Controller {
	if dir != "" {
		c.dataDir = dir
	}
	return c
}

func (c *Controller) WithWebEnabled(enabled bool) *Controller {
	c.webEnabled = enabled
	return c
}

// WithCustomTmplStore 注入自定义模板存储；为 nil 时发送链路保持内置模板行为。
func (c *Controller) WithCustomTmplStore(s store.CustomTemplateStore) *Controller {
	c.customTmplStore = s
	return c
}

func (c *Controller) WithChannelStore(s store.ChannelStore) *Controller {
	c.channelStore = s
	return c
}

func (c *Controller) WithSendStore(s store.SendStore) *Controller {
	c.sendStore = s
	return c
}

func (c *Controller) WithTmplStore(s store.TemplateStore) *Controller {
	c.tmplStore = s
	return c
}

func (c *Controller) WithDebug(debug bool) *Controller {
	if debug {
		fmt.Println("debug mode enabled")
	}
	c.debug = debug
	return c
}

func (c *Controller) Install(container *restful.Container) {

	ws := new(restful.WebService)
	ws.Path("/webhook/send")

	ws.Route(
		ws.POST("/{channel}").To(c.send),
	)

	container.Add(ws)

	// Web UI 管理接口：--web-enabled 开启时注册 /api/*
	if c.webEnabled {
		c.InstallUI(container)
	}

	// 根路径健康检查端点：GET /healthz 与 /readyz，供 T023 K8s 探针使用。
	hws := new(restful.WebService)

	hws.Route(
		hws.GET("/healthz").To(func(request *restful.Request, response *restful.Response) {
			response.WriteHeader(http.StatusOK)
		}),
	)

	hws.Route(
		hws.GET("/readyz").To(func(request *restful.Request, response *restful.Response) {
			response.WriteHeader(http.StatusOK)
		}),
	)

	container.Add(hws)
}

func (c *Controller) logf(format string, a ...any) {
	if c.debug {
		c.logger.Debug(fmt.Sprintf(format, a...))
	}
}

func (c *Controller) log(a ...any) {
	if c.debug {
		c.logger.Debug(fmt.Sprint(a...))
	}
}

func (c *Controller) send(request *restful.Request, response *restful.Response) {
	c.logf("Got request : %s\n", request.Request.URL.String())

	if c.authToken != "" && !c.authorized(request) {
		c.logger.Warn("unauthorized request", "remote", request.Request.RemoteAddr, "path", request.Request.URL.Path)
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 限制请求体最大 1MB，防止超大 body 打爆内存
	request.Request.Body = http.MaxBytesReader(response, request.Request.Body, 1<<20)

	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		errmsg := fmt.Sprintf("Err: read request body failed, err: %s", err)
		c.log(errmsg)
		response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	c.logf("Received raw body: %s\n", string(raw))

	promMsg := &promModels.AlertmanagerWebhookMessage{}
	if err := json.Unmarshal(raw, promMsg); err != nil {
		errmsg := fmt.Sprintf("Err: unmarshal body failed, err: %s", err)
		c.log(errmsg)
		response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}
	promMsg.SetMessageAt().SetSignature(c.signature)

	channelType := request.PathParameter("channel")
	if channelType == "" {
		errmsg := "Err: no channel found in path"
		c.log(errmsg)
		response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	// 渠道凭据从 channelstore 读取，URL 不再携带任何凭据
	cfg, err := c.channelStore.Get(channelType)
	if err != nil {
		errmsg := fmt.Sprintf("Err: read channel config failed, %v", err)
		c.log(errmsg)
		response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}

	senderCreator, exists := senders.ChannelsSenderCreatorMap[channelType]
	if !exists {
		errmsg := fmt.Sprintf("Err: not supported channel of (%s)", channelType)
		c.log(errmsg)
		response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	sender, err := senderCreator(cfg)
	if err != nil {
		errmsg := fmt.Sprintf("Err: create sender failed, %v", err)
		c.log(errmsg)
		// sender 构造失败（如无凭据）也落记录，保留原始调用体便于溯源
		c.recordSend(channelType, "failure", err.Error(), promMsg, &models.Payload{Raw: string(raw)}, 0)
		response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	// 自定义模板优先：渠道配置了自定义模板则用字段映射渲染，否则走内置模板
	payload, err := c.buildPayload(channelType, promMsg, raw)
	if err != nil {
		errmsg := fmt.Sprintf("Err: create msg payload failed, %v", err)
		c.log(errmsg)
		c.recordSend(channelType, "failure", err.Error(), promMsg, &models.Payload{Raw: string(raw)}, 0)
		response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}
	if c.debug {
		pretty.Println(payload)

		c.log(">>> Payload Markdown")
		c.log(payload.Markdown)
	}

	sendStart := time.Now()
	sendErr := utils.Retry(4, 1*time.Second, func(attempt int, err error, backoff time.Duration) {
		c.logger.Warn("send failed, retrying",
			"attempt", attempt,
			"err", err.Error(),
			"next_backoff", backoff.String(),
		)
	}, func() error {
		return sender.Send(payload)
	})
	if sendErr != nil {
		errmsg := fmt.Sprintf("Err: sender send failed, %v", sendErr)
		c.log(errmsg)
		c.recordSend(channelType, "failure", sendErr.Error(), promMsg, payload, time.Since(sendStart))
		response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}

	c.recordSend(channelType, "success", "", promMsg, payload, time.Since(sendStart))
	c.logf("Send succeed: %s\n", request.Request.URL.String())
	response.WriteHeader(http.StatusNoContent)
}

// buildPayload 构造发送 payload：渠道有自定义模板时用字段映射渲染，否则用内置模板。
func (c *Controller) buildPayload(channel string, promMsg *promModels.AlertmanagerWebhookMessage, raw []byte) (*models.Payload, error) {
	if c.customTmplStore != nil {
		if ct, err := c.customTmplStore.Get(channel); err != nil {
			return nil, fmt.Errorf("read custom template failed: %w", err)
		} else if ct != nil {
			return promModels.RenderCustomTmpl(ct.Content, ct.FieldMap, raw)
		}
	}
	return promMsg.ToPayload(channel, raw)
}

// recordSend 将发送结果写入 sendstore（成功/失败均记录），
// 并附带发送内容快照（原始调用体 + 渲染后的 title/text/markdown），便于溯源与模板调试。
func (c *Controller) recordSend(channel string, status string, errMsg string, msg *promModels.AlertmanagerWebhookMessage, payload *models.Payload, duration time.Duration) {
	if c.sendStore == nil {
		return
	}
	rec := store.SendRecord{
		Timestamp:  time.Now().Unix(),
		Channel:    channel,
		Kind:       "real",
		Status:     status,
		Error:      errMsg,
		AlertCount: len(msg.Alerts),
		Duration:   duration.Milliseconds(),
	}
	if payload != nil {
		rec.Raw = payload.Raw
		rec.Title = payload.Title
		rec.Text = payload.Text
		rec.Markdown = payload.Markdown
	}
	if err := c.sendStore.Append(rec); err != nil {
		c.logger.Warn("record send failed", "err", err.Error())
	}
}

// authorized 校验请求头的 Authorization: Bearer <token> 是否匹配 --auth-token。
func (c *Controller) authorized(request *restful.Request) bool {
	h := request.Request.Header.Get("Authorization")
	return h == "Bearer "+c.authToken
}
