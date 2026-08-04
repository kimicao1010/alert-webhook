package senders

import (
	"fmt"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/feishu"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

const (
	ChannelTypeFeishu = "feishu"
)

func init() {
	RegisterChannelsSenderCreator(ChannelTypeFeishu, createFeishuSender)
}

func createFeishuSender(cfg map[string]string) (models.Sender, error) {
	token := cfg["token"]
	if token == "" {
		return nil, fmt.Errorf("not token configured for feishu channel")
	}

	msgType := cfg["msg_type"]
	if msgType == "" {
		msgType = "markdown"
	}

	var sender models.Sender = feishu.NewSender(token, msgType)
	return sender, nil
}
