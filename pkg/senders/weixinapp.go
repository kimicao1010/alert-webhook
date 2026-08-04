package senders

import (
	"fmt"
	"strconv"

	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/weixinapp"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

const (
	ChannelTypeWeixinApp = "weixinapp"
)

func init() {
	RegisterChannelsSenderCreator(ChannelTypeWeixinApp, createWeixinappSender)
}

func createWeixinappSender(cfg map[string]string) (models.Sender, error) {
	corpID := cfg["corp_id"]
	if corpID == "" {
		return nil, fmt.Errorf("not corp_id configured for weixinapp channel")
	}

	agentID := cfg["agent_id"]
	if agentID == "" {
		return nil, fmt.Errorf("not agent_id configured for weixinapp channel")
	}

	aID, err := strconv.Atoi(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent_id must be integer")
	}

	agentSecret := cfg["agent_secret"]
	if agentSecret == "" {
		return nil, fmt.Errorf("not agent_secret configured for weixinapp channel")
	}

	toUser := cfg["to_user"]
	toParty := cfg["to_party"]
	toTag := cfg["to_tag"]

	if toUser == "" && toParty == "" && toTag == "" {
		return nil, fmt.Errorf("must specify one of to_user,to_party,to_tag")
	}

	msgType := cfg["msg_type"]
	if msgType == "" {
		msgType = "markdown"
	}

	var sender models.Sender = weixinapp.NewSender(corpID, aID, agentSecret, msgType, toUser, toParty, toTag)
	return sender, nil
}
