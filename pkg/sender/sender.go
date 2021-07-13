package sender

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	socketio "github.com/googollee/go-socket.io"

	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"yomo.run/vhq/pkg/lib"
)

// sender sends the VHQ events to yomo-zipper for stream processing.
type Sender struct {
	Stream io.Writer
	logger *log.Logger
}

var codec = y3.NewCodec(0x10)

// NewSender send presence as stream to yomo-send-server
func NewSender(host string, port int) *Sender {
	logger := log.New(os.Stdout, "[Sender] ", log.LstdFlags)
	cli, err := client.NewSource("Socket.io").Connect(host, port)
	if err != nil {
		logger.Printf("❌ Connect to the zipper server on [%s:%d] failure with err: %v", host, port, err)
		return nil
	}

	return &Sender{
		Stream: cli,
		logger: logger,
	}
}

func (s *Sender) BindConnectionAsStreamDataSource(server *socketio.Server) {
	// when new user connnected, add them to socket.io room
	server.OnConnect("/", func(conn socketio.Conn) error {
		// userID := getConnectionID(s)
		s.logger.Printf("========> [%s] Connected", conn.ID())
		// s.SetContext(userID)

		conn.Join(lib.RoomID)

		// s.Emit("init", s.ID())

		return nil
	})

	// when user disconnect, leave them from socket.io room,``
	// and notify to others in this room
	server.OnDisconnect("/", func(conn socketio.Conn, reason string) {
		s.logger.Printf("========>> ID=%s, Context=%v", conn.ID(), conn.Context())
		if conn.Context() == nil {
			return
		}

		s.logger.Printf("[%v] | EVT | disconnect | reason=%s", conn.Context().(string), reason)

		// broadcast the data to local mesh users via socket.io directly.
		var payload = &map[string]interface{}{"name": conn.Context().(string)}
		server.BroadcastToRoom("/", lib.RoomID, "offline", payload)

		server.LeaveRoom("/", lib.RoomID, conn)

		// broadcast to other mesh nodes
		s.BroadcastOfflineEvent(lib.RoomID, "offline", fmt.Sprintf("%s", conn.Context()))
	})

	server.OnEvent("/", "movement", func(conn socketio.Conn, payload interface{}) {
		s.logger.Printf("[%s] | EVT | movement | %v - (%T)", conn.Context().(string), payload, payload)
		var signal = payload.(map[string]interface{})

		s.logger.Printf("[%s] EVT | movement | dir=%v - (%T)", conn.Context().(string), signal["dir"], signal["dir"])

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

	// browser will emit "online" event after user connected to WebSocket, will payload:
	// {name: "USER_ID"}
	server.OnEvent("/", "online", func(conn socketio.Conn, payload interface{}) {
		s.logger.Printf("New connection created: %s", conn.ID())
		var signal = payload.(map[string]interface{})
		userID := signal["name"]

		s.logger.Printf("[%s] EVT | online", userID)

		conn.SetContext(userID)

		// // broadcast the data to local users via socket.io directly.
		// server.BroadcastToRoom("/", lib.RoomID, "online", signal)
		// server.BroadcastToRoom("/", lib.RoomID, "ask")

		presence := lib.PresenceOnline{
			Base: lib.PresenceBase{
				Room:      lib.RoomID,
				Event:     "online",
				Timestamp: time.Now().Unix(),
			},
			Name: signal["name"].(string),
		}

		s.logger.Printf("Broadcasted: %v", presence)

		buf, err := codec.Marshal(presence)
		if err != nil {
			s.logger.Println("y3 ERR--")
			// s.logger.Fatalln(err)
			// return
		} else {
			s.logger.Printf("post-y3: buf=%v", buf)
		}
		_, err = s.Stream.Write(buf)
		if err != nil {
			s.logger.Println("send ERR-- --")
			// s.logger.Fatalln(err)
		} else {
			s.logger.Printf("-> [%s] | EVT | online | %v", userID, buf)
		}
	})

	// browser will emit "sync" event to tell others my position, the payload looks like
	// {name: "USER_ID", pos: {x: 0, y: 0}}
	server.OnEvent("/", "sync", func(conn socketio.Conn, payload interface{}) {
		var signal = payload.(map[string]interface{})
		// userID := signal["name"]

		s.logger.Printf("[%s] EVT | sync | pos=%v", conn.Context().(string), signal)

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

// // send the data to `yomo-zipper`.
// func (s *Sender) send(data lib.EventData) {
// 	// init a new Y3 codec.
// 	var codec = y3.NewCodec(lib.EventDataKey)

// 	if s.Stream == nil {
// 		return
// 	}

// 	// encode the data via Y3 Codec.
// 	buf, _ := codec.Marshal(data)
// 	// send the encoded data to `yomo-zipper`.
// 	_, err := s.Stream.Write(buf)

// 	if err != nil {
// 		log.Printf("❌ Send to the zipperStream failure with err: %v", err)
// 	}
// }

func getConnectionID(conn socketio.Conn) string {
	return fmt.Sprint(os.Getenv("MESH_ID"), "-", conn.ID())
}
