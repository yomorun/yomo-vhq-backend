package receiver

import (
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
var broadcastEventToUsers = func(v []byte) (interface{}, error) {
	log.Printf("broadcastEventToUsers : %v\n", v)
	// broadcast the entity to local users via socket.io directly.
	var entity lib.PresenceOnline
	err := y3.ToObject(v, &entity)
	if err != nil {
		log.Printf("❌ Decode the entity to struct failure with err: %v\n", err)
		return nil, err
	}

	log.Printf("broadcastEventToUsers entity: %v\n", entity)

	switch entity.Base.Event {
	case "online":
		processEventOnline(entity)
	case "offline":
		processEventOffline(entity)
	}

	return entity, nil
}

// receiverHandler will handle entity in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	stream := rxstream.
		Subscribe(0x10).
		OnObserve(broadcastEventToUsers)

	return stream
}

// handle "online" event
func processEventOnline(entity lib.PresenceOnline) {
	log.Printf("process event Online, entity: %v\n", entity)
	presence := &map[string]interface{}{"name": entity.Name, "timestamp": entity.Base.Timestamp}
	ws.BroadcastToRoom("/", lib.RoomID, "online", presence)
	ws.BroadcastToRoom("/", lib.RoomID, "ask")
}

// handle "offline" event
func processEventOffline(entity lib.PresenceOnline) {
	log.Printf("process event Offline, entity: %v\n", entity)
	presence := &map[string]interface{}{"name": entity.Name}
	ws.BroadcastToRoom("/", lib.RoomID, "offline", presence)
}
