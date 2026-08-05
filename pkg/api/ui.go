package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	restful "github.com/emicklei/go-restful/v3"
	promModels "github.com/kimicao1010/alert-webhook/pkg/models"
	"github.com/kimicao1010/alert-webhook/pkg/version"
	"github.com/kimicao1010/alert-webhook/web/static"
)

// InstallUI 注册 /api/* 管理接口与 /ui/ 静态页面（仅 --web-enabled 开启时由 Install 调用），
// 管理接口均需 --auth-token 认证。
func (c *Controller) InstallUI(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/api")

	ws.Route(ws.GET("/channels").To(c.listChannels))
	ws.Route(ws.GET("/channels/{channel}").To(c.getChannel))
	ws.Route(ws.POST("/channels/{channel}").To(c.saveChannel))
	ws.Route(ws.DELETE("/channels/{channel}").To(c.deleteChannel))

	ws.Route(ws.GET("/templates").To(c.listTemplates))
	ws.Route(ws.GET("/templates/{name}").To(c.getTemplate))
	ws.Route(ws.POST("/templates/{name}").To(c.saveTemplate))
	ws.Route(ws.DELETE("/templates/{name}").To(c.deleteTemplate))
	ws.Route(ws.POST("/templates/{name}/preview").To(c.previewTemplate))

	ws.Route(ws.GET("/sends").To(c.querySends))
	// 自定义模板（注意：preview 必须注册在 {channel} 之前，避免被路径参数吞掉）
	ws.Route(ws.POST("/custom-templates/preview").To(c.previewCustomTemplate))
	ws.Route(ws.GET("/custom-templates").To(c.listCustomTemplates))
	ws.Route(ws.GET("/custom-templates/{channel}").To(c.getCustomTemplate))
	ws.Route(ws.POST("/custom-templates/{channel}").To(c.saveCustomTemplate))
	ws.Route(ws.DELETE("/custom-templates/{channel}").To(c.deleteCustomTemplate))
	ws.Route(ws.POST("/test-send").To(c.testSend))
	ws.Route(ws.GET("/info").To(c.serverInfo))

	container.Add(ws)

	// /ui/ 静态页面（go:embed）
	uiws := new(restful.WebService)
	uiws.Path("/ui")
	uiws.Route(uiws.GET("/").To(c.serveIndex))
	uiws.Route(uiws.GET("/static/{subpath:*}").To(c.serveStatic))
	container.Add(uiws)
}

