package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"
)

type SnsMockClient struct {
	PublishedMessages []sns.PublishInput
	PublishError      error
	mu                sync.Mutex
}

const (
	testTopicArn = "arn:aws:sns:us-east-1:123456789012:test-topic"
)

func (m *SnsMockClient) Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error) {
	if m.PublishError != nil {
		return nil, m.PublishError
	}
	// check the topic arn on parms to validate the format
	topicArnRegex := `^arn:aws:sns:[a-z0-9-]+:\d{12}:[a-zA-Z0-9-_]+$`
	if params.TopicArn == nil || !regexp.MustCompile(topicArnRegex).MatchString(*params.TopicArn) {
		return nil, errors.New("invalid topic ARN")
	}

	m.mu.Lock()
	m.PublishedMessages = append(m.PublishedMessages, *params)
	m.mu.Unlock()
	return &sns.PublishOutput{
		MessageId: aws.String("mock-message-id"),
	}, nil
}

func (m *SnsMockClient) GetPublishedMessages() []sns.PublishInput {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := make([]sns.PublishInput, len(m.PublishedMessages))
	copy(msgs, m.PublishedMessages)
	return msgs
}

func (m *SnsMockClient) ClearMessages() {
	m.mu.Lock()
	m.PublishedMessages = []sns.PublishInput{}
	m.mu.Unlock()
}

func getNewSnsWebhookForTest() *SnsWebhook {
	wh, err := NewSnsWebhook("us-east-1")
	if err != nil {
		panic(err)
	}
	wh.SnsClient = &SnsMockClient{}
	return wh
}

// Helper function to create a test webhook
func createTestWebhookForSns() pusherClient.Webhook {
	return pusherClient.Webhook{
		TimeMs: int(time.Now().UnixMilli()),
		Events: []pusherClient.WebhookEvent{
			{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test data"}`,
			},
		},
	}
}

// Helper function to create a test webhook with multiple events
func createTestWebhookWithMultipleEventsForSns() pusherClient.Webhook {
	return pusherClient.Webhook{
		TimeMs: int(time.Now().UnixMilli()),
		Events: []pusherClient.WebhookEvent{
			{
				Name:    "channel_occupied",
				Channel: "test-channel-1",
				Data:    `{"message": "test data 1"}`,
			},
			{
				Name:    "channel_vacated",
				Channel: "test-channel-2",
				Data:    `{"message": "test data 2"}`,
			},
		},
	}
}

func TestNewSnsWebhook(t *testing.T) {
	t.Run("ValidRegion", func(t *testing.T) {
		webhook, err := NewSnsWebhook("us-east-1")

		assert.NoError(t, err)
		assert.NotNil(t, webhook)
		assert.Equal(t, "us-east-1", webhook.Region)
		assert.NotNil(t, webhook.SnsClient)
		assert.Equal(t, "EventName", webhook.SnsMessageEventNameKey)
		assert.Equal(t, "pusher-webhook", webhook.SnsMessageEventNameValue)
	})

	t.Run("DifferentRegions", func(t *testing.T) {
		regions := []string{"us-west-2", "eu-west-1", "ap-southeast-1"}

		for _, region := range regions {
			t.Run(region, func(t *testing.T) {
				webhook, err := NewSnsWebhook(region)

				assert.NoError(t, err)
				assert.NotNil(t, webhook)
				assert.Equal(t, region, webhook.Region)
				assert.NotNil(t, webhook.SnsClient)
			})
		}
	})

	t.Run("EmptyRegion", func(t *testing.T) {
		webhook, err := NewSnsWebhook("")

		// This might succeed or fail depending on AWS config
		// We just verify it doesn't panic
		if err != nil {
			assert.Contains(t, err.Error(), "region")
		} else {
			assert.NotNil(t, webhook)
		}
	})

	t.Run("InvalidRegion", func(t *testing.T) {
		webhook, err := NewSnsWebhook("invalid-region-123")

		// This might succeed or fail depending on AWS config
		// We just verify it doesn't panic
		if err != nil {
			assert.Contains(t, err.Error(), "region")
		} else {
			assert.NotNil(t, webhook)
		}
	})
}

