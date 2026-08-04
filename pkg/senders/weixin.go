package senders

import (
	"fmt"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/weixin"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

const (
	ChannelTypeWeixin = "weixin"
)

func init() {
	RegisterChannelsSenderCreator(ChannelTypeWeixin, createWeixinSender)
}

func createWeixinSender(cfg map[string]string) (models.Sender, error) {
	token := cfg["token"]
	if token == "" {
		return nil, fmt.Errorf("not token configured for weixin channel")
	}

	msgType := cfg["msg_type"]
	if msgType == "" {
		msgType = "markdown"
	}

	var sender models.Sender = weixin.NewSender(token, msgType)
	return sender, nil
}
