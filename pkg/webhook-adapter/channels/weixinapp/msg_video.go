package weixinapp

import "github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"

func init() {
	Payload2MsgFnMap[MsgTypeVideo] = NewMsgVideoFromPayload
}

type Video struct {
	MediaID     string `json:"media_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Todo
func NewMsgVideoFromPayload(payload *models.Payload) *Msg {
	return &Msg{}
}
