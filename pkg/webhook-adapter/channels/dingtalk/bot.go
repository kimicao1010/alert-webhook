package dingtalk

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/utils"
)

// DingtalkBot can send messages to dingtalk group
// ref: https://developers.dingtalk.com/document/app/message-types-and-data-format
// ref: https://open.dingtalk.com/document/robots/customize-robot-security-settings (加签)
type DingtalkGroupBot struct {
	addr         string
	access_token string
	secret       string
	client       *http.Client
}

func NewDingtalkGroupBot(access_token string) *DingtalkGroupBot {
	return NewDingtalkGroupBotWithSecret(access_token, "")
}

// NewDingtalkGroupBotWithSecret 支持机器人安全设置「加签」：secret 非空时
// 发送 URL 追加 timestamp 与 sign 参数（HMAC-SHA256 签名）。
func NewDingtalkGroupBotWithSecret(access_token string, secret string) *DingtalkGroupBot {
	return &DingtalkGroupBot{
		addr:         "https://oapi.dingtalk.com",
		access_token: access_token,
		secret:       secret,
		client:       utils.SharedClient,
	}
}

// sign 计算钉钉加签参数：HMAC-SHA256(key=secret, data=timestamp\nsecret) → base64 → urlencode。
// 注意：钉钉官方文档的 Python 示例（hmac.new(string_to_sign, digestmod=sha256)）因 Python
// hmac.new 首个参数是 key 而实际是 key=string_to_sign、data 为空，钉钉服务器端并不接受；
// 实测（2026-08）钉钉校验的是 key=secret、data=timestamp\nsecret。
func sign(secret string, timestamp string) string {
	stringToSign := timestamp + "\n" + secret
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}

// Addr 返回发送 URL；配置了 secret 时附带 timestamp/sign 加签参数。
func (bot *DingtalkGroupBot) Addr() string {
	u := fmt.Sprintf("%s/robot/send?access_token=%s", bot.addr, bot.access_token)
	if bot.secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		u = fmt.Sprintf("%s&timestamp=%s&sign=%s", u, timestamp, sign(bot.secret, timestamp))
	}
	return u
}

func (bot *DingtalkGroupBot) AddrForUpload() string {
	return fmt.Sprintf("%s/robot/upload_media?key=%s&type=file", bot.addr, bot.access_token)
}

func (bot *DingtalkGroupBot) Send(msg *Msg) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", bot.Addr(), bytes.NewBuffer(msgBytes))
	if err != nil {
		return fmt.Errorf("failed to construct request, got %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := bot.client.Do(req)
	if err != nil {
		return fmt.Errorf("send msg error, %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("send msg response error, status: %d", res.StatusCode)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read res body failed, err: %s", err)
	}

	type DingtalkResponse struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	dtRes := &DingtalkResponse{}
	if err := json.Unmarshal(resBody, dtRes); err != nil {
		return fmt.Errorf("unmarshal response body failed, err: %s, body: %s", err, string(resBody))
	}
	if dtRes.ErrCode != 0 {
		return fmt.Errorf("found err in response, errcode: %d, errmsg: %s", dtRes.ErrCode, dtRes.ErrMsg)
	}

	return nil
}

func (bot *DingtalkGroupBot) UploadFile(filename string, fileReader io.Reader) (meidaID string, err error) {

	// Todo
	// 要求文件大小在5B~20M之间

	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	fileWriter, err := w.CreateFormFile("media", filename)
	if err != nil {
		return "", fmt.Errorf("create file writer error, got %v", err)
	}
	_, _ = io.Copy(fileWriter, fileReader)

	req, err := http.NewRequest("POST", bot.AddrForUpload(), body)
	if err != nil {
		return "", fmt.Errorf("failed to construct request, got %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := bot.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send error, %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", fmt.Errorf("upload file failed, status: %d", res.StatusCode)
	}

	type ResponseBody struct {
		ErrCode   int       `json:"errcode"`
		ErrMsg    string    `json:"errmsg"`
		Type      string    `json:"type"`       // 媒体文件类型，分别有图片（image）、语音（voice）、视频（video），普通文件(file)
		MediaID   string    `json:"media_id"`   // 媒体文件上传后获取的唯一标识，3天内有效
		CreatedAt time.Time `json:"created_at"` // 媒体文件上传时间戳
	}

	r := &ResponseBody{}
	if err := json.NewDecoder(res.Body).Decode(r); err != nil {
		return "", fmt.Errorf("can decode res body, err: %v", err)
	}

	if r.ErrCode != 0 {
		return "", fmt.Errorf("upload file failed, errcode: %d, errmsg: %s", r.ErrCode, r.ErrMsg)
	}

	return r.MediaID, nil
}
