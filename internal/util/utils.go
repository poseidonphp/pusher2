package util

import (
	"crypto/hmac"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"

	"pusher/internal/constants"
)

var (
	channelValidationRegex = regexp.MustCompile("^(#server-to-user-)?[-a-zA-Z0-9_=@,.;]+$")
	validAppIDRegex        = regexp.MustCompile(`^[0-9]+$`)
)

// FileExists checks if a file exists at the given path and returns a boolean
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// GenerateSocketID generate a new random Hash
func GenerateSocketID() constants.SocketID {
	return fmt.Sprintf("%d.%d", rand.Intn(math.MaxInt32), rand.Intn(math.MaxInt32))
}

// IsPresenceChannel determines if the channel is a presence channel by looking for specific prefixes
func IsPresenceChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(channel, "presence-") || strings.HasPrefix(channel, "presence-cache-")
}

// IsPrivateChannel determines if the channel is a private channel by looking for specific prefixes
func IsPrivateChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(channel, "private-") || strings.HasPrefix(channel, "private-cache-") || strings.HasPrefix(channel, "#server-to-user-")
}

// IsPrivateEncryptedChannel looks for private-encrypted- or private-encrypted-cache- prefixes
func IsPrivateEncryptedChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(channel, "private-encrypted-") || strings.HasPrefix(channel, "private-encrypted-cache-")
}

// IsCacheChannel determines if the channel is a cache channel by looking for specific prefixes
func IsCacheChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(channel, "cache-") || strings.HasPrefix(channel, "presence-cache-") || strings.HasPrefix(channel, "private-cache-") || strings.HasPrefix(channel, "private-encrypted-cache-")
}

// IsClientEvent checks the event name to see if it is a client event
func IsClientEvent(event string) bool {
	return strings.HasPrefix(event, "client-")
}

// ValidChannel is used by http api to validate channel names
func ValidChannel(channel constants.ChannelName, maxChannelNameLength int) bool {
	if len(channel) > maxChannelNameLength || !channelValidationRegex.MatchString(channel) {
		return false
	}
	return true
}

// ValidateChannelName is used by Websocket subscriptions channel name validation and provides more friendly error messages than ValidChannel()
func ValidateChannelName(channel constants.ChannelName, maxChannelNameLength int) error {
	if len(channel) > maxChannelNameLength {
		return fmt.Errorf("channel name too long, max length is %d", maxChannelNameLength)
	}

	if !channelValidationRegex.MatchString(channel) {
		return fmt.Errorf("invalid characters in channel name")
	}

	return nil
}

// ValidAppID matches the app id against a regex to ensure it is a valid integer
func ValidAppID(appID string) bool {
	return validAppIDRegex.MatchString(appID)
}

// Str2Int64 string -> int64
func Str2Int64(a string) (int64, error) {
	b, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return 0, err
	}
	return b, nil
}

// Str2Int converts a string to an integer
func Str2Int(a string) (int, error) {
	b, err := strconv.Atoi(a)
	if err != nil {
		return 0, err
	}
	return b, nil
}

func ValidateChannelAuth(authToken string, appSecret string, socketId constants.SocketID, channel constants.ChannelName, channelData string) bool {
	if authToken == "" {
		return false
	}

	// split the auth token by the colon
	authParts := strings.Split(authToken, ":")
	if len(authParts) != 2 {
		return false
	}

	key := authParts[0]
	signature := authParts[1]

	if key == "" {
		return false
	}

	// Reconstruct the string that should have been used to authorize, using data from the request
	reconstructedStringParts := []string{string(socketId), channel}

	if channelData != "" {
		reconstructedStringParts = append(reconstructedStringParts, channelData)
	}

	reconstructedString := strings.Join(reconstructedStringParts, ":")
	expected := HmacSignature(reconstructedString, appSecret)

	return hmac.Equal([]byte(signature), []byte(expected))
}
