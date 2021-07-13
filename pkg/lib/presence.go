package lib

// PresenceBase is the base structure for presence
type PresenceBase struct {
	Room      string `y3:"0x11"`
	Event     string `y3:"0x12"`
	Timestamp int64  `y3:"0x13"`
}

// PresenceOnline event will be sent to all users when a user goes online
type PresenceOnline struct {
	Base PresenceBase `y3:"0x21"`
	Name string       `y3:"0x22"`
}

// PresenceSync event will be sent to all users when a user need sync state
type PresenceSync struct {
	Base     PresenceBase `y3:"0x21"`
	Position Position     `y3:"0x22"`
}

// Position represents by (x,y) corrdinate of user
type Position struct {
	X int64 `y3:"0x31"`
	Y int64 `y3:"0x32"`
}
