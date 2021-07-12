package receiver

import (
	"context"
	"encoding/json"
	"log"
	"os"

	socketio "github.com/googollee/go-socket.io"
	"github.com/yomorun/yomo/pkg/client"
	"github.com/yomorun/yomo/pkg/rx"
	"yomo.run/vhq/pkg/lib"
)

// sender sends the VHQ events to yomo-zipper for stream processing.
type Receiver struct {
	client client.ServerlessClient
}

var logger *log.Logger = log.New(os.Stdout, "[Receiver] ", log.LstdFlags)

// setupReceiver connects to `yomo-zipper` as a `yomo-sink`.
// receiver will receive the data from yomo-zipper after stream processing and broadcast it to socket.io clients.
func NewReceiver(host string, port int) (*Receiver, error) {
	logger.Printf("------------Receiver init------------ host=%s, port=%d", host, port)
	cli, err := client.NewServerless("Receiver").Connect(host, port)
	if err != nil {
		logger.Printf("❌ Connect to the zipper server on [%s:%d] failure with err: %v", host, port, err)
		return nil, err
	}
	defer cli.Close()

	go cli.Pipe(receiverHandler)

	return &Receiver{
		client: cli,
	}, nil
}

func (r *Receiver) BindConnectionPresenceStreamProcessing(server *socketio.Server) {

}

// // decode the data via Y3 Codec.
// var decode = func(v []byte) (interface{}, error) {
// 	var mold lib.EventData
// 	err := y3.ToObject(v, &mold)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return mold, nil
// }

// broadcast the events the geo-distributed users.
var broadcastEventToUsers = func(_ context.Context, i interface{}) (interface{}, error) {
	value := i.(*lib.PresenceData)
	logger.Printf("broadcastEventToUsers : %v", value)
	// x, ok := i.(lib.EventData)
	// if !ok {
	// 	err := fmt.Sprintf("expected type 'EventData', got '%v' instead",
	// 		reflect.TypeOf(i))
	// 	log.Printf("❌ %v", err)
	// 	return nil, errors.New(err)
	// }

	// if x.Event == "move" {
	// 	updatePlayerMovement(x)
	// }

	// // broadcast message to all connected users in other regions.
	// if socketioServer != nil && x.ServerRegion != serverRegion {
	// 	socketioServer.BroadcastToRoom("", x.Room, x.Event, x.Data)
	// }

	return value, nil
}

// receiverHandler will handle data in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	logger.Println(">>>>>>>>>> receiverHandler <<<<<<<<<<<<<<")
	stream := rxstream.
		Unmarshal(json.Unmarshal, func() interface{} { return &lib.PresenceData{} }).
		Map(broadcastEventToUsers)

	return stream
}

// // updatePlayerMovement updates the movement action in players list.
// func updatePlayerMovement(x lib.EventData) {
// 	var action Action

// 	err := json.Unmarshal([]byte(x.Data), &action)
// 	if err != nil {
// 		log.Printf("❌ Unmarshal the movement action failed: %v", err)
// 		return
// 	}

// 	p := localPlayersCache[action.ID]
// 	if p.ID != "" {
// 		p.X = action.X
// 		p.Y = action.Y
// 		localPlayersCache[action.ID] = p
// 	}
// }
