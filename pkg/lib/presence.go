package lib

import (
	"time"

	"github.com/yomorun/y3-codec-golang"
)

const RoomID = "yomo-vhq"

// PresenceBase is the base structure for presence
type Presence struct {
	Room      string `y3:"0x11"`
	Event     string `y3:"0x12"`
	Timestamp int64  `y3:"0x13"`
	Payload   []byte `y3:"0x14"`
}

// PresenceOnline event will be sent to all users when a user goes online
type PresenceOnlineState struct {
	Name   string `y3:"0x21"`
	Avatar string `y3:"0x22"`
}

// PresenceMovement is sent to all users when a user moves
type PresenceMovement struct {
	Name      string `y3:"0x21"`
	Direction Vector `y3:"0x22"`
}

// PresenceMovement is sent to all users when a user moves
type PresenceSync struct {
	Name     string `y3:"0x21"`
	Position Vector `y3:"0x22"`
	Avatar   string `y3:"0x23"`
}

// Position represents by (x,y) corrdinate of user
type Vector struct {
	X float64 `y3:"0x31"`
	Y float64 `y3:"0x32"`
}

func EncodeMovement(name string, x float64, y float64) (Presence, error) {
	codec := y3.NewCodec(0x30)
	buf, err := codec.Marshal(PresenceMovement{
		Name:      name,
		Direction: Vector{X: x, Y: y},
	})

	return Presence{
		Room:      RoomID,
		Event:     "movement",
		Timestamp: time.Now().Unix(),
		Payload:   buf,
	}, err
}

func EncodeSync(name string, x float64, y float64, avatar string) (Presence, error) {
	codec := y3.NewCodec(0x30)

	buf, err := codec.Marshal(PresenceSync{
		Name:     name,
		Position: Vector{X: x, Y: y},
		Avatar:   avatar,
	})

	return Presence{
		Room:      RoomID,
		Event:     "sync",
		Timestamp: time.Now().Unix(),
		Payload:   buf,
	}, err
}

func EncodeOnline(name string, avatar string) (Presence, error) {
	codec := y3.NewCodec(0x30)

	buf, err := codec.Marshal(PresenceOnlineState{
		Name:   name,
		Avatar: avatar,
	})

	return Presence{
		Room:      RoomID,
		Event:     "online",
		Timestamp: time.Now().Unix(),
		Payload:   buf,
	}, err
}
