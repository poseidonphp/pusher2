package webhooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	pusherClient "github.com/pusher/pusher-http-go/v5"
	"pusher/internal/util"
	"pusher/log"
)

type HttpWebhook struct {
	WebhookUrl string
}

func (h *HttpWebhook) Send(webhook pusherClient.Webhook, url string, appKey string, appSecret string) error {

	if url == "" {
		log.Logger().Debugf("No webhook URL specified")
		return nil
	}

	// Send the webhook to the specified URL
	data, mErr := json.Marshal(webhook)
	if mErr != nil {
		log.Logger().Debugf("Error marshalling payload: %s", mErr)
		return mErr
	}
	// create a POST request and send the data to the webhook URL
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Logger().Debugf("Error creating request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pusher-Key", appKey)
	req.Header.Set("X-Pusher-Signature", util.HmacSignature(string(data), appSecret))

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Logger().Debugf("Error sending request: %s", err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Logger().Errorf("Error closing response body: %s", err)
		}
	}(resp.Body)

	// handle the response
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	} else {
		log.Logger().Debugf("Success sending webhook: %s", resp.Status)
	}
	return nil
}
