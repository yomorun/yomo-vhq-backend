package lib

// EventData represents the event data for broadcasting to all geo-distributed users.
type EventData struct {
	ServerRegion string `y3:"0x11"`
	Room         string `y3:"0x12"`
	Event        string `y3:"0x13"`
	Data         string `y3:"0x14"`
}
