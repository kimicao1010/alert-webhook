package senders

import (
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

type ChannelSenderCreator func(cfg map[string]string) (models.Sender, error)

var ChannelsSenderCreatorMap = map[string]ChannelSenderCreator{}

func RegisterChannelsSenderCreator(channel string, creator ChannelSenderCreator) {
	ChannelsSenderCreatorMap[channel] = creator
}
