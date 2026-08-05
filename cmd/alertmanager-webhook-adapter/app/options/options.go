package options

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	restful "github.com/emicklei/go-restful/v3"

	"github.com/kimicao1010/alert-webhook/pkg/api"
	"github.com/kimicao1010/alert-webhook/pkg/models"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channelstore"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/customtmplstore"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/sendstore"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/sqlitestore"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/tmplstore"
)

type AppOptions struct {
	Addr        string
	Signature   string
	TmplDir     string
	TmplName    string
	TmplDefault string
	LogLevel    string
	LogFormat   string
	AuthToken   string
	DataDir     string
	WebEnabled  bool
	Version     bool
	Debug       bool
	// UseDataDir 为 true 时使用旧的 JSON 数据目录存储（调试开关，默认 false）。
	// 默认 false：所有数据写入 SQLite 单文件（DataDir/adapter.db 或 SQLitePath 指定路径）。
	UseDataDir bool
	// SQLitePath 覆盖 SQLite 库文件默认路径（空则使用 DataDir/adapter.db）。
	SQLitePath string
}

func NewAppOptions() *AppOptions {
	return &AppOptions{}
}

func (o *AppOptions) Run() error {
	execFile, err := os.Executable()
	if err != nil {
		panic("fatal")
	}

	// 默认模板已由 models 包 init() 加载（全渠道共用一套），此处无需重复加载。

	if o.TmplDir == "" && (o.TmplName != "" || o.TmplDefault != "") {
		fmt.Println("Warning, there is no meaning to specify --tmpl-name or --tmpl-default option without specify --tmpl-dir option, just ignored.")
	}

	if o.TmplDir != "" {
		if o.TmplName != "" && o.TmplDefault != "" {
			fmt.Println("Warning, there is no meaning to specify --tmpl-name and --tmpl-default options together, --tmpl-default is ignored.")
			o.TmplDefault = ""
		}

		if !filepath.IsAbs(o.TmplDir) {
			o.TmplDir = filepath.Join(filepath.Dir(execFile), o.TmplDir)
		}

		if err := models.LoadTemplate(o.TmplDir, o.TmplName, o.TmplDefault); err != nil {
			msg := fmt.Sprintf("Load templates from dir (%s) failed, err: %s", o.TmplDir, err)
			return errors.New(msg)
		}
	}

	logger := api.NewLogger(o.LogLevel, o.LogFormat)
	if o.Debug && o.LogLevel == "" {
		logger = api.NewLogger("debug", o.LogFormat)
	}

	logger.Info("signature", "value", o.Signature)
	if o.Signature == "未知" {
		logger.Warn("using the default signature, suggest to specify a custom signature by --signature option")
	}

	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	noProxy := os.Getenv("NO_PROXY")
	if httpProxy != "" || httpsProxy != "" {
		logger.Info("found http proxy from environment variables",
			"HTTP_PROXY", httpProxy,
			"HTTPS_PROXY", httpsProxy,
			"NO_PROXY", noProxy,
		)
	}

	container := restful.DefaultContainer

	controller := api.NewController(o.Signature)
	controller.WithLogger(logger)
	controller.WithDebug(o.Debug)
	controller.WithAuthToken(o.AuthToken)
	controller.WithDataDir(o.DataDir)
	controller.WithWebEnabled(o.WebEnabled)
	if o.UseDataDir {
		// 调试模式：JSON 数据目录存储（旧实现，不导入也不写入 SQLite）
		controller.WithChannelStore(channelstore.NewJSONStore(filepath.Join(o.DataDir, "channels")))
		controller.WithSendStore(sendstore.NewJSONStore(filepath.Join(o.DataDir, "sends.json"), 1000))
		tmplStore := tmplstore.NewJSONStore(filepath.Join(o.DataDir, "templates"))
		if err := tmplStore.EnsureInitialTemplates(); err != nil {
			return fmt.Errorf("ensure initial templates failed, err: %s", err)
		}
		controller.WithTmplStore(tmplStore)
		controller.WithCustomTmplStore(customtmplstore.NewJSONStore(filepath.Join(o.DataDir, "custom-templates")))
		logger.Info("storage backend", "type", "json-data-dir", "data_dir", o.DataDir)
	} else {
		// 默认：SQLite 单文件存储（不导入旧 JSON 数据）
		sqlitePath := o.SQLitePath
		if sqlitePath == "" {
			sqlitePath = filepath.Join(o.DataDir, "adapter.db")
		}
		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
			return fmt.Errorf("create sqlite dir failed, err: %s", err)
		}
		db, err := sqlitestore.Open(sqlitePath)
		if err != nil {
			return fmt.Errorf("open sqlite store failed, err: %s", err)
		}
		if err := db.Templates().EnsureInitialTemplates(); err != nil {
			return fmt.Errorf("ensure initial templates failed, err: %s", err)
		}
		controller.WithChannelStore(db.Channels())
		controller.WithSendStore(db.Sends())
		controller.WithTmplStore(db.Templates())
		controller.WithCustomTmplStore(customtmplstore.NewJSONStore(filepath.Join(o.DataDir, "custom-templates")))
		logger.Info("storage backend", "type", "sqlite", "path", sqlitePath)
	}

	logger.Info("web ui enabled", "data_dir", o.DataDir, "web_enabled", o.WebEnabled)

	controller.Install(container)

	s := &http.Server{
		Addr:    o.Addr,
		Handler: container,
	}
	logger.Info("start listening", "addr", s.Addr)
	return s.ListenAndServe()
}