func TestSnsWebhook_Send(t *testing.T) {
	t.Run("SuccessfulSend", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)

		publishedMessage := mockClient.GetPublishedMessages()[0]
		assert.Equal(t, testTopicArn, *publishedMessage.TopicArn)
		assert.NotNil(t, publishedMessage.Message)
		assert.NotNil(t, publishedMessage.MessageAttributes)

		// Verify message attributes
		eventNameAttr, exists := publishedMessage.MessageAttributes["EventName"]
		assert.True(t, exists)
		assert.Equal(t, "String", *eventNameAttr.DataType)
		assert.Equal(t, "pusher-webhook", *eventNameAttr.StringValue)

		// Verify message content
		var publishedWebhook pusherClient.Webhook
		err = json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
		assert.NoError(t, err)
		assert.Equal(t, testWebhook.TimeMs, publishedWebhook.TimeMs)
		assert.Len(t, publishedWebhook.Events, 1)
		assert.Equal(t, "channel_occupied", publishedWebhook.Events[0].Name)
	})

	t.Run("NilSnsClient", func(t *testing.T) {
		webhook := &SnsWebhook{
			SnsClient: nil,
			Region:    "us-east-1",
		}

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.Error(t, err)
		assert.Equal(t, "SNS client not initialized", err.Error())
	})

	t.Run("PublishError", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.PublishError = errors.New("SNS publish failed")

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.Error(t, err)
		assert.Equal(t, "SNS publish failed", err.Error())
	})

	t.Run("MultipleEvents", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookWithMultipleEventsForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)

		publishedMessage := mockClient.GetPublishedMessages()[0]

		// Verify message content
		var publishedWebhook pusherClient.Webhook
		err = json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
		assert.NoError(t, err)
		assert.Len(t, publishedWebhook.Events, 2)
		assert.Equal(t, "channel_occupied", publishedWebhook.Events[0].Name)
		assert.Equal(t, "channel_vacated", publishedWebhook.Events[1].Name)
	})

	t.Run("EmptyWebhook", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		emptyWebhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{},
		}

		err := webhook.Send(emptyWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)

		publishedMessage := mockClient.GetPublishedMessages()[0]

		// Verify message content
		var publishedWebhook pusherClient.Webhook
		err = json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
		assert.NoError(t, err)
		assert.Empty(t, publishedWebhook.Events)
	})

	t.Run("LargeWebhook", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		// Create a webhook with large data
		largeData := make([]byte, 10000) // 10KB of data
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		largeWebhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "large_event",
					Channel: "test-channel",
					Data:    string(largeData),
				},
			},
		}

		err := webhook.Send(largeWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)

		publishedMessage := mockClient.GetPublishedMessages()[0]

		// Verify message content
		var publishedWebhook pusherClient.Webhook
		err = json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
		assert.NoError(t, err)
		assert.Len(t, publishedWebhook.Events, 1)
		assert.Equal(t, "large_event", publishedWebhook.Events[0].Name)
		assert.Greater(t, len(publishedWebhook.Events[0].Data), 1000)
	})
}

func TestSnsWebhook_Send_MessageAttributes(t *testing.T) {
	t.Run("DefaultMessageAttributes", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.NoError(t, err)

		publishedMessage := mockClient.GetPublishedMessages()[0]
		attributes := publishedMessage.MessageAttributes

		// Verify default message attributes
		eventNameAttr, exists := attributes["EventName"]
		assert.True(t, exists)
		assert.Equal(t, "String", *eventNameAttr.DataType)
		assert.Equal(t, "pusher-webhook", *eventNameAttr.StringValue)
	})

	t.Run("CustomMessageAttributes", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		webhook.SnsMessageEventNameKey = "CustomEventName"
		webhook.SnsMessageEventNameValue = "custom-pusher-webhook"
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.NoError(t, err)

		publishedMessage := mockClient.GetPublishedMessages()[0]
		attributes := publishedMessage.MessageAttributes

		// Verify custom message attributes
		eventNameAttr, exists := attributes["CustomEventName"]
		assert.True(t, exists)
		assert.Equal(t, "String", *eventNameAttr.DataType)
		assert.Equal(t, "custom-pusher-webhook", *eventNameAttr.StringValue)
	})

	t.Run("EmptyMessageAttributeKey", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		webhook.SnsMessageEventNameKey = ""
		webhook.SnsMessageEventNameValue = snsEventKeyValue
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)
		publishedMessage := mockClient.GetPublishedMessages()[0]
		attributes := publishedMessage.MessageAttributes

		// Should not have any message attributes with empty key
		assert.NotEmpty(t, attributes)
		// Verify default message attributes are still present
		eventNameAttr, exists := attributes[snsEventKeyName]
		assert.True(t, exists)
		assert.Equal(t, "String", *eventNameAttr.DataType)
		assert.Equal(t, snsEventKeyValue, *eventNameAttr.StringValue)
	})

	t.Run("EmptyMessageAttributeValue", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		webhook.SnsMessageEventNameKey = snsEventKeyName
		webhook.SnsMessageEventNameValue = ""
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, testTopicArn)

		assert.NoError(t, err)

		publishedMessage := mockClient.GetPublishedMessages()[0]
		attributes := publishedMessage.MessageAttributes

		// Should have the key with the default value
		testKeyAttr, exists := attributes[snsEventKeyName]
		assert.True(t, exists)
		assert.Equal(t, "String", *testKeyAttr.DataType)
		assert.Equal(t, snsEventKeyValue, *testKeyAttr.StringValue)
	})
}

