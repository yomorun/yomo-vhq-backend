package receiver

import (
	"log"
	"os"

	"github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/client"
	"github.com/yomorun/yomo/pkg/rx"
	"yomo.run/vhq/pkg/lib"
)

var logger *log.Logger = log.New(os.Stdout, "[Receiver] ", log.LstdFlags)

// setupReceiver connects to `yomo-zipper` as a `yomo-sink`.
// receiver will receive the data from yomo-zipper after stream processing and broadcast it to socket.io clients.
func NewReceiver(host string, port int) error {
	logger.Printf("------------Receiver init------------ host=%s, port=%d", host, port)
	cli, err := client.NewServerless("Receiver").Connect(host, port)
	if err != nil {
		logger.Printf("❌ Connect to the zipper server on [%s:%d] failure with err: %v", host, port, err)
		return err
	}
	// defer cli.Close()

	go cli.Pipe(receiverHandler)

	return nil
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
	return mold, nil
}

// receiverHandler will handle data in Rx way
func receiverHandler(rxstream rx.RxStream) rx.RxStream {
	logger.Println(">>>>>>>>>> receiverHandler <<<<<<<<<<<<<<")
	stream := rxstream.
		Subscribe(0x10).
		OnObserve(broadcastEventToUsers).
		Encode(0x12)

	return stream
}
