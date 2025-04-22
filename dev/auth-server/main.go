package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pusher/pusher-http-go/v5"
	"golang.org/x/crypto/nacl/secretbox"

	"strconv"
	"strings"
	"time"
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

var encryptionKey string
var pusherClient *pusher.Client

func init() {

}

func main() {
	if fileExists("../../.env") {
		err := godotenv.Load("../../.env")
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	} else {
		fmt.Println("No .env file found")
	}

	if os.Getenv("APP_ID") == "" {
		panic("APP_ID not set")
	}

	fmt.Println("Creating an encryption key for use with encrypted channels")
	// Generate a random 32-byte key
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		log.Fatal("Failed to generate random key:", err)
	}

	// Convert to Base64
	encryptionKey = base64.StdEncoding.EncodeToString(key)

	pusherClient, _ = pusher.ClientFromURL(
		fmt.Sprintf("http://%s:%s@%s/apps/%s",
			os.Getenv("APP_KEY"),
			os.Getenv("APP_SECRET"),
			"localhost:6001",
			os.Getenv("APP_ID"),
		),
	)

	if os.Getenv("APP_HOST") == "pusher" {
		pusherClient = &pusher.Client{
			AppID:   os.Getenv("APP_ID"),
			Key:     os.Getenv("APP_KEY"),
			Secret:  os.Getenv("APP_SECRET"),
			Cluster: os.Getenv("APP_CLUSTER"),
		}
	}

	pusherClient.EncryptionMasterKeyBase64 = encryptionKey

	router := gin.Default()
	router.Use(CORSMiddleware)
	router.POST("/auth", AuthWebsocket)
	router.POST("/user-auth", AuthUser)
	// test/:channel is used for simulating events being sent from a backend server to connected clients
	router.POST("/test/:channel", SendMessage)

	router.POST("/webhook", ReceiveWebHook)

	router.POST("/send-user/:user_id", SendUserMessage)

	// the following routes are for simulating the Pusher API
	router.GET("/channels", GetChannelsIndex)
	router.GET("/channels/:channel_name", GetChannel)
	router.GET("/channels/:channel_name/users", ChannelUsers)
	router.POST("/terminate/:user_id", TerminateUserConnections) // Go client doesn't yet support user termination

	fmt.Println("Server is running at localhost:8099")
	fmt.Println("DO NOT USE THIS SERVER IN PRODUCTION")
	fmt.Println("This server is for development of the SocketRush server only.")
	fmt.Println("It will authenticate all connections, using a random user ID.")
	_ = router.Run("localhost:8099")
}

type AuthRequestData struct {
	ChannelName string `json:"channel_name" form:"channel_name"`
	SocketID    string `json:"socket_id" form:"socket_id"`
}

