package websocketmanager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// UpgradeClient upgrades a client connection to a websocket connection
// It registers the client and returns a channel that will receive the Client or nil on error
func (wm *Manager) UpgradeClient(w http.ResponseWriter, r *http.Request) (*Client, error) {

	// Upgrade connection and pass it to parent thread
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  wm.wsReadBufferSize,
		WriteBufferSize: wm.wsWriteBufferSize,
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Error creating websocket connection")
		return nil, errors.New("error upgrading websocket connection")
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := wm.register(wsConn, cancel)
	wm.initCommunication(client, ctx)
	return client, nil

}

// initCommunication sets up a goroutine to pass on communication safely
// in both directions, and unregister the client when the connection closes.
func (wm *Manager) initCommunication(client *Client, ctx context.Context) {
	// Send out server messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(client.serverSentChan)
				return
			case serverMsg := <-client.serverSentChan:
				err := client.Conn.WriteMessage(serverMsg.MessageType, serverMsg.Message)
				if err != nil {
					client.CancelFunc()
				}
			}
		}
	}()

	// Listen for client messages and handle unregister
	go func() {
		for {
			select {
			case <-ctx.Done():
				wm.Unregister(*client.ConnId)
				return
			default:
				msgType, msg, err := client.Conn.ReadMessage()
				if err != nil {
					client.CancelFunc()
				} else {
					client.mutex.Lock()
					if client.IsAlive {
						for _, observer := range client.observers {
							go observer(client, msgType, msg)
						}
					}
					client.mutex.Unlock()
				}
			}
		}
	}()
}

// AssignGroups assigns a client to one or more groups
func (wm *Manager) AssignGroups(connId uuid.UUID, groupIds ...uint16) error {
	wsClient, err := wm.GetWebSocketClient(connId)
	if err != nil {
		return err
	}
	wsClient.mutex.Lock()
	wsClient.Groups = append(wsClient.Groups, groupIds...)
	wsClient.mutex.Unlock()
	return nil
}

func (wm *Manager) RegisterClientObserver(connId uuid.UUID, messageHandler ClientObservableFunc) {
	wsUser, err := wm.GetWebSocketClient(connId)
	if err != nil {
		log.Debugf("Websocket RegisterClientObserver error: %s", err)
		return
	}
	wsUser.mutex.Lock()
	wsUser.observers = append(wsUser.observers, messageHandler)
	wsUser.mutex.Unlock()
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
	client, err := wm.GetWebSocketClient(clientId)
	if err != nil {
		return err
	}

	client.mutex.Lock()
	if client.IsAlive {
		client.serverSentChan <- &websocketMessage{
			Message:     message,
			MessageType: messageType,
		}
	}
	client.mutex.Unlock()
	return nil
}

// GetWebSocketClient returns a Client by UUID
func (wm *Manager) GetWebSocketClient(clientUUID uuid.UUID) (*Client, error) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	user, ok := wm.clients[clientUUID]
	if !ok {
		return nil, errors.New("client not found or disconnected")
	}
	return user, nil
}

// Unregister unregisters a client from the manager and marks it as not alive
// Further messages to and from this client are discarded
func (wm *Manager) Unregister(connId uuid.UUID) {
	client, _ := wm.GetWebSocketClient(connId)
	if client != nil {
		client.mutex.Lock()
		wm.mutex.Lock()
		client.IsAlive = false
		delete(wm.clients, connId)
		client.CancelFunc()
		wm.mutex.Unlock()
		client.mutex.Unlock()
	}
}

// register registers a new client to the manager
// This function is called by the UpgradeClient function
func (wm *Manager) register(conn *websocket.Conn, cancelFunc context.CancelFunc) *Client {
	connId := uuid.New()
	client := &Client{
		Conn:           conn,
		ConnId:         &connId,
		Groups:         make([]uint16, 0),
		IsAlive:        true,
		CancelFunc:     cancelFunc,
		serverSentChan: make(chan *websocketMessage),
		observers:      make([]ClientObservableFunc, 0),
		mutex:          &sync.Mutex{},
	}
	wm.mutex.Lock()
	wm.clients[connId] = client
	wm.mutex.Unlock()
	return client
}

// multiCastMessage sends a message to multiple clients
// used by the BroadcastMessage and SendMessageToGroup functions
func (wm *Manager) multiCastMessage(clients []*Client, messageType int, message []byte) {
	wm.mutex.Lock()
	for _, client := range clients {
		clx := client
		if client != nil {
			go func() {
				_ = wm.SendMessageToClient(*clx.ConnId, messageType, message)
			}()
		}
	}
	wm.mutex.Unlock()
}
