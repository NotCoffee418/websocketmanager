package websocketmanager

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

// initialize initializes the websocket manager
// This function is called by the builder
func (wm *Manager) initialize() {
	if wm.allowCleanup {
		go wm.pruneInactiveClients()
	}
}

// UpgradeClientCh upgrades a client connection to a websocket connection
// It registers the client and returns a channel that will receive the Client or nil on error
func (wm *Manager) UpgradeClientCh(w http.ResponseWriter, r *http.Request) chan *Client {
	upgradeChan := make(chan *Client, 1)
	go func() {
		defer close(upgradeChan)
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  wm.wsReadBufferSize,
			WriteBufferSize: wm.wsWriteBufferSize,
		}
		wsConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "Error creating websocket connection")
			upgradeChan <- nil
			return
		}
		upgradeChan <- wm.register(wsConn)
	}()
	return upgradeChan
}

// AssignGroups assigns a client to one or more groups
func (wm *Manager) AssignGroups(connId uuid.UUID, groupIds ...uint16) error {
	wsClient, err := wm.GetWebSocketClient(connId)
	if err != nil {
		return err
	}
	wsClient.Groups = append(wsClient.Groups, groupIds...)
	return nil
}

func (wm *Manager) RegisterClientObserver(connId uuid.UUID, messageHandler ClientObservableFunc) {
	wsUser, err := wm.GetWebSocketClient(connId)
	if err != nil {
		log.Debugf("Websocket RegisterClientObserver error: %s", err)
		return
	}
	go func() {
		for wsUser.IsAlive {
			messageType, message, err := wsUser.Conn.ReadMessage()
			if err != nil {
				log.Debugf("Websocket RegisterClientObserver error: %s", err)
				wm.Unregister(connId)
				return
			}
			messageHandler(wsUser, messageType, message)
		}
	}()
}

// BroadcastMessage sends a message to all connected clients
func (wm *Manager) BroadcastMessage(messageType int, message []byte) {
	clients := make([]*Client, 0)
	wm.mutex.Lock()
	for _, client := range wm.clients {
		if client.IsAlive {
			clients = append(clients, client)
		}
	}
	wm.mutex.Unlock()
	go wm.multiCastMessage(clients, messageType, message)
}

// SendMessageToGroup sends a message to all users in a group
func (wm *Manager) SendMessageToGroup(group uint16, messageType int, message []byte) {
	targetClients := make([]*Client, 0)
	wm.mutex.Lock()
	for _, client := range wm.clients {
		if !client.IsAlive {
			continue
		}
		for _, userGroup := range client.Groups {
			if userGroup == group {
				targetClients = append(targetClients, client)
				break
			}
		}
	}
	wm.mutex.Unlock()
	go wm.multiCastMessage(targetClients, messageType, message)
}

// SendMessageToClient sends a message to a specific user
func (wm *Manager) SendMessageToClient(clientId uuid.UUID, messageType int, message []byte) error {
	wm.mutex.Lock()
	client, ok := wm.clients[clientId]
	wm.mutex.Unlock()
	if !ok {
		return errors.New("client not found")
	}
	if err := client.Conn.WriteMessage(messageType, message); err != nil {
		wm.Unregister(clientId)
		return err
	}
	return nil
}

// GetWebSocketClient returns a Client by UUID
func (wm *Manager) GetWebSocketClient(clientUUID uuid.UUID) (*Client, error) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	user, ok := wm.clients[clientUUID]
	if !ok {
		return nil, errors.New("client not found")
	}
	return user, nil
}

// register registers a new client to the manager
// This function is called by the UpgradeClient function
func (wm *Manager) register(conn *websocket.Conn) *Client {
	connId := uuid.New()
	user := &Client{
		Conn:    conn,
		ConnId:  &connId,
		Groups:  make([]uint16, 0),
		IsAlive: true,
	}
	wm.mutex.Lock()
	wm.clients[connId] = user
	wm.mutex.Unlock()
	return user
}

// Unregister unregisters a client from the manager and marks it as not alive
// Further messages to and from this client are discarded
func (wm *Manager) Unregister(connId uuid.UUID) {
	wm.mutex.Lock()
	wm.clients[connId].IsAlive = false
	delete(wm.clients, connId)
	wm.mutex.Unlock()
}

// pruneInactiveClients pings all clients and unregisters them if they don't respond
// This function runs in the background while the websocket manager lives
func (wm *Manager) pruneInactiveClients() {
	ticker := time.NewTicker(30 * time.Second) // Ping every 30 seconds
	for {
		select {
		case <-ticker.C:
			// Broadcast cleans up unreachable clients
			go wm.BroadcastMessage(websocket.PingMessage, nil)
		}
	}
}

// multiCastMessage sends a message to multiple clients
// used by the BroadcastMessage and SendMessageToGroup functions
func (wm *Manager) multiCastMessage(clients []*Client, messageType int, message []byte) {
	for _, client := range clients {
		clx := client
		if client.IsAlive {
			go func() {
				_ = wm.SendMessageToClient(*clx.ConnId, messageType, message)
			}()
		}
	}
}