func TestSnsWebhook_Send_EdgeCases(t *testing.T) {
	t.Run("SpecialCharactersInData", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		specialWebhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "special_chars_event",
					Channel: "test-channel",
					Data:    `{"special": "chars: !@#$%^&*()_+-=[]{}|;':\",./<>?` + "`" + `"}`,
				},
			},
		}

		err := webhook.Send(specialWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)

		publishedMessage := mockClient.GetPublishedMessages()[0]

		// Verify message content
		var publishedWebhook pusherClient.Webhook
		err = json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
		assert.NoError(t, err)
		assert.Len(t, publishedWebhook.Events, 1)
		assert.Contains(t, publishedWebhook.Events[0].Data, "special")
		assert.Contains(t, publishedWebhook.Events[0].Data, "chars:")
	})

	t.Run("UnicodeCharacters", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		unicodeWebhook := pusherClient.Webhook{
			TimeMs: int(time.Now().UnixMilli()),
			Events: []pusherClient.WebhookEvent{
				{
					Name:    "unicode_event",
					Channel: "test-channel",
					Data:    `{"unicode": "Hello 世界 🌍 emoji test"}`,
				},
			},
		}

		err := webhook.Send(unicodeWebhook, testTopicArn)

		assert.NoError(t, err)
		assert.Len(t, mockClient.GetPublishedMessages(), 1)

		publishedMessage := mockClient.GetPublishedMessages()[0]

		// Verify message content
		var publishedWebhook pusherClient.Webhook
		err = json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
		assert.NoError(t, err)
		assert.Len(t, publishedWebhook.Events, 1)
		assert.Contains(t, publishedWebhook.Events[0].Data, "Hello 世界 🌍 emoji test")
	})

	t.Run("EmptyTopicArn", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()

		err := webhook.Send(testWebhook, "")

		// This should still succeed as AWS SNS will handle the validation
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SNS Topic ARN not specified")
		assert.Len(t, mockClient.GetPublishedMessages(), 0)
	})

	t.Run("InvalidTopicArn", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		testWebhook := createTestWebhookForSns()
		invalidTopicArn := "invalid-topic-arn"

		err := webhook.Send(testWebhook, invalidTopicArn)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid topic ARN")

		assert.Len(t, mockClient.GetPublishedMessages(), 0)
	})
}

func TestSnsWebhook_Send_Integration(t *testing.T) {
	t.Run("MultipleSends", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		// Send multiple webhooks
		for i := 0; i < 5; i++ {
			testWebhook := pusherClient.Webhook{
				TimeMs: int(time.Now().UnixMilli()),
				Events: []pusherClient.WebhookEvent{
					{
						Name:    "test_event",
						Channel: "test-channel",
						Data:    `{"index": ` + string(rune(i)) + `}`,
					},
				},
			}

			err := webhook.Send(testWebhook, testTopicArn)
			assert.NoError(t, err)
		}

		assert.Len(t, mockClient.GetPublishedMessages(), 5)

		// Verify all messages were published
		for _, publishedMessage := range mockClient.GetPublishedMessages() {
			assert.Equal(t, testTopicArn, *publishedMessage.TopicArn)

			var publishedWebhook pusherClient.Webhook
			err := json.Unmarshal([]byte(*publishedMessage.Message), &publishedWebhook)
			assert.NoError(t, err)
			assert.Len(t, publishedWebhook.Events, 1)
			assert.Equal(t, "test_event", publishedWebhook.Events[0].Name)
		}
	})

	t.Run("ConcurrentSends", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		// Send multiple webhooks concurrently
		const numGoroutines = 10
		var errs = make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				testWebhook := pusherClient.Webhook{
					TimeMs: int(time.Now().UnixMilli()),
					Events: []pusherClient.WebhookEvent{
						{
							Name:    "concurrent_event",
							Channel: "test-channel",
							Data:    `{"index": ` + string(rune(index)) + `}`,
						},
					},
				}

				err := webhook.Send(testWebhook, testTopicArn)
				errs <- err
			}(i)
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			err := <-errs
			assert.NoError(t, err)
		}

		assert.Len(t, mockClient.GetPublishedMessages(), numGoroutines)
	})

	t.Run("DifferentTopicArns", func(t *testing.T) {
		webhook := getNewSnsWebhookForTest()
		mockClient := webhook.SnsClient.(*SnsMockClient)
		mockClient.ClearMessages()

		topicArns := []string{
			"arn:aws:sns:us-east-1:123456789012:topic1",
			"arn:aws:sns:us-west-2:123456789012:topic2",
			"arn:aws:sns:eu-west-1:123456789012:topic3",
		}

		testWebhook := createTestWebhookForSns()

		for _, topicArn := range topicArns {
			err := webhook.Send(testWebhook, topicArn)
			assert.NoError(t, err)
		}

		assert.Len(t, mockClient.GetPublishedMessages(), len(topicArns))

		// Verify each message was sent to the correct topic
		for i, publishedMessage := range mockClient.GetPublishedMessages() {
			assert.Equal(t, topicArns[i], *publishedMessage.TopicArn)
		}
	})
}