func AuthWebsocket(c *gin.Context) {
	var data AuthRequestData

	if err := c.Bind(&data); err != nil {
		fmt.Println(err)
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	params := "socket_id=" + data.SocketID + "&channel_name=" + data.ChannelName
	fmt.Println("Auth request params: ", params)

	var response []byte
	var err error
	if strings.HasPrefix(data.ChannelName, "presence-") {
		fmt.Println("Authenticating ", data.ChannelName)
		var idAsString string
		if c.Query("user_id") != "" {
			idAsString = c.Query("user_id")
		} else {
			currentTime := time.Now()
			currentMinute := currentTime.Minute()
			idAsString = strconv.Itoa(currentMinute)
		}
		// get the current minute

		// idAsString := "9876"
		fmt.Println("User ID:", idAsString)
		presenceData := pusher.MemberData{
			UserID: idAsString,
			UserInfo: map[string]string{
				"id":    idAsString,
				"email": "user" + idAsString + "@example.com",
				"name":  "User " + idAsString,
			},
		}
		response, err = pusherClient.AuthorizePresenceChannel([]byte(params), presenceData)
	} else if strings.HasPrefix(data.ChannelName, "private-") {
		response, err = pusherClient.AuthorizePrivateChannel([]byte(params))
	} else if strings.HasPrefix(data.ChannelName, "private-encrypted-") {
		response, err = pusherClient.AuthorizePrivateChannel([]byte(params))
	}
	if err != nil {
		fmt.Println("Error authorizing channel:", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}
	var authResponse map[string]interface{}
	if err2 := json.Unmarshal(response, &authResponse); err2 != nil {
		fmt.Println("Response: ", response)
		fmt.Println("Error unmarshalling response:", err2)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, authResponse)
}

type userAuthObjUserInfo struct {
	Name string `json:"name"`
}
type userAuthObj struct {
	ID        string              `json:"id"`
	UserInfo  userAuthObjUserInfo `json:"user_info,omitempty"`
	Watchlist []string            `json:"watchlist,omitempty"`
}

func AuthUser(c *gin.Context) {

	var idAsString string
	if c.Query("user_id") != "" {
		idAsString = c.Query("user_id")
	} else {
		currentTime := time.Now()
		currentMinute := currentTime.Minute()
		idAsString = strconv.Itoa(currentMinute)
	}
	fmt.Println("Authenticating ", idAsString)

	// Read the raw body
	params, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Println("Error reading request body:", err)
		c.JSON(400, gin.H{"error": "Invalid request data"})
		return
	}

	userMap := map[string]any{
		"id": idAsString,
		"user_info": map[string]any{
			"name": "User " + idAsString,
		},
	}

	fmt.Println("User ID:", idAsString)
	res, err := pusherClient.AuthenticateUser(params, userMap)
	if err != nil {
		fmt.Println("Error authorizing user:", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}

	fmt.Println("Authorized user:", res)
	// userObj := &userAuthObj{
	// 	ID: idAsString,
	// 	UserInfo: userAuthObjUserInfo{
	// 		Name: "User " + idAsString,
	// 	},
	// }
	// userObjJson, _ := json.Marshal(userObj)
	// authResponse2 := struct {
	// 	Auth     string `json:"auth"`
	// 	UserData string `json:"user_data"`
	// }{
	// 	Auth:     fmt.Sprintf("%s:%s", os.Getenv("APP_KEY"), res),
	// 	UserData: string(userObjJson),
	// }
	var aResp authResponse
	// concat a letter at the end of res

	_ = json.Unmarshal([]byte(res), &aResp)
	fmt.Println("Auth response: ", aResp)

	c.JSON(http.StatusOK, aResp)
	// c.JSON(http.StatusOK, res)
	return
}

type authResponse struct {
	Auth     string `json:"auth"`
	UserData string `json:"user_data"`
}

func CORSMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}

type sendEventPayloadType struct {
	Data string `json:"data"`
}

func SendMessage(c *gin.Context) {
	// Publish message to channel
	channel := c.Param("channel")
	client, e := pusher.ClientFromURL(pusherUrl())
	if e != nil {
		panic(e)
	}

	var data sendEventPayloadType
	_ = c.ShouldBindJSON(&data)

	// check if channel name starts with "private-encrypted-"
	if strings.HasPrefix(channel, "private-encrypted-") {
		client.EncryptionMasterKeyBase64 = encryptionKey
	}

	eventName := "EchoEvent"

	if c.Query("name") != "" {
		eventName = c.Query("name")
	}

	tErr := pusherClient.Trigger(channel, eventName, data.Data)

	if tErr != nil {
		fmt.Println(tErr)
	}
}

func GetChannelsIndex(c *gin.Context) {
	// client, e := pusher.ClientFromURL(pusherUrl())
	// client.EncryptionMasterKeyBase64 = encryptionKey
	// if e != nil {
	// 	panic(e)
	// }
	var uc string
	var filter string
	if c.Query("info") != "" {
		uc = "user_count"
	}
	if c.Query("filter_by_prefix") != "" {
		filter = c.Query("filter_by_prefix")
	}
	params := pusher.ChannelsParams{
		Info:           &uc,
		FilterByPrefix: &filter,
	}
	channels, err := pusherClient.Channels(params)
	if err != nil {
		fmt.Println("Error getting channels: ", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	fmt.Printf("Channels: %v", channels)
	c.JSON(http.StatusOK, channels)
}

func GetChannel(c *gin.Context) {
	// client, e := pusher.ClientFromURL(pusherUrl())
	// if e != nil {
	// 	panic(e)
	// }
	channel := c.Param("channel_name")
	info := "user_count,subscription_count,cache"
	params := pusher.ChannelParams{
		Info: &info,
	}
	channelInfo, _ := pusherClient.Channel(channel, params)
	c.JSON(http.StatusOK, channelInfo)
}

func ChannelUsers(c *gin.Context) {
	client, _ := pusher.ClientFromURL(pusherUrl())
	channel := c.Param("channel_name")
	info, _ := client.GetChannelUsers(channel)
	c.JSON(http.StatusOK, info)
}

func SendUserMessage(c *gin.Context) {
	user := c.Param("user_id")

	data := map[string]string{"hello": "world"}
	fmt.Println("sending to user: ", user)
	err := pusherClient.SendToUser(user, "myTestEvent", data)
	if err != nil {
		fmt.Println("Error sending message to user: ", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, nil)
}

func TerminateUserConnections(c *gin.Context) {
	user := c.Param("user_id")
	// client, e := pusher.ClientFromURL(pusherUrl())
	// if e != nil {
	// 	panic(e)
	// }
	// return $this->post("/users/$user_id/terminate_connections", "{}");

	path := fmt.Sprintf("/apps/%s/users/%s/terminate_connections", pusherClient.AppID, user)
	body := []byte("{}")
	u, err := createRequestURL("POST", pusherClient.Host, path, pusherClient.Key, pusherClient.Secret, authTimestamp(), pusherClient.Secure, body, nil, pusherClient.Cluster)
	fmt.Println("URL: ", u)
	if err != nil {
		fmt.Println("Error terminating user connections: ", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	req, _ := http.NewRequest("POST", u, bytes.NewBuffer(body))
	httpClient := &http.Client{}
	res, err := httpClient.Do(req)

	defer res.Body.Close()

	if err != nil {
		fmt.Println("Error terminating user connections 2: ", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User connections terminated"})
}

func ReceiveWebHook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Println("Error reading request body: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	// fmt.Println("Body: ", string(body))
	// fmt.Println("Header: ", c.Request.Header)
	webhook, qerr := pusherClient.Webhook(c.Request.Header, body)
	if qerr != nil {
		fmt.Println("Error parsing webhook: ", qerr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook"})
		return
	}
	//
	// var data pusher.Webhook
	// e := c.BindJSON(&data)
	// if e != nil {
	//	fmt.Println("Error binding JSON: ", e)
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
	//	return
	// }

	fmt.Println("Received Webhook: ", webhook)
	c.JSON(http.StatusOK, nil)
}

func pusherUrl() string {

	return fmt.Sprintf("http://%s:%s@%s/apps/%s",
		os.Getenv("APP_KEY"),
		os.Getenv("APP_SECRET"),
		"localhost:6001",
		os.Getenv("APP_ID"),
	)
}

// ######## HELPER STUFF #########

const authVersion = "1.0"

func md5Signature(body []byte) string {
	_bodyMD5 := md5.New()
	_bodyMD5.Write([]byte(body))
	return hex.EncodeToString(_bodyMD5.Sum(nil))
}

func authTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func unsignedParams(key, timestamp string, body []byte, parameters map[string]string) url.Values {
	params := url.Values{
		"auth_key":       {key},
		"auth_timestamp": {timestamp},
		"auth_version":   {authVersion},
	}

	if body != nil {
		params.Add("body_md5", md5Signature(body))
	}

	if parameters != nil {
		for key, values := range parameters {
			params.Add(key, values)
		}
	}

	return params

}

func unescapeURL(_url url.Values) string {
	unesc, _ := url.QueryUnescape(_url.Encode())
	return unesc
}

func hmacSignature(toSign, secret string) string {
	return hex.EncodeToString(hmacBytes([]byte(toSign), []byte(secret)))
}

func hmacBytes(toSign, secret []byte) []byte {
	_authSignature := hmac.New(sha256.New, secret)
	_authSignature.Write(toSign)
	return _authSignature.Sum(nil)
}

func createRequestURL(method, host, path, key, secret, timestamp string, secure bool, body []byte, parameters map[string]string, cluster string) (string, error) {
	params := unsignedParams(key, timestamp, body, parameters)

	stringToSign := strings.Join([]string{method, path, unescapeURL(params)}, "\n")

	authSignature := hmacSignature(stringToSign, secret)

	params.Add("auth_signature", authSignature)

	if host == "" {
		if cluster != "" {
			host = "api-" + cluster + ".pusher.com"
		} else {
			host = "api.pusherapp.com"
		}
	}
	var base string
	if secure {
		base = "https://"
	} else {
		base = "http://"
	}
	base += host

	endpoint, err := url.ParseRequestURI(base + path)
	if err != nil {
		return "", err
	}
	endpoint.RawQuery = unescapeURL(params)

	return endpoint.String(), nil
}

func encodeTriggerBody(
	channels []string,
	event string,
	data interface{},
	params map[string]string,
	encryptionKey []byte,
	overrideMaxMessagePayloadKB int,
) ([]byte, error) {
	dataBytes, err := encodeEventData(data)
	if err != nil {
		return nil, err
	}
	var payloadData string
	if strings.HasPrefix(channels[0], "private-encrypted-") || strings.HasPrefix(channels[0], "private-encrypted-cache-") {
		payloadData = encrypt(channels[0], dataBytes, encryptionKey)
	} else {
		payloadData = string(dataBytes)
	}

	eventPayload := map[string]interface{}{
		"name":     event,
		"channels": channels,
		"data":     payloadData,
	}
	for k, v := range params {
		if _, ok := eventPayload[k]; ok {
			return nil, errors.New(fmt.Sprintf("Paramater %s specified multiple times", k))
		}
		eventPayload[k] = v
	}
	return json.Marshal(eventPayload)
}

func encodeEventData(data interface{}) ([]byte, error) {
	var dataBytes []byte
	var err error

	switch d := data.(type) {
	case []byte:
		dataBytes = d
	case string:
		dataBytes = []byte(d)
	default:
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}
	return dataBytes, nil
}

func encrypt(channel string, data []byte, encryptionKey []byte) string {
	sharedSecret := generateSharedSecret(channel, encryptionKey)
	nonce := generateNonce()
	nonceB64 := base64.StdEncoding.EncodeToString(nonce[:])
	cipherText := secretbox.Seal([]byte{}, data, &nonce, &sharedSecret)
	cipherTextB64 := base64.StdEncoding.EncodeToString(cipherText)
	return formatMessage(nonceB64, cipherTextB64)
}

func generateNonce() [24]byte {
	var nonce [24]byte
	// Trick ReadFull into thinking nonce is a slice
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}
	return nonce
}

func generateSharedSecret(channel string, encryptionKey []byte) [32]byte {
	return sha256.Sum256(append([]byte(channel), encryptionKey...))
}

func formatMessage(nonce string, cipherText string) string {
	encryptedMessage := &EncryptedMessage{
		Nonce:      nonce,
		Ciphertext: cipherText,
	}
	json, err := json.Marshal(encryptedMessage)
	if err != nil {
		panic(err)
	}

	return string(json)
}

type EncryptedMessage struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}
