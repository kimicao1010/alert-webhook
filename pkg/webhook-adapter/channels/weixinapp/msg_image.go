package weixinapp

import "github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"

func init() {
	Payload2MsgFnMap[MsgTypeImage] = NewMsgImageFromPayload
}

type Image struct {
	MediaID string `json:"media_id"`
}

func NewMsgImage(mediaID string) *Msg {

	return &Msg{
		MsgType: MsgTypeImage,
		Image: &Image{
			MediaID: mediaID,
		},
	}
}

func NewMsgImageFromPayload(payload *models.Payload) *Msg {
	// Todo

	mediaID := ""
	return NewMsgImage(mediaID)
}
