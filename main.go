package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
var macrometaURL = ""
var macrometaAPIKey = ""
var localPlayersCache = make(map[string]Player, 0)

func init() {
	if serverRegion == "" {
		serverRegion = "US"
	}

	macrometaURL = "https://api-gdn.paas.macrometa.io/_fabric/_system/_api/kv/VHQ/value"
	if os.Getenv("MACROMETA_API_KEY") != "" {
		macrometaAPIKey = fmt.Sprintf("apikey %s", os.Getenv("MACROMETA_API_KEY"))
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

// sender sends the movement data to yomo-zipper for stream processing.
func newSender() *Sender {
	cli, err := client.NewSource("Socket.io Client").Connect(host, port)
	if err != nil {
		log.Printf("❌ Connect to the zipper server on %s failure with err: %v", zipperAddr, err)
		return nil
	}

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

// decode the data via Y3 Codec.
var decode = func(v []byte) (interface{}, error) {
	var mold EventData
	err := y3.ToObject(v, &mold)
	if err != nil {
		return nil, err
	}
	return mold, nil
}

// broadcast the events the geo-distributed users.
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

	// broadcast message to all connected users in other regions.
	if socketioServer != nil && x.ServerRegion != serverRegion {
		socketioServer.BroadcastToRoom("", x.Room, x.Event, x.Data)
	}

	return i, nil
}

// receiverHandler will handle data in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	stream := rxstream.
		Subscribe(eventDataKey).
		OnObserve(decode).
		Map(broadcastEventToUsers)

	return stream
}

// newSocketIOServer creates a new socket.io server.
func newSocketIOServer() (*socketio.Server, error) {
	log.Print("Starting socket.io server...")
	server := socketio.NewServer(nil)

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

		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("", socketioRoom, "leave", playerID)

		if sender != nil {
			// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
			sender.send(EventData{
				ServerRegion: serverRegion,
				Room:         socketioRoom,
				Event:        "leave",
				Data:         playerID,
			})
		}

		// update local cache
		if localPlayersCache[playerID].ID != "" {
			delete(localPlayersCache, playerID)
		}

		// delete player in Macrometa
		players, err := getCurrentPlayers()
		if err != nil {
			log.Printf("❌ get the current players from Macrometa failed: %v", err)
		}

		if players[playerID].ID != "" {
			delete(players, playerID)
			saveCurrentPlayers(players)
		}
	})

	server.OnEvent("/", "join", func(s socketio.Conn, msg string) {
		var player Player

		// unmarshal the player from socket.io msg.
		err := json.Unmarshal([]byte(msg), &player)
		if err != nil {
			log.Printf("❌ Unmarshal the join event failed: %v", err)
		}
		if player.Name == "" {
			// skip the player when the name is empty.
			return
		}

		// set player ID
		if player.ID == "" {
			player.ID = getPlayerID(s)
		}

		// set server region
		if player.ServerRegion == "" {
			player.ServerRegion = serverRegion
		}

		// marshal the player for broadcasting.
		newPlayerData, _ := json.Marshal(player)

		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("", socketioRoom, "join", string(newPlayerData))

		// emit host-player-id to current user
		s.Emit("hostPlayerId", player.ID)

		// add player to list.
		localPlayersCache[player.ID] = player

		// broadcast current players list
		broadcastCurrentPlayers(server)

		if sender != nil {
			// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
			sender.send(EventData{
				ServerRegion: serverRegion,
				Room:         socketioRoom,
				Event:        "join",
				Data:         string(newPlayerData),
			})
		}

		// save current players to Macrodata
		go func() {
			err = saveCurrentPlayers(localPlayersCache)
			if err != nil {
				log.Printf("❌ save current players to Macrometa failed: %v", err)
			}
		}()
	})

	server.OnEvent("/", "current", func(s socketio.Conn, msg string) {
		broadcastCurrentPlayers(server)
	})

	server.OnEvent("/", "move", func(s socketio.Conn, movement string) {
		// broadcast the data to local users via socket.io directly.
		socketioServer.BroadcastToRoom("", socketioRoom, "move", movement)

		if sender != nil {
			// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
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
	return fmt.Sprint(serverRegion, "-", s.ID())
}

// broadcast current players to all geo-distributed users.
func broadcastCurrentPlayers(server *socketio.Server) {
	current := make([]Player, 0)

	for _, v := range localPlayersCache {
		current = append(current, v)
	}
	data, _ := json.Marshal(current)

	// broadcast the data to local users via socket.io directly.
	server.BroadcastToRoom("", socketioRoom, "current", string(data))

	if sender != nil {
		// send event data to `yomo-zipper` for broadcasting to geo-distributed users.
		sender.send(EventData{
			ServerRegion: serverRegion,
			Room:         socketioRoom,
			Event:        "current",
			Data:         string(data),
		})
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

	p := localPlayersCache[action.ID]
	if p.ID != "" {
		p.X = action.X
		p.Y = action.Y
		localPlayersCache[action.ID] = p
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

type macrometaKVCreate struct {
	Key      string `json:"_key"`
	Value    string `json:"value"`
	ExpireAt int64  `json:"expireAt"`
}

type macrometaKVGet struct {
	ID       string `json:"_id"`
	Key      string `json:"_key"`
	Rev      string `json:"_rev"`
	ExpireAt int64  `json:"expireAt"`
	Value    string `json:"value"`
}

// saveCurrentPlayers saves data to macrometa
func saveCurrentPlayers(players map[string]Player) error {
	if macrometaAPIKey == "" {
		return errors.New("Macrometa API key is empty")
	}

	playersBuf, err := json.Marshal(players)
	if err != nil {
		return err
	}

	data := []macrometaKVCreate{
		{
			Key:      "current",
			Value:    string(playersBuf),
			ExpireAt: -1,
		},
	}

	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", macrometaURL, bytes.NewBuffer(buf))
	req.Header.Set("authorization", macrometaAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return err

	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err

	}
	if !(resp.StatusCode == 201 || resp.StatusCode == 202) {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return errors.New(string(bodyBytes))
	}

	defer resp.Body.Close()

	return nil
}

// getCurrentPlayers gets current players from macrometa
func getCurrentPlayers() (map[string]Player, error) {
	var players = make(map[string]Player, 0)
	if macrometaAPIKey == "" {
		return players, errors.New("Macrometa API key is empty")
	}

	req, err := http.NewRequest("GET", macrometaURL+"/current", nil)

	req.Header.Set("authorization", macrometaAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return players, err

	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return players, err

	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return players, errors.New(string(bodyBytes))
	}

	defer resp.Body.Close()
	var data macrometaKVGet
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return players, err
	}

	err = json.Unmarshal([]byte(data.Value), &players)
	return players, err
}
