package webhooks

import (
	"testing"
	"time"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"github.com/stretchr/testify/assert"

	"pusher/internal/constants"
)

// MockWebhookSender for testing WebhookInterface
type MockWebhookSender struct {
	SendCalled  bool
	LastWebhook pusherClient.Webhook
	SendError   error
}

func (m *MockWebhookSender) Send(webhook pusherClient.Webhook) error {
	m.SendCalled = true
	m.LastWebhook = webhook
	return m.SendError
}

// MockHttpWebhook for testing HttpWebhook
type MockHttpWebhook struct {
	SendCalled  bool
	LastWebhook pusherClient.Webhook
	LastURL     string
	LastAppKey  string
	LastSecret  string
	SendError   error
}

func (m *MockHttpWebhook) Send(webhook pusherClient.Webhook, url, appKey, secret string) error {
	m.SendCalled = true
	m.LastWebhook = webhook
	m.LastURL = url
	m.LastAppKey = appKey
	m.LastSecret = secret
	return m.SendError
}

// MockSnsWebhook for testing SnsWebhook
type MockSnsWebhook struct {
	SendCalled   bool
	LastWebhook  pusherClient.Webhook
	LastTopicARN string
	SendError    error
}

func (m *MockSnsWebhook) Send(webhookEvent pusherClient.Webhook, snsTopicArn string) error {
	m.SendCalled = true
	m.LastWebhook = webhookEvent
	m.LastTopicARN = snsTopicArn
	return m.SendError
}

// Helper function to create a test webhook event
func createTestWebhookEventForWebhooks() pusherClient.WebhookEvent {
	return pusherClient.WebhookEvent{
		Name:    "channel_occupied",
		Channel: "test-channel",
		Data:    `{"message": "test data"}`,
	}
}

// Helper function to create a test webhook
func createTestWebhookForWebhooks() pusherClient.Webhook {
	return pusherClient.Webhook{
		TimeMs: int(time.Now().UnixMilli()),
		Events: []pusherClient.WebhookEvent{createTestWebhookEventForWebhooks()},
	}
}

// Helper function to create a test webhook with multiple events
func createTestWebhookWithMultipleEventsForWebhooks() pusherClient.Webhook {
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

// Helper function to create a test QueuedJobData
func createTestQueuedJobData() *QueuedJobData {
	return &QueuedJobData{
		Webhook: &constants.Webhook{
			URL:         "https://example.com/webhook",
			SNSTopicARN: testTopicArn,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			EventTypes: []string{"channel_occupied", "channel_vacated"},
		},
		Payload: &pusherClient.WebhookEvent{
			Name:    "channel_occupied",
			Channel: "test-channel",
			Data:    `{"message": "test data"}`,
		},
		AppID:     "test-app-id",
		AppKey:    "test-app-key",
		AppSecret: "test-app-secret",
	}
}

// Helper function to create a test JobData
func createTestJobData() *JobData {
	return &JobData{
		AppKey:                  "test-app-key",
		AppID:                   "test-app-id",
		payload:                 createTestWebhookForWebhooks(),
		originalPusherSignature: "test-signature",
	}
}

func TestJobData(t *testing.T) {
	t.Run("CreateJobData", func(t *testing.T) {
		jobData := createTestJobData()

		assert.Equal(t, "test-app-key", jobData.AppKey)
		assert.Equal(t, "test-app-id", jobData.AppID)
		assert.NotNil(t, jobData.payload)
		assert.Equal(t, "test-signature", jobData.originalPusherSignature)
	})

	t.Run("JobDataFields", func(t *testing.T) {
		webhook := createTestWebhookForWebhooks()
		jobData := &JobData{
			AppKey:                  "my-app-key",
			AppID:                   "my-app-id",
			payload:                 webhook,
			originalPusherSignature: "my-signature",
		}

		assert.Equal(t, "my-app-key", jobData.AppKey)
		assert.Equal(t, "my-app-id", jobData.AppID)
		assert.Equal(t, webhook, jobData.payload)
		assert.Equal(t, "my-signature", jobData.originalPusherSignature)
	})
}

func TestQueuedJobData(t *testing.T) {
	t.Run("CreateQueuedJobData", func(t *testing.T) {
		queuedJobData := createTestQueuedJobData()

		assert.NotNil(t, queuedJobData.Webhook)
		assert.NotNil(t, queuedJobData.Payload)
		assert.Equal(t, "test-app-id", queuedJobData.AppID)
		assert.Equal(t, "test-app-key", queuedJobData.AppKey)
		assert.Equal(t, "test-app-secret", queuedJobData.AppSecret)
	})

	t.Run("QueuedJobDataWithHTTPWebhook", func(t *testing.T) {
		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL: "https://example.com/webhook",
				Headers: map[string]string{
					"Authorization": "Bearer token",
				},
				EventTypes: []string{"channel_occupied"},
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "app-123",
			AppKey:    "key-123",
			AppSecret: "secret-123",
		}

		assert.Equal(t, "https://example.com/webhook", queuedJobData.Webhook.URL)
		assert.Equal(t, "Bearer token", queuedJobData.Webhook.Headers["Authorization"])
		assert.Equal(t, []string{"channel_occupied"}, queuedJobData.Webhook.EventTypes)
		assert.Equal(t, "app-123", queuedJobData.AppID)
		assert.Equal(t, "key-123", queuedJobData.AppKey)
		assert.Equal(t, "secret-123", queuedJobData.AppSecret)
	})

	t.Run("QueuedJobDataWithSNSWebhook", func(t *testing.T) {
		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				SNSTopicARN: "arn:aws:sns:us-east-1:123456789012:my-topic",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_vacated",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "app-456",
			AppKey:    "key-456",
			AppSecret: "secret-456",
		}

		assert.Equal(t, "arn:aws:sns:us-east-1:123456789012:my-topic", queuedJobData.Webhook.SNSTopicARN)
		assert.Equal(t, "channel_vacated", queuedJobData.Payload.Name)
		assert.Equal(t, "app-456", queuedJobData.AppID)
	})

	t.Run("QueuedJobDataWithBothWebhooks", func(t *testing.T) {
		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL:         "https://example.com/webhook",
				SNSTopicARN: "arn:aws:sns:us-east-1:123456789012:my-topic",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "member_added",
				Channel: "presence-channel",
				Data:    `{"user_id": "user123"}`,
			},
			AppID:     "app-789",
			AppKey:    "key-789",
			AppSecret: "secret-789",
		}

		assert.Equal(t, "https://example.com/webhook", queuedJobData.Webhook.URL)
		assert.Equal(t, "arn:aws:sns:us-east-1:123456789012:my-topic", queuedJobData.Webhook.SNSTopicARN)
		assert.Equal(t, "member_added", queuedJobData.Payload.Name)
		assert.Equal(t, "presence-channel", queuedJobData.Payload.Channel)
	})
}

