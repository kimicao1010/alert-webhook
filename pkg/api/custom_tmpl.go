package api

import (
	"encoding/json"
	"io"
	"net/http"

	restful "github.com/emicklei/go-restful/v3"
	promModels "github.com/kimicao1010/alert-webhook/pkg/models"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/store"
)

// customTemplatePayload 前端提交体（channel 取自路径或 body，二者一致）。
type customTemplatePayload struct {
	Content  string            `json:"content"`
	FieldMap map[string]string `json:"fieldMap"`
}

func (c *Controller) listCustomTemplates(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	if c.customTmplStore == nil {
		response.WriteHeaderAndJson(http.StatusOK, []store.CustomTemplate{}, restful.MIME_JSON)
		return
	}
	list, err := c.customTmplStore.List()
	if err != nil {
		c.logger.Warn("list custom templates failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, list, restful.MIME_JSON)
}

func (c *Controller) getCustomTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	channel := request.PathParameter("channel")
	if c.customTmplStore == nil {
		response.WriteHeaderAndJson(http.StatusNotFound, map[string]string{"error": "custom template store disabled"}, restful.MIME_JSON)
		return
	}
	ct, err := c.customTmplStore.Get(channel)
	if err != nil {
		c.logger.Warn("get custom template failed", "channel", channel, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	if ct == nil {
		response.WriteHeaderAndJson(http.StatusNotFound, map[string]string{"error": "custom template not configured for channel"}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, ct, restful.MIME_JSON)
}

func (c *Controller) saveCustomTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	channel := request.PathParameter("channel")
	if c.customTmplStore == nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": "custom template store disabled"}, restful.MIME_JSON)
		return
	}
	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	body := &customTemplatePayload{}
	if err := json.Unmarshal(raw, body); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	if body.Content == "" {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": "content must not be empty"}, restful.MIME_JSON)
		return
	}
	ct := store.CustomTemplate{
		Channel:  channel,
		Content:  body.Content,
		FieldMap: body.FieldMap,
	}
	// 保存前校验模板语法
	if err := promModels.ValidateTemplateSyntax(ct.Content); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": "template syntax error: " + err.Error()}, restful.MIME_JSON)
		return
	}
	if err := c.customTmplStore.Save(ct); err != nil {
		c.logger.Warn("save custom template failed", "channel", channel, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

func (c *Controller) deleteCustomTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	channel := request.PathParameter("channel")
	if c.customTmplStore == nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": "custom template store disabled"}, restful.MIME_JSON)
		return
	}
	if err := c.customTmplStore.Delete(channel); err != nil {
		c.logger.Warn("delete custom template failed", "channel", channel, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

// previewCustomTemplateRequest 预览请求体。
type previewCustomTemplateRequest struct {
	Content  string            `json:"content"`
	FieldMap map[string]string `json:"fieldMap"`
	RawBody  string            `json:"rawBody"` // 调用源原始 JSON（粘贴）
}

func (c *Controller) previewCustomTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	req := &previewCustomTemplateRequest{}
	if err := json.Unmarshal(raw, req); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	if err := promModels.ValidateTemplateSyntax(req.Content); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": "template syntax error: " + err.Error()}, restful.MIME_JSON)
		return
	}
	payload, err := promModels.RenderCustomTmpl(req.Content, req.FieldMap, []byte(req.RawBody))
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{
		"title":    payload.Title,
		"text":     payload.Text,
		"markdown": payload.Markdown,
	}, restful.MIME_JSON)
}