package webhooks

import (
	"pusher/internal/constants"

	pusherClient "github.com/pusher/pusher-http-go/v5"

	"pusher/log"
)

type JobData struct {
	AppKey                  string
	AppID                   constants.AppID
	payload                 pusherClient.Webhook
	originalPusherSignature string
}

type WebhookInterface interface {
	Send(webhook pusherClient.Webhook) error
}

type WebhookSender struct {
	Batch          []pusherClient.Webhook // Batch of ClientEventData to be sent as one webhook
	BatchHasLeader bool                   // Whether current process has nominated batch handler
	HttpSender     *HttpWebhook
	SNSSender      *SnsWebhook
}

type QueuedJobData struct {
	Webhook   *constants.Webhook
	Payload   *pusherClient.WebhookEvent
	AppID     constants.AppID
	AppKey    string
	AppSecret string
}

func (whs *WebhookSender) Send(data *QueuedJobData, event *pusherClient.Webhook) {
	// look through app to find the methods by which we should send the webhook
	if data == nil || data.Webhook == nil || data.Payload == nil {
		log.Logger().Errorf("No webhook data specified")
		return
	}

	if event == nil {
		log.Logger().Errorf("No webhook event specified")
		return
	}

	if data.Webhook.URL != "" {
		// send an HTTP request
		if whs.HttpSender == nil {
			log.Logger().Errorf("No HTTP sender specified")
			return
		}
		_ = whs.HttpSender.Send(*event, data.Webhook.URL, data.AppKey, data.AppSecret)
		log.Logger().Debugf("Sending webhook to %s", data.Webhook.URL)
	}
	if data.Webhook.SNSTopicARN != "" {
		// send to an SNS topic
		if whs.SNSSender == nil {
			log.Logger().Errorf("No SNS sender specified")
			return
		}
		_ = whs.SNSSender.Send(*event, data.Webhook.SNSTopicARN)
		log.Logger().Debugf("Sending webhook to %s", data.Webhook.SNSTopicARN)
	}
}

//
// func (whs *WebhookSender) SendClientEvent(app *apps.App, channel constants.ChannelName, event string, data any, socketID constants.SocketID, userID constants.UserID) {
// 	//
// }
//
// func (whs *WebhookSender) SendMemberAdded(app *apps.App, channel constants.ChannelName, userID constants.UserID) {
// 	//
// }
//
// func (whs *WebhookSender) SendMemberRemoved(app *apps.App, channel constants.ChannelName, userID constants.UserID) {
// 	//
// }
//
// func (whs *WebhookSender) SendChannelVacated(app *apps.App, channel constants.ChannelName) {
// 	//
// }
//
// func (whs *WebhookSender) SendChannelOccupied(app *apps.App, channel constants.ChannelName) {
// 	//
// }
//
// func (whs *WebhookSender) SendCacheMissed(app *apps.App, channel constants.ChannelName) {
// 	//
// }
//
// func (whs *WebhookSender) send(app *apps.App, data pusherClient.Webhook, queueName string) {
// 	//
// }
//
// func (whs *WebhookSender) sendWebhook(app *apps.App, data []pusherClient.Webhook, queueName string) {
// 	//
// }
//
// func (whs *WebhookSender) sendWebhookByBatching(app *apps.App, data pusherClient.Webhook, queueName string) {
// 	//
// }
