package lib

// PresenceData represents the event data for broadcasting to all geo-distributed users.
type PresenceData struct {
	Room    string                 `json:"room"`
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload"`
}
