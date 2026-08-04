package weixinapp

import "github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"

func init() {
	Payload2MsgFnMap[MsgTypeFile] = NewMsgFileFromPayload
}

type File struct {
	MediaID string `json:"media_id"`
}

func NewMsgFile(file *File) *Msg {
	return &Msg{
		MsgType: MsgTypeFile,
		File:    file,
	}
}

func NewMsgFileFromPayload(payload *models.Payload) *Msg {
	msg := &Msg{}

	return msg
}
