package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	socketio "github.com/googollee/go-socket.io"
	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"github.com/yomorun/yomo/pkg/rx"
)

// sender sends the VHQ events to yomo-zipper for stream processing.
type Sender struct {
	Stream io.Writer
}

// EventData represents the event data for broadcasting to all geo-distributed users.
type EventData struct {
	ServerRegion string `y3:"0x11"`
	Room         string `y3:"0x12"`
	Event        string `y3:"0x13"`
	Data         string `y3:"0x14"`
}

// Player represents the users in VHQ room.
type Player struct {
	ID string `json:"id"`
	// ClientRegion string  `json:"client_region"`
	ServerRegion string  `json:"server_region"`
	Name         string  `json:"name"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
}

// Action represents the user's action in VHQ room.
type Action struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

var players = make(map[string]Player, 0)

const (
	socketioRoom = "yomo-demo"
	socketioAddr = "0.0.0.0:19001"
	zipperAddr   = "localhost:9000"
)

var urls = strings.Split(zipperAddr, ":")
var host = urls[0]
var port, _ = strconv.Atoi(urls[1])
var sender *Sender
var socketioServer *socketio.Server
var serverRegion = os.Getenv("REGION")

func init() {
	if serverRegion == "" {
		serverRegion = "US"
	}
}

func main() {
	// sender will send the data to `yomo-zipper` for stream processing.
	sender = newSender()

	// receiver will receive the data from `yomo-zipper` after stream processing.
	go setupReceiver()

	// socket.io server
	server, err := newSocketIOServer()
	if err != nil {
		log.Printf("❌ Initialize the socket.io server failure with err: %v", err)
		return
	}

	socketioServer = server

	// serve socket.io server.
	go server.Serve()
	defer server.Close()

	router := gin.New()
	router.Use(ginMiddleware())
	router.GET("/socket.io/*any", gin.WrapH(server))
	router.POST("/socket.io/*any", gin.WrapH(server))
	router.Run(socketioAddr)

	log.Print("✅ Serving socket.io on ", socketioAddr)
	err = http.ListenAndServe(socketioAddr, nil)
	if err != nil {
		log.Printf("❌ Serving the socket.io server on %s failure with err: %v", socketioAddr, err)
		return
	}
}

var senderStream io.Writer = nil

// sender sends the movement data to yomo-zipper for stream processing.
func newSender() *Sender {
	cli, err := client.NewSource("Socket.io Client").Connect(host, port)
	if err != nil {
		log.Printf("❌ Connect to the zipper server on %s failure with err: %v", zipperAddr, err)
		return nil
	}
	defer cli.Close()

	return &Sender{
		Stream: cli,
	}
}

// eventDataKey represents the Tag of a Y3 encoded data packet.
const eventDataKey = 0x10

// init a new Y3 codec.
var codec = y3.NewCodec(eventDataKey)

// send the data to `yomo-zipper`.
func (s *Sender) send(data EventData) {
	if s.Stream == nil {
		return
	}

	// encode the data via Y3 Codec.
	buf, _ := codec.Marshal(data)
	// send the encoded data to `yomo-zipper`.
	_, err := s.Stream.Write(buf)

	if err != nil {
		log.Printf("❌ Send to the zipperStream failure with err: %v", err)
	}
}

// setupReceiver connects to `yomo-zipper` as a `yomo-sink`.
// receiver will receive the data from yomo-zipper after stream processing and broadcast it to socket.io clients.
func setupReceiver() {
	cli, err := client.NewServerless("Socket.io").Connect(host, port)
	if err != nil {
		log.Printf("❌ Connect to the zipper server on %s failure with err: %v", zipperAddr, err)
		return
	}

	defer cli.Close()
	cli.Pipe(receiverHandler)
}

var decode = func(v []byte) (interface{}, error) {
	var mold EventData
	err := y3.ToObject(v, &mold)
	if err != nil {
		return nil, err
	}
	return mold, nil
}

var broadcastEventToUsers = func(_ context.Context, i interface{}) (interface{}, error) {
	x, ok := i.(EventData)
	if !ok {
		err := fmt.Sprintf("expected type 'EventData', got '%v' instead",
			reflect.TypeOf(i))
		log.Printf("❌ %v", err)
		return nil, errors.New(err)
	}

	if x.Event == "move" {
		updatePlayerMovement(x)
	}

	// broadcast message to all connected user.
	if socketioServer != nil {
		socketioServer.BroadcastToRoom("", x.Room, x.Event, x.Data)
	}

	return i, nil
}

// receiverHandler will handle data in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	stream := rxstream.
		Subscribe(eventDataKey).
		OnObserve(decode).
		Map(broadcastEventToUsers).
		Marshal(json.Marshal)

	return stream
}

// newSocketIOServer creates a new socket.io server.
func newSocketIOServer() (*socketio.Server, error) {
	log.Print("Starting socket.io server...")
	server, err := socketio.NewServer(nil)
	if err != nil {
		return nil, err
	}

	// add all connected user to the room "yomo-demo".
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Print("connected: ", getPlayerID(s))
		s.Join(socketioRoom)

		return nil
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		server.LeaveRoom("/", socketioRoom, s)
		playerID := getPlayerID(s)
		if players[playerID].ID != "" {
			delete(players, playerID)
		}

		if sender != nil {
			// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
			sender.send(EventData{
				ServerRegion: serverRegion,
				Room:         socketioRoom,
				Event:        "leave",
				Data:         playerID,
			})
		} else {
			// if the sender is nil, broadcast the data to users via socket.io directly.
			server.BroadcastToRoom("", socketioRoom, "leave", playerID)
		}
	})

	server.OnEvent("/", "join", func(s socketio.Conn, msg string) {
		var player Player

		// unmarshal the player from socket.io msg.
		json.Unmarshal([]byte(msg), &player)
		if player.ID == "" {
			player.ID = getPlayerID(s)
		}

		// set server region
		player.ServerRegion = serverRegion

		// add player to list.
		players[player.ID] = player

		// marshal the player for broadcasting.
		newPlayerData, _ := json.Marshal(player)
		if sender != nil {
			// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
			sender.send(EventData{
				ServerRegion: serverRegion,
				Room:         socketioRoom,
				Event:        "join",
				Data:         string(newPlayerData),
			})
		} else {
			// if the sender is nil, broadcast the data to users via socket.io directly.
			server.BroadcastToRoom("", socketioRoom, "join", string(newPlayerData))
		}

		broadcastCurrentPlayers(server)
	})

	server.OnEvent("/", "current", func(s socketio.Conn, msg string) {
		broadcastCurrentPlayers(server)
	})

	server.OnEvent("/", "move", func(s socketio.Conn, movement string) {
		if sender != nil {
			data := EventData{
				ServerRegion: serverRegion,
				Room:         socketioRoom,
				Event:        "move",
				Data:         movement,
			}
			sender.send(data)
		}
	})

	return server, nil
}

func getPlayerID(s socketio.Conn) string {
	// return fmt.Sprint(region, "-", s.ID())
	return s.ID()
}

// broadcast current players to all geo-distributed users.
func broadcastCurrentPlayers(server *socketio.Server) {
	current := make([]Player, 0)

	for _, v := range players {
		current = append(current, v)
	}
	allPlayers, _ := json.Marshal(current)

	if sender != nil {
		// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
		sender.send(EventData{
			ServerRegion: serverRegion,
			Room:         socketioRoom,
			Event:        "current",
			Data:         string(allPlayers),
		})
	} else {
		server.BroadcastToRoom("", socketioRoom, "current", string(allPlayers))
	}
}

// updatePlayerMovement updates the movement action in players list.
func updatePlayerMovement(x EventData) {
	var action Action

	err := json.Unmarshal([]byte(x.Data), &action)
	if err != nil {
		log.Printf("❌ Unmarshal the movement action failed: %v", err)
		return
	}

	p := players[action.ID]
	players[action.ID] = Player{
		ID:           action.ID,
		ServerRegion: serverRegion,
		Name:         p.Name,
		X:            action.X,
		Y:            action.Y,
	}
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
