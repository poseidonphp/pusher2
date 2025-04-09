package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/pusher/pusher-http-go/v5"
	"io"
	"log"
	"net/http"
	"os"

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
	pusherClient.EncryptionMasterKeyBase64 = encryptionKey

	router := gin.Default()
	router.Use(CORSMiddleware)
	router.POST("/auth", AuthWebsocket)
	// test/:channel is used for simulating events being sent from a backend server to connected clients
	router.POST("/test/:channel", SendMessage)

	router.POST("/webhook", ReceiveWebHook)

	// the following routes are for simulating the Pusher API
	router.GET("/channels", GetChannelsIndex)
	router.GET("/channels/:channel_name", GetChannel)
	router.GET("/channels/:channel_name/users", ChannelUsers)

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

		//idAsString := "9876"
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
	//if strings.HasPrefix(channel, "private-encrypted-") {
	//	client.EncryptionMasterKeyBase64 = encryptionKey
	//}

	eventName := "EchoEvent"

	if c.Query("name") != "" {
		eventName = c.Query("name")
	}

	tErr := client.Trigger(channel, eventName, data.Data)
	if tErr != nil {
		fmt.Println(tErr)
	}
}

func GetChannelsIndex(c *gin.Context) {
	client, e := pusher.ClientFromURL(pusherUrl())
	client.EncryptionMasterKeyBase64 = encryptionKey
	if e != nil {
		panic(e)
	}
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
	channels, err := client.Channels(params)
	if err != nil {
		fmt.Println("Error getting channels: ", err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	fmt.Printf("Channels: %v", channels)
	c.JSON(http.StatusOK, channels)
}

func GetChannel(c *gin.Context) {
	client, e := pusher.ClientFromURL(pusherUrl())
	if e != nil {
		panic(e)
	}
	channel := c.Param("channel_name")
	info := "user_count,subscription_count,cache"
	params := pusher.ChannelParams{
		Info: &info,
	}
	channelInfo, _ := client.Channel(channel, params)
	c.JSON(http.StatusOK, channelInfo)
}

func ChannelUsers(c *gin.Context) {
	client, _ := pusher.ClientFromURL(pusherUrl())
	channel := c.Param("channel_name")
	info, _ := client.GetChannelUsers(channel)
	c.JSON(http.StatusOK, info)
}

func ReceiveWebHook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Println("Error reading request body: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	//fmt.Println("Body: ", string(body))
	//fmt.Println("Header: ", c.Request.Header)
	webhook, qerr := pusherClient.Webhook(c.Request.Header, body)
	if qerr != nil {
		fmt.Println("Error parsing webhook: ", qerr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook"})
		return
	}
	//
	//var data pusher.Webhook
	//e := c.BindJSON(&data)
	//if e != nil {
	//	fmt.Println("Error binding JSON: ", e)
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
	//	return
	//}

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