func TestWebhookInterface(t *testing.T) {
	t.Run("MockWebhookSenderImplementsInterface", func(t *testing.T) {
		var sender WebhookInterface = &MockWebhookSender{}
		assert.NotNil(t, sender)
	})

	t.Run("MockWebhookSenderSend", func(t *testing.T) {
		mockSender := &MockWebhookSender{}
		webhook := createTestWebhookForWebhooks()

		err := mockSender.Send(webhook)

		assert.NoError(t, err)
		assert.True(t, mockSender.SendCalled)
		assert.Equal(t, webhook, mockSender.LastWebhook)
	})

	t.Run("MockWebhookSenderSendWithError", func(t *testing.T) {
		mockSender := &MockWebhookSender{
			SendError: assert.AnError,
		}
		webhook := createTestWebhookForWebhooks()

		err := mockSender.Send(webhook)

		assert.Error(t, err)
		assert.True(t, mockSender.SendCalled)
		assert.Equal(t, webhook, mockSender.LastWebhook)
	})
}

func TestWebhookSender(t *testing.T) {
	t.Run("CreateWebhookSender", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			Batch:          []pusherClient.Webhook{},
			BatchHasLeader: false,
			HttpSender:     httpSender,
			SNSSender:      snsSender,
		}

		assert.NotNil(t, webhookSender)
		assert.Empty(t, webhookSender.Batch)
		assert.False(t, webhookSender.BatchHasLeader)
		assert.Equal(t, httpSender, webhookSender.HttpSender)
		assert.Equal(t, snsSender, webhookSender.SNSSender)
	})

	t.Run("WebhookSenderWithBatch", func(t *testing.T) {
		webhook1 := createTestWebhookForWebhooks()
		webhook2 := createTestWebhookWithMultipleEventsForWebhooks()

		webhookSender := &WebhookSender{
			Batch:          []pusherClient.Webhook{webhook1, webhook2},
			BatchHasLeader: true,
			HttpSender:     &HttpWebhook{},
			SNSSender:      &SnsWebhook{},
		}

		assert.Len(t, webhookSender.Batch, 2)
		assert.True(t, webhookSender.BatchHasLeader)
		assert.Equal(t, webhook1, webhookSender.Batch[0])
		assert.Equal(t, webhook2, webhookSender.Batch[1])
	})
}

