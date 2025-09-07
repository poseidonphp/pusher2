package constants

type WebhookFilters struct {
	ChannelNameStartsWith string
	ChannelNameEndsWith   string
}

type Webhook struct {
	URL            string
	Headers        map[string]string
	EventTypes     []string
	Filter         WebhookFilters
	LambdaFunction string
	Lambda         struct {
		Async         bool
		Region        string
		ClientOptions string // replace later with lambda types client configuration
	}
	SNSTopicARN string
}
