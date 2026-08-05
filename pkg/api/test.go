package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	restful "github.com/emicklei/go-restful/v3"
	promModels "github.com/kimicao1010/alert-webhook/pkg/models"
	"github.com/kimicao1010/alert-webhook/pkg/senders"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// testSendRequest 测试发送请求体。
type testSendRequest struct {
	// Channel 目标渠道名（channelstore 中已配置）。
	Channel string `json:"channel"`
	// Text 测试消息文本（template 为空时使用）。
	Text string `json:"text"`
	// Template 关联的模板名（如 feishu.zh.tmpl），非空时用模板渲染告警内容发送。
	Template string `json:"template"`
	// Fields 模板字段表单值（构造 AlertmanagerWebhookMessage 渲染用）。
	Fields map[string]string `json:"fields"`
	// Labels 自定义告警标签 KV（追加到 alerts[0].labels 与 commonLabels）。
	Labels map[string]string `json:"labels"`
}

// testSend 复用与真实发送相同的 sender 链路，向指定渠道发一条测试消息，
// 用于在 Web UI 中验证渠道配置是否有效。
func (c *Controller) testSend(request *restful.Request, response *restful.Response) {
	if c.authToken != "" && !c.authorized(request) {
		c.logger.Warn("unauthorized request", "remote", request.Request.RemoteAddr, "path", request.Request.URL.Path)
		response.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 限制请求体大小
	request.Request.Body = http.MaxBytesReader(response, request.Request.Body, 1<<20)

	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		errmsg := fmt.Sprintf("Err: read request body failed, err: %s", err)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	req := &testSendRequest{}
	if err := json.Unmarshal(raw, req); err != nil {
		errmsg := fmt.Sprintf("Err: unmarshal body failed, err: %s", err)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}
	if req.Channel == "" {
		errmsg := "Err: no channel specified"
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	// 从 channelstore 读渠道配置
	cfg, err := c.channelStore.Get(req.Channel)
	if err != nil {
		errmsg := fmt.Sprintf("Err: read channel config failed, %v", err)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}

	senderCreator, exists := senders.ChannelsSenderCreatorMap[req.Channel]
	if !exists {
		errmsg := fmt.Sprintf("Err: not supported channel of (%s)", req.Channel)
		c.log(errmsg)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	sender, err := senderCreator(cfg)
	if err != nil {
		errmsg := fmt.Sprintf("Err: create sender failed, %v", err)
		c.log(errmsg)
		// sender 构造失败（如无凭据）也落记录，保留原始请求体便于溯源
		c.recordTestSend(req.Channel, "failure", err.Error(), &models.Payload{Raw: string(raw)}, 0)
		_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
		return
	}

	// 构造测试 payload
	var payload *models.Payload
	if req.Template != "" {
		// 关联模板模式：校验模板渠道约束 + 用字段渲染模板
		payload, err = c.buildPayloadFromTemplate(req)
		if err != nil {
			errmsg := fmt.Sprintf("Err: build payload from template failed, %v", err)
			c.log(errmsg)
			_ = response.WriteHeaderAndJson(http.StatusBadRequest, errmsg, restful.MIME_JSON)
			return
		}
	} else {
		// 简单文本模式
		text := req.Text
		if text == "" {
			text = "alertmanager-webhook-adapter test message"
		}
		payload = &models.Payload{
			Title:    "测试消息",
			Text:     text,
			Markdown: text,
		}
	}
	// 记录调用源数据：原始测试发送请求体（含 channel/template/fields/labels），便于溯源与模板调试
	payload.Raw = string(raw)

	start := time.Now()
	if err := sender.Send(payload); err != nil {
		errmsg := fmt.Sprintf("Err: test send failed, %v", err)
		c.log(errmsg)
		c.logger.Warn("test send failed", "channel", req.Channel, "duration_ms", time.Since(start).Milliseconds(), "err", err.Error())
		c.recordTestSend(req.Channel, "failure", err.Error(), payload, time.Since(start))
		_ = response.WriteHeaderAndJson(http.StatusInternalServerError, errmsg, restful.MIME_JSON)
		return
	}

	c.logger.Info("test send succeeded", "channel", req.Channel, "duration_ms", time.Since(start).Milliseconds())
	c.recordTestSend(req.Channel, "success", "", payload, time.Since(start))
	_ = response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

// buildPayloadFromTemplate 用指定模板 + 字段表单构造告警数据，渲染出 title/text/markdown 三段 payload。
// 模板名必须与发送渠道匹配（模板文件名以 <channel>. 开头）。
func (c *Controller) buildPayloadFromTemplate(req *testSendRequest) (*models.Payload, error) {
	// 模板渠道约束：<channel>.tmpl / <channel>.<lang>.tmpl
	if !strings.HasPrefix(req.Template, req.Channel+".") {
		return nil, fmt.Errorf("template %q does not match channel %q", req.Template, req.Channel)
	}

	content, err := c.tmplStore.Get(req.Template)
	if err != nil {
		return nil, fmt.Errorf("read template %q failed: %w", req.Template, err)
	}

	// 构造 AlertmanagerWebhookMessage 告警数据
	msg := c.buildAlertMessageFromFields(req)

	payload := &models.Payload{}
	for _, section := range []struct {
		name string
		dst  *string
	}{
		{"prom.title", &payload.Title},
		{"prom.text", &payload.Text},
		{"prom.markdown", &payload.Markdown},
	} {
		rendered, err := promModels.RenderTemplateContent(content, section.name, msg)
		if err != nil {
			continue // 模板未定义该段则跳过
		}
		*section.dst = rendered
	}
	return payload, nil
}

// buildAlertMessageFromFields 用字段表单 + 自定义标签构造 AlertmanagerWebhookMessage。
// 预定义字段：status/alertname/severity/instance/summary；其余字段与 labels 合并为告警标签。
func (c *Controller) buildAlertMessageFromFields(req *testSendRequest) *promModels.AlertmanagerWebhookMessage {
	fields := req.Fields
	if fields == nil {
		fields = map[string]string{}
	}

	status := fields["status"]
	if status == "" {
		status = "firing"
	}

	alertLabels := promModels.KV{}
	commonLabels := promModels.KV{}
	for k, v := range req.Labels {
		alertLabels[k] = v
		commonLabels[k] = v
	}
	// 预定义字段映射到标签/注解
	for _, k := range []string{"alertname", "severity", "instance"} {
		if v := fields[k]; v != "" {
			alertLabels[k] = v
			commonLabels[k] = v
		}
	}
	annotations := promModels.KV{}
	if v := fields["summary"]; v != "" {
		annotations["summary"] = v
	}

	alert := promModels.Alert{
		Status:      status,
		Labels:      alertLabels,
		Annotations: annotations,
	}

	msg := &promModels.AlertmanagerWebhookMessage{
		Status:            status,
		Receiver:          req.Channel,
		Alerts:            promModels.Alerts{alert},
		CommonLabels:      commonLabels,
		CommonAnnotations: annotations,
		Signature:         c.signature,
		ExternalURL:       fields["externalURL"],
	}
	return msg
}

// recordTestSend 将测试发送结果写入 sendstore（记录真实渠道名，Kind 标记为 "test" 与真实发送区分），
// 并附带发送内容快照（原始请求体 + 渲染后的 title/text/markdown），便于溯源与模板调试。
func (c *Controller) recordTestSend(channel string, status string, errMsg string, payload *models.Payload, duration time.Duration) {
	if c.sendStore == nil {
		return
	}
	rec := store.SendRecord{
		Timestamp: time.Now().Unix(),
		Channel:   channel,
		Kind:      "test",
		Status:    status,
		Error:     errMsg,
		Duration:  duration.Milliseconds(),
	}
	if payload != nil {
		rec.Raw = payload.Raw
		rec.Title = payload.Title
		rec.Text = payload.Text
		rec.Markdown = payload.Markdown
	}
	if err := c.sendStore.Append(rec); err != nil {
		c.logger.Warn("record test send failed", "err", err.Error())
	}
}
