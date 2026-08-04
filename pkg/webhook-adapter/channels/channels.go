package channels

import (
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/dingtalk"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/feishu"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/weixin"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/channels/weixinapp"
	"github.com/kimicao1010/alert-webhook/pkg/webhook-adapter/models"
)

func NewDingtalkSender(token string, msgType string) models.Sender {
	return dingtalk.NewSender(token, msgType)
}

func NewFeishuSender(token string, msgType string) models.Sender {
	return feishu.NewSender(token, msgType)
}

func NewWeixinSender(token string, msgType string) models.Sender {
	return weixin.NewSender(token, msgType)
}

func NewWeixinAppSender(corpID string, agentID int, agentSecret string, msgType string, toUser string, toParty string, toTag string) models.Sender {
	return weixinapp.NewSender(corpID, agentID, agentSecret, msgType, toUser, toParty, toTag)
}