// serveIndex 返回 Web UI 入口页面。
func (c *Controller) serveIndex(request *restful.Request, response *restful.Response) {
	content, err := static.FS.ReadFile("index.html")
	if err != nil {
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Write(content)
}

// serveStatic 返回 /ui/static/* 下的静态资源（css/js）。
func (c *Controller) serveStatic(request *restful.Request, response *restful.Response) {
	subpath := request.PathParameter("subpath")
	if subpath == "" {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	content, err := static.FS.ReadFile(subpath)
	if err != nil {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	switch {
	case subpath == "style.css":
		response.Header().Set("Content-Type", "text/css; charset=utf-8")
	case subpath == "app.js":
		response.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	default:
		response.Header().Set("Content-Type", "application/octet-stream")
	}
	response.Write(content)
}

// requireAuth 校验 Bearer 认证；未通过时写 401 并返回 false。
func (c *Controller) requireAuth(request *restful.Request, response *restful.Response) bool {
	if c.authToken == "" {
		return true
	}
	if c.authorized(request) {
		return true
	}
	c.logger.Warn("unauthorized request", "remote", request.Request.RemoteAddr, "path", request.Request.URL.Path)
	response.WriteHeader(http.StatusUnauthorized)
	return false
}

// ---------- 渠道配置 ----------

func (c *Controller) listChannels(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	list, err := c.channelStore.List()
	if err != nil {
		c.logger.Warn("list channels failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, list, restful.MIME_JSON)
}

func (c *Controller) getChannel(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	channel := request.PathParameter("channel")
	cfg, err := c.channelStore.Get(channel)
	if err != nil {
		c.logger.Warn("get channel failed", "channel", channel, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, cfg, restful.MIME_JSON)
}

func (c *Controller) saveChannel(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	channel := request.PathParameter("channel")
	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	cfg := map[string]string{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	if err := c.channelStore.Save(channel, cfg); err != nil {
		c.logger.Warn("save channel failed", "channel", channel, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

func (c *Controller) deleteChannel(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	channel := request.PathParameter("channel")
	if err := c.channelStore.Delete(channel); err != nil {
		c.logger.Warn("delete channel failed", "channel", channel, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

// ---------- 模板 ----------

func (c *Controller) listTemplates(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	list, err := c.tmplStore.List()
	if err != nil {
		c.logger.Warn("list templates failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, list, restful.MIME_JSON)
}

func (c *Controller) getTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	name := request.PathParameter("name")
	content, err := c.tmplStore.Get(name)
	if err != nil {
		c.logger.Warn("get template failed", "name", name, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusNotFound, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"name": name, "content": content}, restful.MIME_JSON)
}

func (c *Controller) saveTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	name := request.PathParameter("name")
	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	body := map[string]string{}
	if err := json.Unmarshal(raw, &body); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	content := body["content"]
	if err := c.tmplStore.Save(name, content); err != nil {
		c.logger.Warn("save template failed", "name", name, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	// 热重载模板存储，使修改立即生效（SQLite / JSON 两模式统一从 store 重载）
	if err := promModels.LoadTemplatesFromSource(c.tmplStore, ""); err != nil {
		c.logger.Warn("reload templates failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok", "reload": "failed", "error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

func (c *Controller) deleteTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	name := request.PathParameter("name")
	if err := c.tmplStore.Delete(name); err != nil {
		c.logger.Warn("delete template failed", "name", name, "err", err.Error())
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	// 热重载模板存储（SQLite / JSON 两模式统一从 store 重载）
	if err := promModels.LoadTemplatesFromSource(c.tmplStore, ""); err != nil {
		c.logger.Warn("reload templates failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok", "reload": "failed", "error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{"status": "ok"}, restful.MIME_JSON)
}

// previewTemplateRequest 预览请求体。
type previewTemplateRequest struct {
	// Content 待渲染的模板内容（含 {{ define "prom.title" }} 等定义）。
	Content string `json:"content"`
	// Alert 用于渲染的示例 Alertmanager webhook 消息。
	Alert promModels.AlertmanagerWebhookMessage `json:"alert"`
}

// previewTemplate 渲染模板内容中的 prom.title / prom.text / prom.markdown 三段，返回给前端实时预览。
func (c *Controller) previewTemplate(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	raw, err := io.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	req := &previewTemplateRequest{}
	if err := json.Unmarshal(raw, req); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}

	// 先整体 Parse 校验语法，语法错误直接报错
	if err := promModels.ValidateTemplateSyntax(req.Content); err != nil {
		response.WriteHeaderAndJson(http.StatusBadRequest, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}

	out := map[string]string{}
	for _, section := range []string{"prom.title", "prom.text", "prom.markdown"} {
		rendered, err := promModels.RenderTemplateContent(req.Content, section, &req.Alert)
		if err != nil {
			// 模板中未定义该段（如只有 prom.title），跳过而不是报错
			continue
		}
		out[section] = rendered
	}
	response.WriteHeaderAndJson(http.StatusOK, out, restful.MIME_JSON)
}

// ---------- 发送记录 ----------

func (c *Controller) querySends(request *restful.Request, response *restful.Response) {
	if !c.requireAuth(request, response) {
		return
	}
	offset, _ := strconv.Atoi(request.QueryParameter("offset"))
	limit, _ := strconv.Atoi(request.QueryParameter("limit"))
	channel := request.QueryParameter("channel")
	status := request.QueryParameter("status")

	// 先取全量用于统计 total，再取分页
	all, err := c.sendStore.Query(0, 100000, channel, status)
	if err != nil {
		c.logger.Warn("query sends failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	records, err := c.sendStore.Query(offset, limit, channel, status)
	if err != nil {
		c.logger.Warn("query sends failed", "err", err.Error())
		response.WriteHeaderAndJson(http.StatusInternalServerError, map[string]string{"error": err.Error()}, restful.MIME_JSON)
		return
	}
	response.WriteHeaderAndJson(http.StatusOK, map[string]interface{}{
		"records": records,
		"total":   len(all),
	}, restful.MIME_JSON)
}

// errMsg 构造 JSON 错误响应（供 test.go 等复用）。
func errMsg(msg string) string {
	return fmt.Sprintf("Err: %s", msg)
}

// serverInfo 返回服务端运行信息（实际数据目录、版本），供 UI 侧栏展示。
// 该端点不要求认证：dataDir/版本属非敏感运行信息，且需保证 UI 在未配置 token 时也能展示。
func (c *Controller) serverInfo(request *restful.Request, response *restful.Response) {
	response.WriteHeaderAndJson(http.StatusOK, map[string]string{
		"dataDir": c.dataDir,
		"version": version.Version,
		"commit":  version.Commit,
	}, restful.MIME_JSON)
}
