package lib

import (
	"time"
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
	Name string `y3:"0x21"`
}

// PresenceMovement is sent to all users when a user moves
type PresenceMovement struct {
	Name      string   `y3:"0x21"`
	Direction Position `y3:"0x22"`
}

// PresenceSync event will be sent to all users when a user need sync state
type PresenceSync struct {
	Position Position `y3:"0x21"`
}

// Position represents by (x,y) corrdinate of user
type Position struct {
	X int `y3:"0x31"`
	Y int `y3:"0x32"`
}

func EncodeOnlineOfflineState(event string, name string) Presence {
	// // encode name to y3, as the payload of presence
	// encoder := y3.NewPrimitivePacketEncoder(0x14)
	// encoder.SetBytes([]byte(name))

	return Presence{
		Room:      RoomID,
		Event:     event,
		Timestamp: time.Now().Unix(),
		// Payload:   encoder.Encode(),
		Payload: []byte(name),
	}
}
