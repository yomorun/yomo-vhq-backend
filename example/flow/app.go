package main

import (
	"github.com/yomorun/yomo/pkg/rx"
)

// Handler will handle data in Rx way
func Handler(rxstream rx.RxStream) rx.RxStream {
	stream := rxstream.
		Y3Decoder("0x10", string("")).
		StdOut()

	return stream
}
