package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/pusher/pusher-http-go/v5"
	"pusher/log"
)

type SnsWebhook struct {
	TopicArn                 string
	snsClient                *sns.Client
	Region                   string
	SnsMessageEventNameKey   string
	SnsMessageEventNameValue string
	messageAttributes        map[string]types.MessageAttributeValue
}

func (s *SnsWebhook) InitSNS() error {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(s.Region))
	if err != nil {
		return err
	}
	snsClient := sns.NewFromConfig(cfg)
	s.snsClient = snsClient
	if s.SnsMessageEventNameKey == "" {
		s.SnsMessageEventNameKey = "EventName"
	}
	if s.SnsMessageEventNameValue == "" {
		s.SnsMessageEventNameValue = "pusher-webhook"
	}

	return nil
}

func (s *SnsWebhook) Send(webhookEvent pusher.Webhook) error {
	// TODO Implement SNS webhook sending
	if s.snsClient == nil {
		return errors.New("SNS client not initialized")
	}

	message, mErr := json.Marshal(webhookEvent)
	if mErr != nil {
		return mErr
	}

	attributes := map[string]types.MessageAttributeValue{
		s.SnsMessageEventNameKey: {
			DataType:    aws.String("String"),
			StringValue: aws.String(s.SnsMessageEventNameValue),
		},
	}

	input := &sns.PublishInput{
		Message:           aws.String(string(message)),
		TopicArn:          aws.String(s.TopicArn),
		MessageAttributes: attributes,
	}

	log.Logger().Debugf("Publishing message to SNS: %s", string(message))

	_, err := s.snsClient.Publish(context.Background(), input)
	if err != nil {
		return err
	}

	log.Logger().Tracef("Published message to SNS: %s", string(message))

	return nil
}
