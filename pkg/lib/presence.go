package lib

import (
	"encoding/json"
	"os"
	"time"
)

// PresenceBase is the base structure for presence
type Presence struct {
	Room      string `json:"room"`
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Payload   []byte `json:"payload"`
}

// PresenceOnline event will be sent to all users when a user goes online
type PresenceOnlineState struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	MeshID string `json:"meshID"`
}

// PresenceMovement is sent to all users when a user moves
type PresenceMovement struct {
	Name      string `json:"name"`
	Direction Vector `json:"direction"`
}

// PresenceMovement is sent to all users when a user moves
type PresenceSync struct {
	Name     string `json:"name"`
	Position Vector `json:"position"`
	Avatar   string `json:"avatar"`
}

// Position represents by (x,y) corrdinate of user
type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func EncodeMovement(name string, x float64, y float64, roomID string) Presence {
	buf, _ := json.Marshal(PresenceMovement{
		Name:      name,
		Direction: Vector{X: x, Y: y},
	})
	return Presence{
		Room:      roomID,
		Event:     "movement",
		Timestamp: time.Now().Unix(),
		Payload:   buf,
	}
}

func EncodeSync(name string, x float64, y float64, avatar string, roomID string) Presence {
	buf, _ := json.Marshal(PresenceSync{
		Name:     name,
		Position: Vector{X: x, Y: y},
		Avatar:   avatar,
	})
	return Presence{
		Room:      roomID,
		Event:     "sync",
		Timestamp: time.Now().Unix(),
		Payload:   buf,
	}
}

func EncodeOnline(name string, avatar string, roomID string) Presence {
	buf, _ := json.Marshal(PresenceOnlineState{
		Name:   name,
		Avatar: avatar,
		MeshID: os.Getenv("MESH_ID"),
	})
	return Presence{
		Room:      roomID,
		Event:     "online",
		Timestamp: time.Now().Unix(),
		Payload:   buf,
	}
}
