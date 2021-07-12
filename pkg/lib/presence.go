package lib

// PresenceData represents the event data for broadcasting to all geo-distributed users.
type PresenceData struct {
	Room    string                 `json:"room"`
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload"`
}

type Presence struct {
	Room  string `y3:"0x11"`
	Event string `y3:"0x12"`
}

type PresenceOnline struct {
	Room      string `y3:"0x11"`
	Event     string `y3:"0x12"`
	Name      string `y3:"0x21"`
	Timestamp int64  `y3:"0x22"`
}
