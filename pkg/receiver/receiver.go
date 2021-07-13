package receiver

import (
	"log"
	"os"

	socketio "github.com/googollee/go-socket.io"

	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"github.com/yomorun/yomo/pkg/rx"
	"yomo.run/vhq/pkg/lib"
)

var logger *log.Logger = log.New(os.Stdout, "[Receiver] ", log.LstdFlags)

var ws *socketio.Server

// setupReceiver connects to `yomo-zipper` as a `yomo-sink`.
// receiver will receive the data from yomo-zipper after stream processing and broadcast it to socket.io clients.
func NewReceiver(host string, port int, websocket *socketio.Server) error {
	logger.Printf("------------Receiver init------------ host=%s, port=%d", host, port)
	cli, err := client.NewServerless("Receiver").Connect(host, port)
	if err != nil {
		logger.Printf("❌ Connect to the zipper server on [%s:%d] failure with err: %v", host, port, err)
		return err
	}
	// defer cli.Close()

	go cli.Pipe(receiverHandler)

	hookToSocketIO(websocket)

	return nil
}

func hookToSocketIO(websocket *socketio.Server) {
	ws = websocket
}

// broadcast the events the geo-distributed users.
var broadcastEventToUsers = func(v []byte) (interface{}, error) {
	logger.Printf("=====broadcastEventToUsers : %v", v)

	var mold lib.PresenceOnline
	err := y3.ToObject(v, &mold)
	if err != nil {
		logger.Printf("❌ Decode the data to struct failure with err: %v", err)
		return nil, err
	}

	logger.Printf("=====broadcastEventToUsers mold: %v", mold)

	switch mold.Base.Event {
	case "online":
		processEventOnline(mold)
	}

	return mold, nil
}

// receiverHandler will handle data in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	logger.Println(">>>>>>>>>> receiverHandler <<<<<<<<<<<<<<")
	stream := rxstream.
		Subscribe(0x11).
		OnObserve(broadcastEventToUsers).
		Encode(0x12)

	return stream
}

func processEventOnline(mold lib.PresenceOnline) {
	logger.Printf("=====processEventOnline mold: %v", mold)
	// broadcast the data to local users via socket.io directly.
	ws.BroadcastToRoom("/", lib.RoomID, "online", &map[string]interface{}{"name": mold.Name, "timestamp": mold.Base.Timestamp})
	ws.BroadcastToRoom("/", lib.RoomID, "ask")
}
