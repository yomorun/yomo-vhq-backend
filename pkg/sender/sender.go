package sender

import (
	"io"

	"time"

	color "github.com/fatih/color"
	socketio "github.com/googollee/go-socket.io"

	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"github.com/yomorun/yomo/pkg/logger"
	"yomo.run/vhq/pkg/lib"
)

// sender sends the VHQ events to yomo-zipper for stream processing.
type Sender struct {
	Stream io.Writer
	logger *color.Color
}

var codec = y3.NewCodec(0x10)

// NewSender send presence as stream to yomo-send-server
func NewSender(host string, port int) *Sender {
	logger := color.New(color.FgYellow)
	cli, err := client.NewSource("websocket").Connect(host, port)
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
		s.logger.Printf("EVT | OnConnect | conn.ID=[%s]\n", conn.ID())
		conn.Join(lib.RoomID)
		return nil
	})

	// when user disconnect, leave them from socket.io room
	// and notify to others in this room
	server.OnDisconnect("/", func(conn socketio.Conn, reason string) {
		s.logger.Printf("EVT | OnDisconnect | ID=%s, Context=%v\n", conn.ID(), conn.Context())
		if conn.Context() == nil {
			return
		}

		userID := conn.Context().(string)
		s.logger.Printf("[%s] | EVT | OnDisconnect | reason=%s\n", userID, reason)

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
		s.logger.Printf("[%s] | EVT | movement | %v - (%T)\n", userID, payload, payload)

		signal, ok := payload.(map[string]interface{})
		s.logger.Printf("[%s] | EVT | movement | ok=%v dir=%v - (%T)\n", ok, userID, signal["dir"], signal["dir"])

		signal["name"] = userID

		// broadcast to all receivers
		dir := signal["dir"].(map[string]interface{})
		p, err := lib.EncodeMovement(userID, dir["x"].(float64), dir["y"].(float64))

		if err != nil {
			logger.Printf("ERR | lib.EncodeMovement err: %v\n", err)
		} else {
			// s.logger.Printf("[%s] | EVT | movement | p=%v\n", userID, p)
			s.dispatchToYoMoReceivers(p)
		}
	})

	// browser will emit "online" event when user connected to WebSocket, with payload:
	// {name: "USER_ID"}
	server.OnEvent("/", "online", func(conn socketio.Conn, payload interface{}) {
		// s.logger.Printf("(online) New connection created: %s\n", conn.ID())
		// get the userID from websocket
		var signal = payload.(map[string]interface{})
		userID := signal["name"].(string)
		s.logger.Printf("[%s] | EVT | online\n", userID)
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

		s.logger.Printf("[%s] | EVT | sync | pos=%v", conn.Context().(string), signal)

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
	s.logger.Printf("dispatchToYoMoReceivers: %v\n", payload)
	buf, err := codec.Marshal(payload)
	if err != nil {
		s.logger.Printf("codec.Marshal err: %v\n", err)
		return 0, err
	}
	sent, err := s.Stream.Write(buf)
	if err != nil {
		s.logger.Printf("Stream.Write err: %v\n", err)
		return 0, err
	}

	// s.logger.Printf("dispatchToYoMoReceivers done: (sent %d bytes) % X\n", sent, buf)

	return sent, nil
}
