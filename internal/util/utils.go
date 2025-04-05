package util

import (
	"crypto/hmac"
	"fmt"
	"math"
	"math/rand"
	"pusher/env"
	"pusher/internal/constants"
	"regexp"
	"strconv"
	"strings"
)

var (
	channelValidationRegex = regexp.MustCompile("^[-a-zA-Z0-9_=@,.;]+$")
	validAppIDRegex        = regexp.MustCompile(`^[0-9]+$`)
	maxChannelNameSize     = 164
)

// GenerateSocketID generate a new random Hash
func GenerateSocketID() constants.SocketID {
	return constants.SocketID(fmt.Sprintf("%d.%d", rand.Intn(math.MaxInt32), rand.Intn(math.MaxInt32)))
}

// IsPresenceChannel ...
func IsPresenceChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(string(channel), "presence-")
}

// IsPrivateChannel ...
func IsPrivateChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(string(channel), "private-")
}

// IsPrivateEcryptedChannel ...
func IsPrivateEncryptedChannel(channel constants.ChannelName) bool {
	return strings.HasPrefix(string(channel), "private-encrypted-")
}

// IsClientEvent ...
func IsClientEvent(event string) bool {
	return strings.HasPrefix(event, "client-")
}

// ValidChannel ...
func ValidChannel(channel constants.ChannelName) bool {
	if len(channel) > maxChannelNameSize || !channelValidationRegex.MatchString(string(channel)) {
		return false
	}
	return true
}

// ValidAppID ...
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

// Str2Int string -> int
func Str2Int(a string) (int, error) {
	b, err := strconv.Atoi(a)
	if err != nil {
		return 0, err
	}
	return b, nil
}

func ValidateChannelAuth(authToken string, socketId constants.SocketID, channel constants.ChannelName, channelData string) bool {
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
	reconstructedStringParts := []string{string(socketId), string(channel)}

	if channelData != "" {
		reconstructedStringParts = append(reconstructedStringParts, channelData)
	}

	reconstructedString := strings.Join(reconstructedStringParts, ":")
	expected := HmacSignature(reconstructedString, env.GetString("APP_SECRET", ""))

	return hmac.Equal([]byte(signature), []byte(expected))
}

func ListContains[T comparable](list []T, item T) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
