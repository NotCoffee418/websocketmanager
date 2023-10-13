package websocketmanager

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ManagerBuilder is a builder for the websocket manager
type ManagerBuilder struct {
	wsm Manager
}

// Manager is a websocket manager
type Manager struct {
	clients           map[uuid.UUID]*Client
	mutex             *sync.Mutex // Protects clients map
	wsReadBufferSize  int
	wsWriteBufferSize int
	allowCleanup      bool
}

// Client contains information about a websocket user
type Client struct {
	Conn           *websocket.Conn
	ConnId         *uuid.UUID
	Groups         []uint16
	IsAlive        bool
	CancelFunc     context.CancelFunc
	serverSentChan chan *websocketMessage
	observers      []ClientObservableFunc
	mutex          *sync.Mutex // Protects communication chans
}

// ClientObservableFunc is a function that will be called when a client sends a message
type ClientObservableFunc func(wsClient *Client, messageType int, message []byte)

// WS Message type for internal usage
type websocketMessage struct {
	Message     []byte
	MessageType int
}
