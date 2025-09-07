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

type SnsClientInterface interface {
	Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

type SnsWebhook struct {
	SnsClient                SnsClientInterface
	Region                   string
	SnsMessageEventNameKey   string
	SnsMessageEventNameValue string
	messageAttributes        map[string]types.MessageAttributeValue
}

const (
	snsEventKeyName  = "EventName"
	snsEventKeyValue = "pusher-webhook"
)

func NewSnsWebhook(region string) (*SnsWebhook, error) {
	snsWebhook := &SnsWebhook{
		Region: region,
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(snsWebhook.Region))
	if err != nil {
		return nil, err
	}

	snsClient := sns.NewFromConfig(cfg)
	snsWebhook.SnsClient = snsClient

	if snsWebhook.SnsMessageEventNameKey == "" {
		snsWebhook.SnsMessageEventNameKey = snsEventKeyName
	}
	if snsWebhook.SnsMessageEventNameValue == "" {
		snsWebhook.SnsMessageEventNameValue = snsEventKeyValue
	}
	return snsWebhook, nil
}

func (s *SnsWebhook) Send(webhookEvent pusher.Webhook, snsTopicArn string) error {
	if s.SnsClient == nil {
		return errors.New("SNS client not initialized")
	}

	if s.SnsMessageEventNameKey == "" {
		s.SnsMessageEventNameKey = snsEventKeyName
	}
	if s.SnsMessageEventNameValue == "" {
		s.SnsMessageEventNameValue = snsEventKeyValue
	}

	if snsTopicArn == "" {
		return errors.New("SNS Topic ARN not specified")
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
		TopicArn:          aws.String(snsTopicArn),
		MessageAttributes: attributes,
	}

	log.Logger().Debugf("Publishing message to SNS: %s", string(message))

	_, err := s.SnsClient.Publish(context.Background(), input)
	if err != nil {
		return err
	}

	log.Logger().Tracef("Published message to SNS: %s", string(message))

	return nil
}
