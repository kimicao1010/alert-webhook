package feishu

import "github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"

func init() {
	Payload2MsgFnMap[MsgTypeText] = NewMsgTextFromPayload
}

func NewMsgText(text string) *Msg {
	return &Msg{
		MsgType: MsgTypeText,
		Content: &Content{
			Text: text,
		},
	}
}

func NewMsgTextFromPayload(payload *models.Payload) *Msg {
	return NewMsgText(payload.Text)
}
