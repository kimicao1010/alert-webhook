package feishu

import "github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"

func init() {
	Payload2MsgFnMap[MsgTypeShareChat] = NewMsgShareChatFromPayload
}

func NewMsgShareChat(shareChatID string) *Msg {
	return &Msg{
		MsgType: MsgTypeShareChat,
		Content: &Content{
			ShareChatID: shareChatID,
		},
	}
}

func NewMsgShareChatFromPayload(payload *models.Payload) *Msg {
	// todo

	shareChatID := ""
	return NewMsgShareChat(shareChatID)
}
