package websocketmanager

import (
	"github.com/google/uuid"
	"sync"
)

// NewDefaultManager Creates a new Manager with default values
func NewDefaultManager() *Manager {
	return NewBuilder().Build()
}

func NewBuilder() *ManagerBuilder {
	return &ManagerBuilder{
		wsm: Manager{
			clients:           make(map[uuid.UUID]*Client),
			mutex:             &sync.Mutex{},
			wsReadBufferSize:  1024,
			wsWriteBufferSize: 1024,
			allowCleanup:      true,
		},
	}
}

// WithReadBufferSize is a builder for the websocket manager
func (b *ManagerBuilder) WithReadBufferSize(size int) *ManagerBuilder {
	b.wsm.wsReadBufferSize = size
	return b
}

// WithWriteBufferSize sets the write buffer size for the websocket
func (b *ManagerBuilder) WithWriteBufferSize(size int) *ManagerBuilder {
	b.wsm.wsWriteBufferSize = size
	return b
}

// WithClientCleanupDisabled disables the periodic pinging of clients to check if they are still alive
func (b *ManagerBuilder) WithClientCleanupDisabled() *ManagerBuilder {
	b.wsm.allowCleanup = false
	return b
}

func (b *ManagerBuilder) Build() *Manager {
	mgr := &b.wsm
	mgr.initialize()
	return mgr
}
