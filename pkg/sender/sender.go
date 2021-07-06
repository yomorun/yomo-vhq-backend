package sender

import (
	"fmt"
	"io"
	"log"
	"os"

	socketio "github.com/googollee/go-socket.io"

	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"yomo.run/vhq/pkg/lib"
)

// sender sends the VHQ events to yomo-zipper for stream processing.
type Sender struct {
	Stream io.Writer
}

// NewSender send presence as stream to yomo-send-server
func NewSender(host string, port int, zipperAddr string) *Sender {
	cli, err := client.NewSource("Socket.io Client").Connect(host, port)
	if err != nil {
		log.Printf("❌ Connect to the zipper server on %s failure with err: %v", zipperAddr, err)
		return nil
	}

	return &Sender{
		Stream: cli,
	}
}

func (s *Sender) BindConnectionAsStreamDataSource(server *socketio.Server) {
	// when new user connnected, add them to socket.io room
	server.OnConnect("/", func(s socketio.Conn) error {
		userID := getConnectionID(s)
		log.Printf("[%s] Connected", userID)
		s.SetContext(userID)

		s.Join(lib.RoomID)

		s.Emit("init", s.ID())

		return nil
	})

	// when user disconnect, leave them from socket.io room,
	// and notify to others in this room
	server.OnDisconnect("/", func(conn socketio.Conn, reason string) {
		server.LeaveRoom("/", lib.RoomID, conn)

		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("/", lib.RoomID, "offline", conn.Context())

		// broadcast to other yomo-zipper nodes
		s.BroadcastOfflineEvent(lib.RoomID, "offline", fmt.Sprintf("%s", conn.Context()))
	})

	server.OnEvent("/", "movement", func(conn socketio.Conn, movement string) {
		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("/", lib.RoomID, "movement", movement)

		// send event data to `yomo-zipper` for broadcasting to other mesh nodes
		data := lib.EventData{
			Room:  lib.RoomID,
			Event: "movement",
			Data:  movement,
		}
		s.send(data)
	})

	server.OnEvent("/", "online", func(conn socketio.Conn, state string) {
		// broadcast the data to local users via socket.io directly.
		server.BroadcastToRoom("/", lib.RoomID, "online", state)

		// send event data to `yomo-zipper` for broadcasting to other mesh nodes
		data := lib.EventData{
			Room:  lib.RoomID,
			Event: "online",
			Data:  state,
		}
		s.send(data)

	})
}

func (s *Sender) BroadcastOfflineEvent(room string, eventName string, userID string) {
	s.send(lib.EventData{
		Room:  room,
		Event: eventName,
		Data:  userID,
	})
}

// send the data to `yomo-zipper`.
func (s *Sender) send(data lib.EventData) {
	// init a new Y3 codec.
	var codec = y3.NewCodec(lib.EventDataKey)

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

func getConnectionID(conn socketio.Conn) string {
	return fmt.Sprint(os.Getenv("MESH_ID"), "-", conn.ID())
}
