// Package store 定义存储层接口，本期使用 JSON 文件实现，
// 便于将来替换为 SQLite / MySQL 等持久化后端。
package store

// ChannelStore 渠道配置存储。
// 实现注意：channel 名称须防路径穿越（不允许包含 "/" 或 ".."）。
type ChannelStore interface {
	// List 返回全部已配置渠道名。
	List() ([]string, error)
	// Get 返回指定渠道的配置；渠道不存在时返回空 map（非错误）。
	Get(channel string) (map[string]string, error)
	// Save 保存渠道配置（创建或覆盖）。
	Save(channel string, cfg map[string]string) error
	// Delete 删除渠道配置；渠道不存在时不视为错误。
	Delete(channel string) error
}

// TemplateStore 模板存储。
// 实现注意：name 须防路径穿越。
type TemplateStore interface {
	// List 返回全部模板名。
	List() ([]string, error)
	// Get 返回指定模板内容；不存在时返回错误。
	Get(name string) (string, error)
	// Save 保存模板内容（创建或覆盖）。
	Save(name string, content string) error
	// Delete 删除模板；不存在时不视为错误。
	Delete(name string) error
}

// SendRecord 单条发送记录。
type SendRecord struct {
	// Timestamp 发送时间戳（Unix 秒）。
	Timestamp int64 `json:"timestamp"`
	// Channel 目标渠道名。
	Channel string `json:"channel"`
	// Kind 记录类型：real（真实发送，来自 /webhook/send）| test（测试发送，来自 /api/test-send）。
	Kind string `json:"kind,omitempty"`
	// Status 发送状态：success | failure。
	Status string `json:"status"`
	// Error 失败时的错误信息（成功时省略）。
	Error string `json:"error,omitempty"`
	// AlertCount 本次触发的告警条数。
	AlertCount int `json:"alertCount"`
	// Duration 发送耗时（毫秒）。
	Duration int64 `json:"durationMs"`
	// Raw 调用源原始请求体（原始 webhook JSON body），用于溯源与模板调试。
	Raw string `json:"raw,omitempty"`
	// Title 渲染后的消息标题（prom.title 段）。
	Title string `json:"title,omitempty"`
	// Text 渲染后的纯文本内容（prom.text 段）。
	Text string `json:"text,omitempty"`
	// Markdown 渲染后的 Markdown 内容（prom.markdown 段）。
	Markdown string `json:"markdown,omitempty"`
	// Failover 是否为故障转移代发（主渠道失败后由备用渠道发出）。
	Failover bool `json:"failover,omitempty"`
	// FailoverFrom 故障转移的原始渠道（主渠道）；代发记录中必填。
	FailoverFrom string `json:"failoverFrom,omitempty"`
}

// SendStore 发送记录存储。
type SendStore interface {
	// Append 追加一条发送记录（实现负责裁剪超限记录）。
	Append(r SendRecord) error
	// Query 按 offset/limit 分页查询，可按 channel/status 过滤；
	// 返回记录按时间倒序（最新在前）。
	Query(offset, limit int, channel, status string) ([]SendRecord, error)
}

// CustomTemplate 自定义模板：关联单个渠道，含字段映射（模板变量名 -> JSON 路径）。
// FieldMap 声明「模板变量名 = JSON 路径」，渲染时从原始 body 提取后以 .Custom.<var> 暴露。
type CustomTemplate struct {
	// Channel 关联渠道名（一对一：一个渠道最多一个自定义模板）。
	Channel string `json:"channel"`
	// Content 模板内容（含 {{ define "prom.title" }} 等三段定义，与内置模板同构）。
	Content string `json:"content"`
	// FieldMap 字段映射：模板变量名 -> JSON 路径（如 severity -> alerts[0].labels.severity）。
	FieldMap map[string]string `json:"fieldMap"`
}

// CustomTemplateStore 自定义模板存储。
// 实现注意：channel 须防路径穿越（不允许包含 "/" 或 ".."）。
type CustomTemplateStore interface {
	// List 返回全部已配置自定义模板（含关联渠道）。
	List() ([]CustomTemplate, error)
	// Get 返回指定渠道的自定义模板；未配置时返回 nil（非错误）。
	Get(channel string) (*CustomTemplate, error)
	// Save 保存指定渠道的自定义模板（创建或覆盖）。
	Save(t CustomTemplate) error
	// Delete 删除指定渠道的自定义模板；不存在时不视为错误。
	Delete(channel string) error
}
