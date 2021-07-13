package sender

import (
	"io"

	"time"

	color "github.com/fatih/color"
	socketio "github.com/googollee/go-socket.io"

	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"yomo.run/vhq/pkg/lib"
)

// sender sends the VHQ events to yomo-zipper for stream processing.
type Sender struct {
	Stream io.Writer
	log    *color.Color
}

var codec = y3.NewCodec(0x10)

// NewSender send presence as stream to yomo-send-server
func NewSender(host string, port int) *Sender {
	log := color.New(color.FgYellow)
	cli, err := client.NewSource("websocket").Connect(host, port)
	if err != nil {
		log.Printf("❌ Connect to the zipper server on [%s:%d] failure with err: %v", host, port, err)
		return nil
	}

	return &Sender{
		Stream: cli,
		log:    log,
	}
}

func (s *Sender) BindConnectionAsStreamDataSource(server *socketio.Server) {
	// when new user connnected, add them to socket.io room
	server.OnConnect("/", func(conn socketio.Conn) error {
		s.log.Printf("EVT | OnConnect | conn.ID=[%s]\n", conn.ID())
		conn.Join(lib.RoomID)
		return nil
	})

	// when user disconnect, leave them from socket.io room
	// and notify to others in this room
	server.OnDisconnect("/", func(conn socketio.Conn, reason string) {
		s.log.Printf("EVT | OnDisconnect | ID=%s, Context=%v\n", conn.ID(), conn.Context())
		if conn.Context() == nil {
			return
		}

		userID := conn.Context().(string)
		s.log.Printf("[%s] | EVT | OnDisconnect | reason=%s\n", userID, reason)

		// broadcast to all receivers I am offline
		s.dispatchToYoMoReceivers(lib.Presence{
			Room:      lib.RoomID,
			Event:     "offline",
			Timestamp: time.Now().Unix(),
			Payload:   []byte(userID),
		})

		// leave from socket.io room
		server.LeaveRoom("/", lib.RoomID, conn)
	})

	server.OnEvent("/", "movement", func(conn socketio.Conn, payload interface{}) {
		userID := conn.Context().(string)
		s.log.Printf("[%s] | EVT | movement | %v - (%T)\n", userID, payload, payload)

		var signal = payload.(map[string]interface{})
		s.log.Printf("[%s] EVT | movement | dir=%v - (%T)\n", userID, signal["dir"], signal["dir"])

		signal["name"] = userID

		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("/", lib.RoomID, "movement", signal)

		// TODO: broadcast to all receivers
		// var dir = signal["dir"].(map[string]interface{})
		// s.dispatchToYoMoReceivers(lib.PresenceMovement{
		// 	Base: lib.PresenceBase{
		// 		Room:      lib.RoomID,
		// 		Event:     "movement",
		// 		Timestamp: time.Now().Unix(),
		// 	},
		// 	Name: userID,
		// 	Direction: lib.Position{
		// 		X: dir["x"].(int),
		// 		Y: dir["y"].(int),
		// 	},
		// })
	})

	// browser will emit "online" event when user connected to WebSocket, with payload:
	// {name: "USER_ID"}
	server.OnEvent("/", "online", func(conn socketio.Conn, payload interface{}) {
		// s.log.Printf("(online) New connection created: %s\n", conn.ID())
		// get the userID from websocket
		var signal = payload.(map[string]interface{})
		userID := signal["name"].(string)
		s.log.Printf("[%s] EVT | online\n", userID)
		// set userID to websocket connection context
		conn.SetContext(userID)

		s.dispatchToYoMoReceivers(lib.Presence{
			Room:      lib.RoomID,
			Event:     "offline",
			Timestamp: time.Now().Unix(),
			Payload:   []byte(userID),
		})
	})

	// browser will emit "sync" event to tell others my position, the payload looks like
	// {name: "USER_ID", pos: {x: 0, y: 0}}
	server.OnEvent("/", "sync", func(conn socketio.Conn, payload interface{}) {
		var signal = payload.(map[string]interface{})
		// userID := signal["name"]

		s.log.Printf("[%s] EVT | sync | pos=%v", conn.Context().(string), signal)

		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("/", lib.RoomID, "sync", signal)

		// // send event data to `yomo-zipper` for broadcasting to other mesh nodes
		// data := lib.EventData{
		// 	Room:  lib.RoomID,
		// 	Event: "sync",
		// 	Data:  "state",
		// }
		// s.send(data)
	})
}

// send data to downstream Presence-Receiver Servers
func (s *Sender) dispatchToYoMoReceivers(payload interface{}) (int, error) {
	s.log.Printf("dispatchToYoMoReceivers: %v\n", payload)
	buf, err := codec.Marshal(payload)
	if err != nil {
		s.log.Printf("codec.Marshal err: %v\n", err)
		return 0, err
	}
	sent, err := s.Stream.Write(buf)
	if err != nil {
		s.log.Printf("Stream.Write err: %v\n", err)
		return 0, err
	}

	s.log.Printf("dispatchToYoMoReceivers done: (sent %d bytes) % X\n", sent, buf)

	return sent, nil
}
