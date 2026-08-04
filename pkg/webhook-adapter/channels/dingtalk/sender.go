package dingtalk

import (
	"fmt"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

type Sender struct {
	bot     *DingtalkGroupBot
	msgType string
}

var _ models.Sender = (*Sender)(nil)

func NewSender(token string, msgType string) models.Sender {
	return NewSenderWithSecret(token, "", msgType)
}

// NewSenderWithSecret 支持钉钉机器人加签：secret 非空时发送 URL 带 HMAC-SHA256 签名参数。
func NewSenderWithSecret(token string, secret string, msgType string) models.Sender {
	if msgType == "" {
		msgType = MsgTypeMarkdown
	}

	return &Sender{
		bot:     NewDingtalkGroupBotWithSecret(token, secret),
		msgType: msgType,
	}
}

func (s *Sender) Send(payload *models.Payload) error {
	payload2Msg, ok := Payload2MsgFnMap[s.msgType]
	if !ok {
		return fmt.Errorf("not found dingtalk Payload2MsgFn for msg type (%s)", s.msgType)
	}
	msg := payload2Msg(payload)
	return s.SendMsg(msg)
}

func (s *Sender) SendMsg(msgSource interface{}) error {
	return s.SendMsgT(s.msgType, msgSource)
}

func (s *Sender) SendMsgT(msgType string, msgSource interface{}) error {
	msg, ok := msgSource.(*Msg)
	if !ok {
		return fmt.Errorf("passed msgSource is not type *dingtalk.Msg")
	}

	// todo, check fields of the msg according to sender's msgType
	switch msgType {
	case MsgTypeActionCard:
	case MsgTypeFeedCard:
	case MsgTypeLink:
	case MsgTypeMarkdown:
	case MsgTypeText:
	default:
		return fmt.Errorf("unsupported msgtype of (%s)", msgType)
	}

	if err := validateMsg(msgType, msg); err != nil {
		return fmt.Errorf("valid msg failed, err: %s", err)
	}

	return s.bot.Send(msg)
}
