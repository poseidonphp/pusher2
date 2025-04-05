package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const AuthToken = "appKey:23d0e15319e1147630bf07eaf806c04a09d503e2b8ccc7b24d3dc087c5c5d994"
const APPKEY = "appKey"
const SocketID = "105556947.10598081"
const ChannelData string = "{\"user_id\":\"7329\",\"user_info\":{\"email\":\"user7329@example.com\",\"id\":\"7329\",\"name\":\"User 7329\"}}"

func main() {
	if check() {
		fmt.Println("Valid")
	} else {
		fmt.Println("Invalid")
	}
}

func check() bool {
	encodedString := createEncodedString()
	fmt.Println("Encoded from raw: ", encodedString)

	// split the auth token by the colon
	authParts := strings.Split(APPKEY+":"+encodedString, ":")
	if len(authParts) != 2 {
		return false
	}

	key := authParts[0]
	signature := authParts[1]
	if key != APPKEY {
		return false
	}
	isValid := checkSignature(signature)

	return isValid
}

func createEncodedString() string {
	secret := "SuperSecret"
	strToEncode := SocketID + ":presence-users" + ":" + ChannelData
	encodedString := hmacSignature(strToEncode, secret)
	return encodedString
}

func checkSignature(encodedString string) bool {
	// reconstruct the string that should have been used to authorize, using data from the request
	secret := "SuperSecret"
	rawData := strings.Join([]string{SocketID, "presence-users", ChannelData}, ":")

	expectedSignature := hmacSignature(rawData, secret)
	return hmac.Equal([]byte(encodedString), []byte(expectedSignature))
}

func hmacSignature(toSign, secret string) string {
	return hex.EncodeToString(hmacBytes([]byte(toSign), []byte(secret)))
}
func hmacBytes(toSign, secret []byte) []byte {
	_authSignature := hmac.New(sha256.New, secret)
	_authSignature.Write(toSign)
	return _authSignature.Sum(nil)
}
