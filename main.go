package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	socketio "github.com/googollee/go-socket.io"
	"github.com/yomorun/y3-codec-golang/pkg/codes"
	"github.com/yomorun/yomo/pkg/quic"
	"github.com/yomorun/yomo/pkg/rx"
)

const (
	socketioRoom   = "yomo-demo"
	socketioAddr   = "0.0.0.0:8000"
	sinkServerAddr = "0.0.0.0:4141"
	zipperAddr     = "localhost:9999"
)

var zipperStream quic.Stream = nil

func init() {
	_client, err := quic.NewClient(zipperAddr)

	if err != nil {
		log.Printf("❌ Connect to the zipper server on %s failure with err: %v", zipperAddr, err)
		return
	}

	_stream, err := _client.CreateStream(context.Background())

	if err != nil {
		log.Printf("❌ Initialize the zipper stream on %s failure with err: %v", zipperAddr, err)
		return
	}

	zipperStream = _stream

	log.Printf("✅ Connected to yomo-zipper %s", zipperAddr)
}

func main() {
	socketioServer, err := newSocketIOServer()
	if err != nil {
		log.Printf("❌ Initialize the socket.io server failure with err: %v", err)
		return
	}

	// sink server which will receive the data from `yomo-sink`.
	go serveSinkServer(socketioServer, sinkServerAddr)

	// serve socket.io server.
	go socketioServer.Serve()
	defer socketioServer.Close()

	router := gin.New()
	router.Use(ginMiddleware())
	router.GET("/socket.io/*any", gin.WrapH(socketioServer))
	router.POST("/socket.io/*any", gin.WrapH(socketioServer))
	router.Run(socketioAddr)

	log.Print("✅ Serving socket.io on ", socketioAddr)
	err = http.ListenAndServe(socketioAddr, nil)
	if err != nil {
		log.Printf("❌ Serving the socket.io server on %s failure with err: %v", socketioAddr, err)
		return
	}
}

func newSocketIOServer() (*socketio.Server, error) {
	log.Print("Starting socket.io server...")
	server, err := socketio.NewServer(nil)
	if err != nil {
		return nil, err
	}

	// add all connected user to the room "yomo-demo".
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Print("connected:", s.ID())
		s.Join(socketioRoom)

		return nil
	})

	server.OnEvent("/", "receive_source", func(s socketio.Conn, key string) {
		fmt.Println("receive_source:", key)
		proto := codes.NewProtoCodec(0x10)
		sendingBuf, _ := proto.Marshal(key)
		write2zipper(sendingBuf)
	})

	return server, nil
}

// serveSinkServer serves the Sink server over QUIC.
func serveSinkServer(socketioServer *socketio.Server, addr string) {
	log.Print("Starting sink server...")
	handler := &quicServerHandler{
		socketioServer,
	}
	quicServer := quic.NewServer(handler)

	err := quicServer.ListenAndServe(context.Background(), addr)
	if err != nil {
		log.Printf("❌ Serve the sink server on %s failure with err: %v", addr, err)
	}
}

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

type quicServerHandler struct {
	socketioServer *socketio.Server
}

func (s *quicServerHandler) Listen() error {
	// you can add the customized codes which will be triggered when QUIC server is listening.
	return nil
}

func (s *quicServerHandler) Read(st quic.Stream) error {
	// receive the data from `yomo-flow` and use rx (ReactiveX) to process the stream.
	rxStream := rx.FromReader(st).
		Y3Decoder("0x10", string("")) // decode the data via Y3 Codec.

	go func() {
		for customer := range rxStream.Observe() {
			if customer.Error() {
				log.Print(customer.E.Error())
			} else if customer.V != nil {
				// broadcast message to all connected user.
				s.socketioServer.BroadcastToRoom("", socketioRoom, "receive_sink", customer.V.(string))
			}
		}
	}()

	return nil
}

func write2zipper(data []byte) {
	if zipperStream == nil {
		return
	}

	_, err := zipperStream.Write(data)

	if err != nil {
		log.Printf("❌ Send to the zipperStream failure with err: %v", err)
	}
}
