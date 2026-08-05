package models

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/text/cases"

	"github.com/kimicao1010/alert-webhook/pkg/models/templates"
)

var (
	topLevelTemplateName = "prom"

	// store the default templates
	promMsgTemplateDefault *safeTemplate

	// store templates for different channels
	promMsgTemplatesMap = make(map[string]*safeTemplate)

	defaultFuncs = map[string]interface{}{
		"toUpper":  strings.ToUpper,
		"toLower":  strings.ToLower,
		"title":    cases.Title,
		"markdown": markdownEscapeString,
	}
	isMarkdownSpecial [128]bool
)

func init() {
	var err error

	if err = LoadDefaultTemplate(); err != nil {
		panic(err)
	}

	for _, c := range "_*`" {
		isMarkdownSpecial[c] = true
	}
}

// LoadDefaultTemplate 加载内置默认模板（全渠道共用一套），
// 初始化 promMsgTemplateDefault 与 promMsgTemplatesMap。
func LoadDefaultTemplate() error {
	defaultTmpl := templates.DefaultTmpl
	promMsgTemplateDefault = &safeTemplate{}
	if err := promMsgTemplateDefault.UpdateTemplate(defaultTmpl); err != nil {
		msg := fmt.Sprintf("UpdateTemplate for default failed, err: %s", err)
		return errors.New(msg)
	}

	// 每渠道初始都用默认模板（发送时可被自定义模板覆盖）
	for _, channel := range []string{"dingtalk", "feishu", "weixin", "weixinapp"} {
		t := &safeTemplate{}
		if err := t.UpdateTemplate(defaultTmpl); err != nil {
			msg := fmt.Sprintf("UpdateTemplate for (%s) failed, err: %s", channel, err)
			return errors.New(msg)
		}
		promMsgTemplatesMap[channel] = t
	}

	return nil
}

// LoadTemplate loads external templates from specified template dir.
func LoadTemplate(tmplDir, tmplName, tmplDefault string) error {
	// If tmplName is not empty, use the specified tmpl to update the default promMsgTemplate
	// and clear the promMsgTemplatesMap, thus will use the specified tmpl for all notification channels.
	if tmplName != "" {
		for k := range promMsgTemplatesMap {
			delete(promMsgTemplatesMap, k)
		}

		tmplFile := path.Join(tmplDir, fmt.Sprintf("%s.%s", tmplName, "tmpl"))
		b, err := os.ReadFile(tmplFile)
		if err != nil {
			msg := fmt.Sprintf("read file (%s) failed, err: %s", tmplFile, err)
			return errors.New(msg)
		}

		if err := promMsgTemplateDefault.UpdateTemplate(string(b)); err != nil {
			msg := fmt.Sprintf("UpdateTemplate for default failed, err: %s", err)
			return errors.New(msg)
		}

		return nil
	}

	var customDefaultTmpl string
	if tmplDefault != "" {
		tmplFile := path.Join(tmplDir, fmt.Sprintf("%s.%s", tmplDefault, "tmpl"))
		b, err := os.ReadFile(tmplFile)
		if err != nil {
			msg := fmt.Sprintf("read file (%s) failed, err: %s", tmplFile, err)
			return errors.New(msg)
		}
		customDefaultTmpl = string(b)
	}

	// try to find template file named "<channel>.tmpl" and update the promTemplatesMap
	for channel, t := range promMsgTemplatesMap {
		var channelTmpl string

		tmplFile := path.Join(tmplDir, fmt.Sprintf("%s.%s", channel, "tmpl"))
		b, err := os.ReadFile(tmplFile)
		if os.IsNotExist(err) {
			// case 1: <channel>.tmpl file does not exist, and not specified custom default
			// then will use the builtin default, continue the next loop
			if tmplDefault == "" {
				continue
			}
			// case 2: <channel>.tmpl file does not exist, but specified custom default
			// then will use custom default as tmpl
			channelTmpl = customDefaultTmpl
		} else {
			// case 3: <channel>.tmpl exists, but read failed, error and return
			if err != nil {
				msg := fmt.Sprintf("read file (%s) failed, err: %s", tmplFile, err)
				return errors.New(msg)
			}
			// case 4: <channel>.tmpl exists, and read succeeded, use file content as tmpl
			channelTmpl = string(b)
		}

		if err := t.UpdateTemplate(channelTmpl); err != nil {
			msg := fmt.Sprintf("UpdateTemplate for (%s) failed, err: %s", channel, err)
			return errors.New(msg)
		}
	}

	return nil
}

