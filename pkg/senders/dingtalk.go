package senders

import (
	"fmt"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/dingtalk"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

const (
	ChannelTypeDingtalk = "dingtalk"
)

func init() {
	RegisterChannelsSenderCreator(ChannelTypeDingtalk, createDingtalkSender)
}

func createDingtalkSender(cfg map[string]string) (models.Sender, error) {
	token := cfg["token"]
	if token == "" {
		return nil, fmt.Errorf("not token configured for dingtalk channel")
	}

	msgType := cfg["msg_type"]
	if msgType == "" {
		msgType = "markdown"
	}

	// 可选：机器人安全设置「加签」密钥
	secret := cfg["secret"]

	var sender models.Sender = dingtalk.NewSenderWithSecret(token, secret, msgType)
	return sender, nil
}
