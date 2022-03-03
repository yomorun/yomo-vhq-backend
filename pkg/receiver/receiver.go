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
func NewReceiver(zipperAddress string, websocket *socketio.Server, appID string, appSecret string) error {
	log.Printf("------------Receiver init------------ zipper=%s\n", zipperAddress)
	sfn := yomo.NewStreamFunction("PresenceHandler",
		yomo.WithZipperAddr(zipperAddress),
		yomo.WithObserveDataTags(0x10),
		yomo.WithAppKeyCredential(appID, appSecret),
	)

	sfn.SetHandler(handler)

	// start
	err := sfn.Connect()
	if err != nil {
		log.Printf("[flow] connect err=%v\n", err)
		return err
	}

	ws = websocket

	return nil
}

func handler(data []byte) (byte, []byte) {
	var presence lib.Presence
	err := json.Unmarshal(data, &presence)
	if err != nil {
		log.Printf("handler json.Unmarshal error: %v\n", err)
	}

	log.Printf("handler: deocde data: %v\n", presence)

	switch presence.Event {
	case "online":
		processEventOnline(presence)
	case "offline":
		processEventOffline(presence)
	case "movement":
		processMovement(presence)
	case "sync":
		processSync(presence)
	case "ding":
		processDing(presence)
	case "latency":
		processLatency(presence)
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
		log.Printf("(processOnline) Decode the presence.payload to PresenceOnline failure with err: %v\n", err)
	} else {
		data := &map[string]interface{}{"name": online.Name, "timestamp": presence.Timestamp, "avatar": online.Avatar, "mesh_id": online.MeshID, "country": online.Country}
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
			"timestamp": movement.Timestamp,
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
		log.Printf("(processSync) Decode the presence.payload to PresenceSync: %+v\n", sync)
		data := &map[string]interface{}{
			"name": sync.Name,
			"pos": &map[string]interface{}{
				"x": sync.Position.X,
				"y": sync.Position.Y,
			},
			"avatar":    sync.Avatar,
			"country":   sync.Country,
			"timestamp": sync.Timestamp,
		}
		ws.BroadcastToRoom("/", presence.Room, "sync", data)
	}
}

// handle "ping" event
func processDing(presence lib.Presence) {
	log.Printf("process event Ping, presence: %v\n", presence)
	data := map[string]interface{}{}
	err := json.Unmarshal(presence.Payload, &data)
	if err != nil {
		log.Printf("(processPing) Decode the presence.payload to PresencePing failure with err: %v\n", err)
	} else {
		ws.BroadcastToRoom("/", presence.Room, "dang", &data)
	}
}

// handle "latency" event
func processLatency(presence lib.Presence) {
	log.Printf("process event Latency, presence: %v\n", presence)
	data := map[string]interface{}{}
	err := json.Unmarshal(presence.Payload, &data)
	if err != nil {
		log.Printf("(processLatency) Decode the presence.payload to PresenceLatency failure with err: %v\n", err)
	} else {
		ws.BroadcastToRoom("/", presence.Room, "latency", &data)
	}
}
