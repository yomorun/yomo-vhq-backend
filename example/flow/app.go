package main

import (
	y3 "github.com/yomorun/y3-codec-golang"
	"github.com/yomorun/yomo/pkg/rx"
)

var callback = func(v []byte) (interface{}, error) {
	return y3.ToUTF8String(v)
}

// Handler will handle data in Rx way
func Handler(rxstream rx.RxStream) rx.RxStream {
	stream := rxstream.
		Subscribe(0x10).
		OnObserve(callback)

	return stream
}