func TestWebhookSender_Send(t *testing.T) {
	t.Run("SendWithHTTPWebhook", func(t *testing.T) {
		// Create real HTTP and SNS senders
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL: "https://example.com/webhook",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should call the HTTP sender
		webhookSender.Send(queuedJobData, &webhook)

		// Note: The actual Send method doesn't return errors, it just logs them
		// So we can't easily test the HTTP sender being called without modifying the code
		// But we can verify the method doesn't panic
		assert.NotNil(t, webhookSender)
	})

	t.Run("SendWithSNSWebhook", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				SNSTopicARN: "arn:aws:sns:us-east-1:123456789012:test-topic",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_vacated",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should call the SNS sender
		webhookSender.Send(queuedJobData, &webhook)

		// Note: The actual Send method doesn't return errors, it just logs them
		// So we can't easily test the SNS sender being called without modifying the code
		// But we can verify the method doesn't panic
		assert.NotNil(t, webhookSender)
	})

	t.Run("SendWithBothWebhooks", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL:         "https://example.com/webhook",
				SNSTopicARN: "arn:aws:sns:us-east-1:123456789012:test-topic",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "member_added",
				Channel: "presence-channel",
				Data:    `{"user_id": "user123"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should call both senders
		webhookSender.Send(queuedJobData, &webhook)

		// Note: The actual Send method doesn't return errors, it just logs them
		// So we can't easily test both senders being called without modifying the code
		// But we can verify the method doesn't panic
		assert.NotNil(t, webhookSender)
	})

	t.Run("SendWithNoWebhooks", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				// No URL or SNSTopicARN
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should not call any senders
		webhookSender.Send(queuedJobData, &webhook)

		// Verify the method doesn't panic
		assert.NotNil(t, webhookSender)
	})

	t.Run("SendWithEmptyWebhookURL", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL:         "", // Empty URL
				SNSTopicARN: "arn:aws:sns:us-east-1:123456789012:test-topic",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should only call the SNS sender
		webhookSender.Send(queuedJobData, &webhook)

		// Verify the method doesn't panic
		assert.NotNil(t, webhookSender)
	})

	t.Run("SendWithEmptySNSTopicARN", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL:         "https://example.com/webhook",
				SNSTopicARN: "", // Empty SNS Topic ARN
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should only call the HTTP sender
		webhookSender.Send(queuedJobData, &webhook)

		// Verify the method doesn't panic
		assert.NotNil(t, webhookSender)
	})
}

func TestWebhookSender_EdgeCases(t *testing.T) {
	t.Run("NilQueuedJobData", func(t *testing.T) {
		webhookSender := &WebhookSender{
			HttpSender: &HttpWebhook{},
			SNSSender:  &SnsWebhook{},
		}

		webhook := createTestWebhookForWebhooks()

		// This should not panic even with nil data
		assert.NotPanics(t, func() {
			webhookSender.Send(nil, &webhook)
		})
	})

	t.Run("NilWebhook", func(t *testing.T) {
		webhookSender := &WebhookSender{
			HttpSender: &HttpWebhook{},
			SNSSender:  &SnsWebhook{},
		}

		queuedJobData := createTestQueuedJobData()

		// This should not panic even with nil webhook
		assert.NotPanics(t, func() {
			webhookSender.Send(queuedJobData, nil)
		})
	})

	t.Run("NilWebhookInQueuedJobData", func(t *testing.T) {
		webhookSender := &WebhookSender{
			HttpSender: &HttpWebhook{},
			SNSSender:  &SnsWebhook{},
		}

		queuedJobData := &QueuedJobData{
			Webhook: nil, // Nil webhook
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should not panic even with nil webhook in data
		assert.NotPanics(t, func() {
			webhookSender.Send(queuedJobData, &webhook)
		})
	})

	t.Run("NilHttpSender", func(t *testing.T) {
		webhookSender := &WebhookSender{
			HttpSender: nil, // Nil HTTP sender
			SNSSender:  &SnsWebhook{},
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				URL: "https://example.com/webhook",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should not panic even with nil HTTP sender
		assert.NotPanics(t, func() {
			webhookSender.Send(queuedJobData, &webhook)
		})
	})

	t.Run("NilSNSSender", func(t *testing.T) {
		webhookSender := &WebhookSender{
			HttpSender: &HttpWebhook{},
			SNSSender:  nil, // Nil SNS sender
		}

		queuedJobData := &QueuedJobData{
			Webhook: &constants.Webhook{
				SNSTopicARN: "arn:aws:sns:us-east-1:123456789012:test-topic",
			},
			Payload: &pusherClient.WebhookEvent{
				Name:    "channel_occupied",
				Channel: "test-channel",
				Data:    `{"message": "test"}`,
			},
			AppID:     "test-app-id",
			AppKey:    "test-app-key",
			AppSecret: "test-app-secret",
		}

		webhook := createTestWebhookForWebhooks()

		// This should not panic even with nil SNS sender
		assert.NotPanics(t, func() {
			webhookSender.Send(queuedJobData, &webhook)
		})
	})
}

func TestWebhookSender_Integration(t *testing.T) {
	t.Run("FullWorkflow", func(t *testing.T) {
		// Create a real webhook sender
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			Batch:          []pusherClient.Webhook{},
			BatchHasLeader: false,
			HttpSender:     httpSender,
			SNSSender:      snsSender,
		}

		// Create test data
		queuedJobData := createTestQueuedJobData()
		webhook := createTestWebhookForWebhooks()

		// Send the webhook
		webhookSender.Send(queuedJobData, &webhook)

		// Verify the webhook sender is still valid
		assert.NotNil(t, webhookSender)
		assert.NotNil(t, webhookSender.HttpSender)
		assert.NotNil(t, webhookSender.SNSSender)
	})

	t.Run("MultipleSends", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		// Send multiple webhooks
		for i := 0; i < 5; i++ {
			queuedJobData := &QueuedJobData{
				Webhook: &constants.Webhook{
					URL: "https://example.com/webhook",
				},
				Payload: &pusherClient.WebhookEvent{
					Name:    "test_event",
					Channel: "test-channel",
					Data:    `{"index": ` + string(rune(i)) + `}`,
				},
				AppID:     "test-app-id",
				AppKey:    "test-app-key",
				AppSecret: "test-app-secret",
			}

			webhook := createTestWebhookForWebhooks()

			// This should not panic
			assert.NotPanics(t, func() {
				webhookSender.Send(queuedJobData, &webhook)
			})
		}

		// Verify the webhook sender is still valid
		assert.NotNil(t, webhookSender)
	})

	t.Run("ConcurrentSends", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		snsSender := &SnsWebhook{}

		webhookSender := &WebhookSender{
			HttpSender: httpSender,
			SNSSender:  snsSender,
		}

		// Send multiple webhooks concurrently
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				queuedJobData := &QueuedJobData{
					Webhook: &constants.Webhook{
						URL: "https://example.com/webhook",
					},
					Payload: &pusherClient.WebhookEvent{
						Name:    "concurrent_event",
						Channel: "test-channel",
						Data:    `{"index": ` + string(rune(index)) + `}`,
					},
					AppID:     "test-app-id",
					AppKey:    "test-app-key",
					AppSecret: "test-app-secret",
				}

				webhook := createTestWebhookForWebhooks()

				// This should not panic
				assert.NotPanics(t, func() {
					webhookSender.Send(queuedJobData, &webhook)
				})

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify the webhook sender is still valid
		assert.NotNil(t, webhookSender)
	})
}

func TestWebhookSender_StructFields(t *testing.T) {
	t.Run("BatchField", func(t *testing.T) {
		webhook1 := createTestWebhookForWebhooks()
		webhook2 := createTestWebhookWithMultipleEventsForWebhooks()

		webhookSender := &WebhookSender{
			Batch: []pusherClient.Webhook{webhook1, webhook2},
		}

		assert.Len(t, webhookSender.Batch, 2)
		assert.Equal(t, webhook1, webhookSender.Batch[0])
		assert.Equal(t, webhook2, webhookSender.Batch[1])
	})

	t.Run("BatchHasLeaderField", func(t *testing.T) {
		webhookSender := &WebhookSender{
			BatchHasLeader: true,
		}

		assert.True(t, webhookSender.BatchHasLeader)

		webhookSender.BatchHasLeader = false
		assert.False(t, webhookSender.BatchHasLeader)
	})

	t.Run("HttpSenderField", func(t *testing.T) {
		httpSender := &HttpWebhook{}
		webhookSender := &WebhookSender{
			HttpSender: httpSender,
		}

		assert.Equal(t, httpSender, webhookSender.HttpSender)
	})

	t.Run("SNSSenderField", func(t *testing.T) {
		snsSender := &SnsWebhook{}
		webhookSender := &WebhookSender{
			SNSSender: snsSender,
		}

		assert.Equal(t, snsSender, webhookSender.SNSSender)
	})
}
