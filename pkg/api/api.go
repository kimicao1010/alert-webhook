package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
	// failoverDisabled 为 true 时关闭渠道故障转移（主渠道失败不做备用渠道重试）。
	failoverDisabled bool
	// retryBackoffs 发送重试的固定间隔序列（默认 1s/2s/3s；测试可注入毫秒级）。
	retryBackoffs []time.Duration
}

func NewController(signature string) *Controller {
	return &Controller{
		signature:     signature,
		logger:        slog.Default(),
		dataDir:       "/data",
		retryBackoffs: []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second},
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

// WithFailoverDisabled 设置是否关闭渠道故障转移（默认 false=启用）。
func (c *Controller) WithFailoverDisabled(disabled bool) *Controller {
	c.failoverDisabled = disabled
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
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	c.logf("Received raw body: %s\n", string(raw))

	promMsg := &promModels.AlertmanagerWebhookMessage{}
	if err := json.Unmarshal(raw, promMsg); err != nil {
		errmsg := fmt.Sprintf("Err: unmarshal body failed, err: %s", err)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}
	promMsg.SetMessageAt().SetSignature(c.signature)

	channelType := request.PathParameter("channel")
	if channelType == "" {
		errmsg := "Err: no channel found in path"
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	// 渠道凭据从 channelstore 读取，URL 不再携带任何凭据
	cfg, err := c.channelStore.Get(channelType)
	if err != nil {
		errmsg := fmt.Sprintf("Err: read channel config failed, %v", err)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}

	senderCreator, exists := senders.ChannelsSenderCreatorMap[channelType]
	if !exists {
		errmsg := fmt.Sprintf("Err: not supported channel of (%s)", channelType)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	sender, err := senderCreator(cfg)
	if err != nil {
		// sender 构造失败（如无凭据）：渠道不可用，尝试故障转移；无备用则失败
		if !c.failoverDisabled {
			fallbackErr, used := c.sendWithFailover(channelType, &models.Payload{Raw: string(raw)})
			if fallbackErr == nil {
				c.logger.Info("send succeeded via failover (sender create failed)", "primary", channelType, "used", used)
				c.recordSend(channelType, "success", "", promMsg, &models.Payload{Raw: string(raw)}, 0)
				response.WriteHeader(http.StatusNoContent)
				return
			}
		}
		errmsg := fmt.Sprintf("Err: create sender failed, %v", err)
		c.log(errmsg)
		// sender 构造失败（如无凭据）也落记录，保留原始调用体便于溯源
		c.recordSend(channelType, "failure", err.Error(), promMsg, &models.Payload{Raw: string(raw)}, 0)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	// 自定义模板优先：渠道配置了自定义模板则用字段映射渲染，否则走内置模板
	payload, err := c.buildPayload(channelType, promMsg, raw)
	if err != nil {
		errmsg := fmt.Sprintf("Err: create msg payload failed, %v", err)
		c.log(errmsg)
		c.recordSend(channelType, "failure", err.Error(), promMsg, &models.Payload{Raw: string(raw)}, 0)
		_ = response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}
	if c.debug {
		pretty.Println(payload)

		c.log(">>> Payload Markdown")
		c.log(payload.Markdown)
	}

	sendStart := time.Now()
	sendErr := utils.Retry(4, []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second}, func(attempt int, err error, backoff time.Duration) {
		c.logger.Warn("send failed, retrying",
			"attempt", attempt,
			"err", err.Error(),
			"next_backoff", backoff.String(),
		)
	}, func() error {
		return sender.Send(payload)
	})
	if sendErr != nil {
		// 主渠道重试耗尽：立即尝试备用渠道（故障转移）
		if !c.failoverDisabled {
			fallbackErr, used := c.sendWithFailover(channelType, payload)
			if fallbackErr == nil {
				c.logger.Info("send succeeded via failover", "primary", channelType, "used", used)
				c.recordSend(channelType, "success", "", promMsg, payload, time.Since(sendStart))
				response.WriteHeader(http.StatusNoContent)
				return
			}
			// 全部备用渠道也失败：记录失败（含 failover exhausted）
			errmsg := fmt.Sprintf("Err: sender send failed, %v (failover exhausted: %v)", sendErr, fallbackErr)
			c.log(errmsg)
			c.recordSend(channelType, "failure", errmsg, promMsg, payload, time.Since(sendStart))
			_ = response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
			return
		}
		errmsg := fmt.Sprintf("Err: sender send failed, %v", sendErr)
		c.log(errmsg)
		c.recordSend(channelType, "failure", sendErr.Error(), promMsg, payload, time.Since(sendStart))
		_ = response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}

	c.recordSend(channelType, "success", "", promMsg, payload, time.Since(sendStart))
	c.logf("Send succeed: %s\n", request.Request.URL.String())
	response.WriteHeader(http.StatusNoContent)
}

// sendViaChannel 用指定渠道发送 payload。
// 固定间隔序列重试（默认 1s/2s/3s，总 4 次），供主渠道与备用渠道共用。
func (c *Controller) sendViaChannel(channel string, payload *models.Payload) error {
	cfg, err := c.channelStore.Get(channel)
	if err != nil {
		return fmt.Errorf("read channel config failed: %w", err)
	}
	creator, exists := senders.ChannelsSenderCreatorMap[channel]
	if !exists {
		return fmt.Errorf("not supported channel of (%s)", channel)
	}
	sender, err := creator(cfg)
	if err != nil {
		return fmt.Errorf("create sender failed: %w", err)
	}
	return utils.Retry(4, c.retryBackoffs, func(attempt int, err error, backoff time.Duration) {
		c.logger.Warn("send failed, retrying", "channel", channel, "attempt", attempt, "err", err.Error(), "next_backoff", backoff.String())
	}, func() error { return sender.Send(payload) })
}

// failoverChannels 返回除主渠道外的已配置渠道列表（按存储顺序）。
func (c *Controller) failoverChannels(primary string) []string {
	list, err := c.channelStore.List()
	if err != nil {
		return nil
	}
	res := []string{}
	for _, ch := range list {
		if ch != primary {
			res = append(res, ch)
		}
	}
	return res
}

// withFailoverNotice 返回 payload 的副本，并在 Markdown/Text 尾部追加故障转移提示，
// 使备用渠道接收方知晓消息经过代发。不修改原 payload。
func withFailoverNotice(payload *models.Payload, primary, backup string) *models.Payload {
	cp := *payload
	notice := fmt.Sprintf("\n\n> ⚠️ **故障转移**：渠道 `%s` 发送失败，本消息由备用渠道 `%s` 代发", primary, backup)
	if cp.Markdown != "" {
		cp.Markdown += notice
	}
	if cp.Text != "" {
		cp.Text += notice
	}
	return &cp
}

// sendWithFailover 遍历备用渠道逐个尝试发送，返回 (聚合错误, 实际成功渠道)。
// 任一备用渠道成功立即返回 (nil, 该渠道)；全部失败返回聚合错误。
// 每个备用渠道发送的内容均追加故障转移提示（withFailoverNotice）。
func (c *Controller) sendWithFailover(primary string, payload *models.Payload) (error, string) {
	var errs []string
	for _, ch := range c.failoverChannels(primary) {
		noticePayload := withFailoverNotice(payload, primary, ch)
		if err := c.sendViaChannel(ch, noticePayload); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ch, err))
			c.logger.Warn("failover channel failed", "channel", ch, "err", err.Error())
			continue
		}
		return nil, ch
	}
	return fmt.Errorf("all failover channels failed: %s", strings.Join(errs, "; ")), ""
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
