package socket

type WebSocketConnection interface {
	Close() error
	SetReadLimit(limit int64)
	SetPingHandler(h func(appData string) error)
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
}
