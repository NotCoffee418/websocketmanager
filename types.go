package websocketmanager

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"sync"
)

// ManagerBuilder is a builder for the websocket manager
type ManagerBuilder struct {
	wsm Manager
}

// Manager is a websocket manager
type Manager struct {
	clients           map[uuid.UUID]*Client
	mutex             *sync.Mutex
	wsReadBufferSize  int
	wsWriteBufferSize int
	allowCleanup      bool
}

// Client is a client that is connected to the websocket
type Client struct {
	Conn    *websocket.Conn
	ConnId  *uuid.UUID
	Groups  []uint16
	IsAlive bool
}

// ClientObservableFunc is a function that will be called when a client sends a message
type ClientObservableFunc func(wsClient *Client, messageType int, message []byte)
