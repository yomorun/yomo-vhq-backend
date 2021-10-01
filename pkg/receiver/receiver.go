package receiver

import (
	"encoding/json"

	color "github.com/fatih/color"
	socketio "github.com/googollee/go-socket.io"
	"github.com/yomorun/yomo"

	"yomo.run/vhq/pkg/lib"
)

// var logger *log.Logger = log.New(os.Stdout, "[Receiver] ", log.LstdFlags)
var log = color.New(color.FgCyan)

var ws *socketio.Server

// setupReceiver connects to `yomo-zipper` as a `yomo-sink`.
// receiver will receive the entity from yomo-zipper after stream processing and broadcast it to socket.io clients.
func NewReceiver(zipperAddress string, websocket *socketio.Server) error {
	log.Printf("------------Receiver init------------ zipper=%s", zipperAddress)
	sfn := yomo.NewStreamFunction("PresenceHandler", yomo.WithZipperAddr(zipperAddress))

	sfn.SetObserveDataID(0x10)

	sfn.SetHandler(handler)

	// start
	err := sfn.Connect()
	if err != nil {
		log.Printf("[flow] connect err=%v", err)
		return err
	}

	ws = websocket

	return nil
}

func handler(data []byte) (byte, []byte) {
	var presence lib.Presence
	err := json.Unmarshal(data, &presence)
	if err != nil {
		log.Printf("handler json.Unmarshal error: %v", err)
	}

	log.Printf("handler: deocde data: %v", presence)

	switch presence.Event {
	case "online":
		processEventOnline(presence)
	case "offline":
		processEventOffline(presence)
	case "movement":
		processMovement(presence)
	case "sync":
		processSync(presence)
	}

	return 0x0, nil
}

// handle "online" event
func processEventOnline(presence lib.Presence) {
	log.Printf("process event Online, presence: %v\n", presence)
	// decode presence.payload from []byte to PresenceOnlineState
	var online lib.PresenceOnlineState
	err := json.Unmarshal(presence.Payload, &online)
	if err != nil {
		log.Printf("(processMovement) Decode the presence.payload to PresenceMovement failure with err: %v\n", err)
	} else {
		data := &map[string]interface{}{"name": online.Name, "timestamp": presence.Timestamp, "avatar": online.Avatar, "mesh_id": online.MeshID}
		ws.BroadcastToRoom("/", presence.Room, "online", data)
		ws.BroadcastToRoom("/", presence.Room, "ask")
	}
}

// handle "offline" event
func processEventOffline(presence lib.Presence) {
	log.Printf("process event Offline, presence: %v\n", presence)
	name := string(presence.Payload)
	data := &map[string]interface{}{"name": name}
	ws.BroadcastToRoom("/", presence.Room, "offline", data)
}

// handle "movement" event
func processMovement(presence lib.Presence) {
	log.Printf("process event Movement, presence: %v\n", presence)
	// decode presence.payload from []byte to PresenceMovement
	var movement lib.PresenceMovement
	err := json.Unmarshal(presence.Payload, &movement)

	if err != nil {
		log.Printf("(processMovement) Decode the presence.payload to PresenceMovement failure with err: %v\n", err)
	} else {
		log.Printf("(processMovement) Decode the presence.payload to PresenceMovement: %v\n", movement)
		data := &map[string]interface{}{
			"name": movement.Name,
			"dir": &map[string]interface{}{
				"x": movement.Direction.X,
				"y": movement.Direction.Y,
			},
		}
		ws.BroadcastToRoom("/", presence.Room, "movement", data)
	}
}

// handle "sync" event
func processSync(presence lib.Presence) {
	log.Printf("process event Sync, presence: %v\n", presence)
	// decode presence.payload from []byte to PresenceSync
	var sync lib.PresenceSync
	err := json.Unmarshal(presence.Payload, &sync)

	if err != nil {
		log.Printf("(processSync) Decode the presence.payload to PresenceSync failure with err: %v\n", err)
	} else {
		log.Printf("(processSync) Decode the presence.payload to PresenceSync: %v\n", sync)
		data := &map[string]interface{}{
			"name": sync.Name,
			"pos": &map[string]interface{}{
				"x": sync.Position.X,
				"y": sync.Position.Y,
			},
			"avatar": sync.Avatar,
		}
		ws.BroadcastToRoom("/", presence.Room, "sync", data)
	}
}
