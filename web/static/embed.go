// Package static 嵌入 Web UI 静态资源（go:embed），供 /ui/ 路由使用。
package static

import "embed"

//go:embed index.html app.js style.css
var FS embed.FS