type safeTemplate struct {
	*template.Template
	current string
	mu      sync.RWMutex
}

func (t *safeTemplate) UpdateTemplate(newTpl string) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	tpl, err := template.New(topLevelTemplateName).
		Funcs(defaultFuncs).
		Option("missingkey=zero").
		Parse(newTpl)
	if err != nil {
		return
	}

	_ = t.current // old template
	t.Template = tpl
	t.current = newTpl
	return
}

func (t *safeTemplate) Clone() (*template.Template, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Template.Clone()
}

func markdownEscapeString(s string) string {
	b := make([]byte, 0, len(s))
	buf := bytes.NewBuffer(b)

	for _, c := range s {
		if c < 128 && isMarkdownSpecial[c] {
			buf.WriteByte('\\')
		}
		buf.WriteRune(c)
	}
	return buf.String()
}

func ExecuteTextString(text string, data interface{}) (string, error) {
	if text == "" {
		return "", nil
	}

	tmpl, err := promMsgTemplateDefault.Clone()
	if err != nil {
		return "", err
	}

	tmpl, err = tmpl.New("").Option("missingkey=zero").Parse(text)
	if err != nil {
		return "", err
	}

	// reserve a buffer in 1k
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.String(), err
}

// ValidateTemplateSyntax 校验模板内容语法是否合法（不执行），供预览前检查。
func ValidateTemplateSyntax(content string) error {
	_, err := template.New(topLevelTemplateName).
		Funcs(defaultFuncs).
		Option("missingkey=zero").
		Parse(content)
	return err
}

// RenderTemplateContent 渲染任意模板内容中的指定模板定义（如 prom.title），
// 与真实发送链路使用相同的 defaultFuncs 与 missingkey=zero 语义，
// 供 Web UI 模板预览与测试发送复用。
func RenderTemplateContent(content string, templateName string, data interface{}) (string, error) {
	tpl, err := template.New(topLevelTemplateName).
		Funcs(defaultFuncs).
		Option("missingkey=zero").
		Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TemplateSource 模板内容来源的最小抽象（List + Get），
// 由存储层结构化满足（JSONStore / sqlitestore.TemplateView），避免 models 反向依赖 store。
type TemplateSource interface {
	List() ([]string, error)
	Get(name string) (string, error)
}

// LoadTemplatesFromSource 从模板存储（而非文件系统）热重载各渠道模板。
// 语义与 LoadTemplate 的渠道循环一致：
//   - 存储中存在 <channel>.tmpl → 用其内容更新该渠道模板
//   - 存储中不存在 → 保持当前模板不变
//
// 供 Web UI 保存/删除模板后触发，SQLite 与 JSON 两种存储后端统一生效。
func LoadTemplatesFromSource(src TemplateSource) error {
	names, err := src.List()
	if err != nil {
		return fmt.Errorf("list templates from source failed, err: %s", err)
	}
	available := make(map[string]bool, len(names))
	for _, n := range names {
		available[n] = true
	}

	for channel, t := range promMsgTemplatesMap {
		name := fmt.Sprintf("%s.%s", channel, "tmpl")
		if !available[name] {
			// 该渠道无自定义模板，保持当前（内置或已加载）模板
			continue
		}
		content, err := src.Get(name)
		if err != nil {
			return fmt.Errorf("read template (%s) from source failed, err: %s", name, err)
		}
		if err := t.UpdateTemplate(content); err != nil {
			return fmt.Errorf("UpdateTemplate for (%s) failed, err: %s", channel, err)
		}
	}
	return nil
}
