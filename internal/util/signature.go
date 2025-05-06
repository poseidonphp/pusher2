package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"pusher/log"
)

// Verify signature
// @doc https://pusher.com/docs/channels/library_auth_reference/rest-api#authentication
func Verify(r *http.Request, appID string, appSecret string) (bool, error) {
	query := r.URL.Query()
	log.Logger().Debugf("Verifying appId: %s", appID)
	if !checkVersion(query.Get("auth_version")) {
		return false, errors.New("invalid version")
	}

	// queryString := prepareQueryString(query)
	// stringToSign := strings.Join([]string{strings.ToUpper(r.Method), r.URL.Path, queryString}, "\n")
	// fmt.Println("String to sign: ", stringToSign)

	timestamp, _ := Str2Int64(query.Get("auth_timestamp"))
	if !checkTimestamp(timestamp, 600) {
		return false, errors.New("invalid timestamp")
	}

	signature := query.Get("auth_signature")
	query.Del("auth_signature")
	queryString := prepareQueryString(query)
	stringToSign := strings.Join([]string{strings.ToUpper(r.Method), r.URL.Path, queryString}, "\n")
	signatureIsValid := HmacSignature(stringToSign, appSecret) == signature
	if !signatureIsValid {
		log.Logger().Debugf("Signature mismatch: %s != %s", HmacSignature(stringToSign, appSecret), signature)
		return false, errors.New("invalid signature")
	}
	return true, nil
}

func checkVersion(version string) bool {
	return version == "1.0"
}

func checkTimestamp(timestamp, grace int64) bool {
	now := time.Now().Unix()
	// allow 5 seconds onto the check for future timestamps to account for possible clock skew
	return (now-timestamp) < grace && (now+5) >= timestamp
}

//nolint:unused // not used yet
// func checkBodyMD5(toSign []byte, md5Str string) bool {
// 	return md5Hex(toSign) == md5Str
// }

func prepareQueryString(params url.Values) string {
	var keys []string
	for key := range params {
		keys = append(keys, strings.ToLower(key))
	}

	sort.Strings(keys)
	var pieces []string

	for _, key := range keys {
		pieces = append(pieces, key+"="+params.Get(key))
	}

	return strings.Join(pieces, "&")
}

func HmacBytes(toSign, secret []byte) []byte {
	_authSignature := hmac.New(sha256.New, secret)
	_authSignature.Write(toSign)
	return _authSignature.Sum(nil)
}

func HmacSignature(toSign, secret string) string {
	return hex.EncodeToString(HmacBytes([]byte(toSign), []byte(secret)))
}

//nolint:unused // not used yet
// func md5Hex(toSign []byte) string {
// 	h := md5.New()
// 	h.Write(toSign)
// 	return hex.EncodeToString(h.Sum(nil))
// }
