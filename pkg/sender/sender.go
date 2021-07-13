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
		s.dispatchToYoMoReceivers(lib.PresenceOnline{
			Base: lib.PresenceBase{
				Room:      lib.RoomID,
				Event:     "offline",
				Timestamp: time.Now().Unix(),
			},
			Name: userID,
		})

		// leave from socket.io room
		server.LeaveRoom("/", lib.RoomID, conn)
	})

	server.OnEvent("/", "movement", func(conn socketio.Conn, payload interface{}) {
		s.log.Printf("[%s] | EVT | movement | %v - (%T)\n", conn.Context().(string), payload, payload)
		var signal = payload.(map[string]interface{})

		s.log.Printf("[%s] EVT | movement | dir=%v - (%T)\n", conn.Context().(string), signal["dir"], signal["dir"])

		signal["name"] = conn.Context().(string)

		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("/", lib.RoomID, "movement", signal)

		// // send event data to `yomo-zipper` for broadcasting to other mesh nodes
		// data := lib.EventData{
		// 	Room:  lib.RoomID,
		// 	Event: "movement",
		// 	Data:  "movement",
		// }
		// s.send(data)
	})

	// browser will emit "online" event when user connected to WebSocket, with payload:
	// {name: "USER_ID"}
	server.OnEvent("/", "online", func(conn socketio.Conn, payload interface{}) {
		// s.log.Printf("(online) New connection created: %s\n", conn.ID())
		// get the userID from websocket
		var signal = payload.(map[string]interface{})
		userID := signal["name"]
		s.log.Printf("[%s] EVT | online\n", userID)
		// set userID to websocket connection context
		conn.SetContext(userID)

		presence := lib.PresenceOnline{
			Base: lib.PresenceBase{
				Room:      lib.RoomID,
				Event:     "online",
				Timestamp: time.Now().Unix(),
			},
			Name: signal["name"].(string),
		}

		// sent to all Presence-Receiver Servers
		s.dispatchToYoMoReceivers(presence)
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

func (s *Sender) BroadcastOfflineEvent(room string, eventName string, userID string) {
	// s.send(lib.EventData{
	// 	Room:  room,
	// 	Event: eventName,
	// 	Data:  userID,
	// })
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

	// // this a test trying to decode y3 buf
	// var mold lib.PresenceOnline
	// err = y3.ToObject(buf, &mold)
	// s.log.Printf("=======y3.ToObject err=%v", err)
	// s.log.Printf("=======y3.ToObject mold=%v", mold)

	return sent, nil
}
