package sender

import (
	"encoding/json"
	"time"

	color "github.com/fatih/color"
	socketio "github.com/googollee/go-socket.io"
	"github.com/yomorun/yomo"

	"yomo.run/vhq/pkg/lib"
)

type onlineState struct {
	userID string
	roomID string
}

var sender yomo.Source
var logger = color.New(color.FgYellow)

// NewSender send presence as stream to yomo-send-server
func NewSender(zipperAddress string, server *socketio.Server) {
	sender = yomo.NewSource("yomo-source", yomo.WithZipperAddr(zipperAddress))

	err := sender.Connect()
	if err != nil {
		logger.Printf("[source] ❌ Connect to YoMo-Zipper failure with err: %v\n", err)
	} else {
		logger.Printf("[source] ✅ Connect to YoMo-Zipper\n")
	}

	sender.SetDataTag(0x10)

	bindConnection(server)
}

func bindConnection(server *socketio.Server) {
	// when new user connnected, add them to socket.io room
	server.OnConnect("/", func(conn socketio.Conn) error {
		logger.Printf("EVT | OnConnect | conn.ID=[%s]\n", conn.ID())
		return nil
	})

	// when user disconnect, leave them from socket.io room
	// and notify to others in this room
	server.OnDisconnect("/", func(conn socketio.Conn, reason string) {
		if conn.Context() == nil {
			logger.Println("[onDisconnect] empty conn context")
			return
		}
		state := conn.Context().(*onlineState)
		logger.Printf("[%s-%s] | EVT | OnDisconnect | \n", state.userID, state.roomID)

		conn.LeaveAll()
		if conn.Context() == nil {
			return
		}

		// broadcast to all receivers I am offline
		dispatchToReceivers(lib.Presence{
			Room:      state.roomID,
			Event:     "offline",
			Timestamp: time.Now().Unix(),
			Payload:   []byte(state.userID),
		})

		// leave from socket.io room
		server.LeaveRoom("/", state.roomID, conn)
	})

	// browser will emit "online" event when user connected to WebSocket, with payload:
	// {name: "USER_ID", avatar: "URL", room: "ROOM_ID"}
	server.OnEvent("/", "online", func(conn socketio.Conn, payload interface{}) {
		// get the userID, roomID from websocket
		var signal = payload.(map[string]interface{})
		userID := signal["name"].(string)
		roomID := "void"
		if _, ok := signal["room"]; ok {
			roomID = signal["room"].(string)
		}
		logger.Printf("[%s][%s] | EVT | online | %v\n", userID, roomID, signal)

		// join room
		conn.Join(roomID)

		// store userID to websocket connection context
		conn.SetContext(&onlineState{userID: userID, roomID: roomID})

		dispatchToReceivers(lib.EncodeOnline(userID, signal["avatar"].(string), roomID))
	})

	// browser will emit "movement" event when user moving around, with payload:
	// {name: "USER_ID", dir: {x:1, y:0}}
	server.OnEvent("/", "movement", func(conn socketio.Conn, payload interface{}) {
		state := conn.Context().(*onlineState)
		logger.Printf("[%s-%s] | EVT | movement | %v - (%T)\n", state.userID, state.roomID, payload, payload)

		signal := payload.(map[string]interface{})
		dir := signal["dir"].(map[string]interface{})

		// broadcast to all receivers
		dispatchToReceivers(lib.EncodeMovement(state.userID, dir["x"].(float64), dir["y"].(float64), state.roomID))
	})

	// browser will emit "sync" event to tell others my position, the payload looks like
	// {name: "USER_ID", pos: {x: 0, y: 0}}
	server.OnEvent("/", "sync", func(conn socketio.Conn, payload interface{}) {
		state := conn.Context().(*onlineState)
		logger.Printf("[%s-%s] | EVT | sync | %v - (%T)\n", state.userID, state.roomID, payload, payload)

		signal := payload.(map[string]interface{})
		pos := signal["pos"].(map[string]interface{})

		// broadcast to all receivers
		dispatchToReceivers(lib.EncodeSync(state.userID, pos["x"].(float64), pos["y"].(float64), signal["avatar"].(string), state.roomID))
	})
}

// dispatch data to all downstream Presence-Receiver Servers
func dispatchToReceivers(payload lib.Presence) {
	sendingBuf, err := json.Marshal(&payload)
	if err != nil {
		logger.Printf("dispatchToReceivers json.Marshal error: %v", err)
	}

	_, err = sender.Write(sendingBuf)
	if err != nil {
		logger.Printf("dispatchToReceivers sender.Write error: %v", err)
	}
}
