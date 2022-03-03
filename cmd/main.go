package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	socketio "github.com/googollee/go-socket.io"

	"yomo.run/vhq/pkg/receiver"
	"yomo.run/vhq/pkg/sender"
)

const (
	socketioAddr = "0.0.0.0:19001"
)

// YoMo zipper for sending messages
var senderYoMoServer string

// YoMo zipper for receiving messages
var receiverYoMoServer string

// The app_id when connects to zipper.
var zipperAppID string

// The app_secret when connects to zipper.
var zipperAppSecret string

// meshID is the access point for users. This services can be deployed at anywhere
// to decrease connection_time. Users can connect to the nearest access point.
var meshID = os.Getenv("MESH_ID")

func main() {
	log.Printf("MESH_ID: %s", meshID)

	// the YoMo server responsible for send messages
	senderYoMoServer = os.Getenv("SENDER")
	if len(senderYoMoServer) == 0 {
		senderYoMoServer = "localhost:8000"
	}

	// the YoMo server responsible for receive messages
	receiverYoMoServer = os.Getenv("RECEIVER")
	if len(receiverYoMoServer) == 0 {
		receiverYoMoServer = "localhost:9000"
	}

	// The app_id when connects to zipper.
	zipperAppID = os.Getenv("ZIPPER_APP_ID")
	if len(zipperAppID) == 0 {
		zipperAppID = "app_vhq"
	}

	// The app_secret when connects to zipper.
	zipperAppSecret = os.Getenv("ZIPPER_APP_SECRET")
	if len(zipperAppSecret) == 0 {
		zipperAppSecret = "app_vhq_secret"
	}

	// create the socket.io server, handle user connections.
	server, err := newSocketIOServer()
	if err != nil {
		log.Printf("❌ Initialize the socket.io server failure with err: %v", err)
		return
	}
	defer server.Close()

	// sender will send the data to `yomo-zipper` for stream processing.
	go sender.NewSender(senderYoMoServer, server, zipperAppID, zipperAppSecret)

	// receiver will receive the data from `yomo-zipper` after stream processing.
	go receiver.NewReceiver(receiverYoMoServer, server, zipperAppID, zipperAppSecret)

	// serve socket.io server.
	go server.Serve()

	router := gin.New()
	router.Use(ginMiddleware())
	router.GET("/socket.io/*any", gin.WrapH(server))
	router.POST("/socket.io/*any", gin.WrapH(server))
	router.GET("/config", func(c *gin.Context) {
		c.String(200, "[{\"name\":\"Receiver-Local\",\"host\":\"localhost\",\"port\":9000}]")
	})
	router.GET("/whoami", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"mesh_id": os.Getenv("MESH_ID"),
		})
	})
	router.Run(socketioAddr)

	log.Print("✅ Serving socket.io on ", socketioAddr)
	err = http.ListenAndServe(socketioAddr, nil)
	if err != nil {
		log.Printf("❌ Serving the socket.io server on %s failure with err: %v", socketioAddr, err)
		return
	}
}

// newSocketIOServer creates a new socket.io server.
func newSocketIOServer() (*socketio.Server, error) {
	log.Print("Starting socket.io server...")
	server := socketio.NewServer(nil)

	return server, nil
}

// ginMiddleware allows the CORS in socket.io server.
func ginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestOrigin := c.Request.Header.Get("Origin")

		c.Writer.Header().Set("Access-Control-Allow-Origin", requestOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Content-Length, X-CSRF-Token, Token, session, Origin, Host, Connection, Accept-Encoding, Accept-Language, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Request.Header.Del("Origin")
		c.Next()
	}
}

func getHostAndPort(url string) (host string, port int) {
	var urls = strings.Split(url, ":")
	host = urls[0]
	port, _ = strconv.Atoi(urls[1])
	return host, port
}
