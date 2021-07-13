package receiver

import (
	"context"

	color "github.com/fatih/color"
	socketio "github.com/googollee/go-socket.io"
	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"github.com/yomorun/yomo/pkg/rx"

	"yomo.run/vhq/pkg/lib"
)

// var logger *log.Logger = log.New(os.Stdout, "[Receiver] ", log.LstdFlags)
var log = color.New(color.FgCyan)

var ws *socketio.Server

// setupReceiver connects to `yomo-zipper` as a `yomo-sink`.
// receiver will receive the entity from yomo-zipper after stream processing and broadcast it to socket.io clients.
func NewReceiver(host string, port int, websocket *socketio.Server) error {
	log.Printf("------------Receiver init------------ host=%s, port=%d\n", host, port)
	cli, err := client.NewServerless("PresenceHandler").Connect(host, port)
	if err != nil {
		log.Printf("❌ Connect to the zipper server on [%s:%d] failure with err: %v\n", host, port, err)
		return err
	}

	go cli.Pipe(receiverHandler)

	hookToSocketIO(websocket)

	return nil
}

func hookToSocketIO(websocket *socketio.Server) {
	ws = websocket
}

// broadcast the events the geo-distributed users.
var decode = func(v []byte) (interface{}, error) {
	log.Printf("decode : %v\n", v)
	// broadcast the entity to local users via socket.io directly.
	var presence lib.Presence
	err := y3.ToObject(v, &presence)
	if err != nil {
		log.Printf("❌ Decode the presence to struct failure with err: %v\n", err)
		return nil, err
	}

	return presence, nil
}

var messageHandler = func(_ context.Context, v interface{}) (interface{}, error) {
	log.Printf("messageHandler: %v\n", v)

	presence := v.(lib.Presence)

	switch presence.Event {
	case "online":
		processEventOnline(presence)
	case "offline":
		processEventOffline(presence)
	case "movement":
		processMovement(presence)
	}

	return v, nil
}

// receiverHandler will handle entity in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	stream := rxstream.
		Subscribe(0x10).
		OnObserve(decode).
		Map(messageHandler)

	return stream
}

// handle "online" event
func processEventOnline(presence lib.Presence) {
	log.Printf("process event Online, presence: %v\n", presence)
	name := string(presence.Payload)
	data := &map[string]interface{}{"name": name, "timestamp": presence.Timestamp}
	ws.BroadcastToRoom("/", lib.RoomID, "online", data)
	ws.BroadcastToRoom("/", lib.RoomID, "ask")
}

// handle "offline" event
func processEventOffline(presence lib.Presence) {
	log.Printf("process event Offline, presence: %v\n", presence)
	name := string(presence.Payload)
	data := &map[string]interface{}{"name": name}
	ws.BroadcastToRoom("/", lib.RoomID, "offline", data)
}

// handle "movement" event
func processMovement(presence lib.Presence) {
	log.Printf("process event Movement, presence: %v\n", presence)
	// decode presence.payload from []byte to PresenceMovement
	var movement lib.PresenceMovement
	err := y3.ToObject(presence.Payload, &movement)

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
		ws.BroadcastToRoom("/", lib.RoomID, "movement", data)
	}
}
