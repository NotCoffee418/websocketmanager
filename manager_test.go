package websocketmanager

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestManagerBuilder tests the builder pattern for constructing Manager
func TestManagerBuilder(t *testing.T) {
	mgr := NewBuilder().WithReadBufferSize(2048).WithWriteBufferSize(2048).Build()
	assert.Equal(t, 2048, mgr.wsReadBufferSize)
	assert.Equal(t, 2048, mgr.wsWriteBufferSize)
}

// TestClientRegistration tests the client registration process
func TestClientRegistration(t *testing.T) {
	// Create a new instance of Manager
	manager := NewDefaultManager()

	var client *Client // to store the client returned by the channel

	// Create a test HTTP server and request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientCh := manager.UpgradeClientCh(w, r)
		client = <-clientCh
		if client == nil {
			t.Fatalf("Client was nil!")
		}
	}))
	defer server.Close()

	// Create a WebSocket dialer to connect to the server
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		t.Fatalf("Failed to open WebSocket connection: %v", err)
	}
	defer conn.Close()

	assert.NotNil(t, client)
	assert.NotNil(t, client.Conn)
	assert.NotNil(t, client.ConnId)
	assert.True(t, client.IsAlive)
}

func TestClientRegistrationWithHelper(t *testing.T) {
	mgr := NewDefaultManager()
	client, _, _ := helperGetWsClientComms(t, mgr)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Conn)
	assert.NotNil(t, client.ConnId)
	assert.True(t, client.IsAlive)
}

// TestAssignGroups tests assigning groups to a client
func TestAssignGroups(t *testing.T) {
	mgr := NewDefaultManager()
	clientId := uuid.New()
	client := &Client{
		ConnId: &clientId,
		Groups: []uint16{},
	}

	mgr.clients[clientId] = client
	err := mgr.AssignGroups(clientId, 1, 2, 3)

	assert.NoError(t, err)
	assert.ElementsMatch(t, []uint16{1, 2, 3}, client.Groups)
}

// TestUnregister tests unregistering a client
func TestUnregister(t *testing.T) {
	mgr := NewDefaultManager()
	clientId := uuid.New()
	client := &Client{
		ConnId: &clientId,
	}

	mgr.clients[clientId] = client
	mgr.Unregister(clientId)

	_, exists := mgr.clients[clientId]
	assert.False(t, exists)
}

func TestSendMessageToUser(t *testing.T) {
	mgr := NewDefaultManager()

	client1, cChan1, _ := helperGetWsClientComms(t, mgr)
	client2, cChan2, _ := helperGetWsClientComms(t, mgr)

	assert.NotNil(t, client1)
	assert.NotNil(t, client2)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		message := []byte("test message")
		assert.NoError(t, mgr.SendMessageToClient(*client1.ConnId, websocket.TextMessage, message))
	}()
	wg.Wait()

	receivedMessage1, ok1 := helperGetOneFromChannelOrTimeout(t, cChan1)
	_, ok2 := helperGetOneFromChannelOrTimeout(t, cChan2) // expect timeout

	assert.True(t, ok1)
	assert.False(t, ok2, "Client 2 should not receive the message")

	assert.Equal(t, "test message", string(receivedMessage1))
}

// TestBroadcastMessage tests if the broadcast method is working
func TestBroadcastMessage(t *testing.T) {
	mgr := NewDefaultManager()

	client1, cChan1, _ := helperGetWsClientComms(t, mgr)
	client2, cChan2, _ := helperGetWsClientComms(t, mgr)
	client3, cChan3, _ := helperGetWsClientComms(t, mgr)
	mgr.Unregister(*client3.ConnId) // spoof dead

	assert.NotNil(t, client1)
	assert.NotNil(t, client2)
	assert.NotNil(t, client3)

	// Broadcast message
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		message := []byte("test message")
		mgr.BroadcastMessage(1, message)
	}()
	wg.Wait()

	receivedMessage1, ok1 := helperGetOneFromChannelOrTimeout(t, cChan1)
	receivedMessage2, ok2 := helperGetOneFromChannelOrTimeout(t, cChan2)
	_, ok3 := helperGetOneFromChannelOrTimeout(t, cChan3) // expect timeout

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.False(t, ok3)

	assert.Equal(t, "test message", string(receivedMessage1))
	assert.Equal(t, "test message", string(receivedMessage2))
	assert.False(t, ok3, "Client 3 is dead, should not receive the message")
}

// TestSpamUsers Checks for random panics on nil clients
func TestSpamUsers(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			TestSendMessageToUser(t)
		}()
	}
	wg.Wait()
}

// TestSendMessageToGroup tests if we can send a message to a specific group
func TestSendMessageToGroup(t *testing.T) {
	mgr := NewDefaultManager()

	initGroupedClient := func(groups ...uint16) (*Client, chan []byte) {
		client, cChan, _ := helperGetWsClientComms(t, mgr)
		assert.NoError(t, mgr.AssignGroups(*client.ConnId, groups...))
		return client, cChan
	}

	client1, cChan1 := initGroupedClient(1)
	client2, cChan2 := initGroupedClient(1)
	client3, cChan3 := initGroupedClient(2) // Separate group

	assert.NotNil(t, client1)
	assert.NotNil(t, client2)
	assert.NotNil(t, client3)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		message := []byte("test message")
		mgr.SendMessageToGroup(1, websocket.TextMessage, message)
	}()
	wg.Wait()

	receivedMessage1, ok1 := helperGetOneFromChannelOrTimeout(t, cChan1)
	receivedMessage2, ok2 := helperGetOneFromChannelOrTimeout(t, cChan2)
	_, ok3 := helperGetOneFromChannelOrTimeout(t, cChan3) // expect timeout

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.False(t, ok3)

	assert.Equal(t, "test message", string(receivedMessage1))
	assert.Equal(t, "test message", string(receivedMessage2))
	assert.False(t, ok3, "Client 3 should not receive the message")
}

func TestRegisterClientObserver(t *testing.T) {
	mgr := NewDefaultManager()

	var client *Client

	// Expects a message from client within a second
	msgChan := make(chan []byte, 1)
	clientObservableFunc := func(wsClient *Client, messageType int, message []byte) {
		msgChan <- message
	}

	// Set up spoof server
	wg := sync.WaitGroup{}
	wg.Add(1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientCh := mgr.UpgradeClientCh(w, r)
		client = <-clientCh
		assert.NotNil(t, client)
		mgr.RegisterClientObserver(*client.ConnId, clientObservableFunc)
		wg.Done()
	}))

	// Set up spoof client
	dialer := websocket.Dialer{}
	wsClient, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		t.Fatalf("Failed to open WebSocket connection: %v", err)
	}
	wg.Wait() // Wait for client to be registered after being accessed by spoof client

	// Send message spoof browser to server
	err = wsClient.WriteMessage(websocket.TextMessage, []byte("pong"))
	if err != nil {
		fmt.Printf("WriteMessage Error: %v\n", err)
	}

	// Wait for message from client
	receivedMsg, ok := helperGetOneFromChannelOrTimeout(t, msgChan)

	// Check if we got the message and it's valid
	assert.True(t, ok)
	assert.Equal(t, "pong", string(receivedMsg))
}

func helperGetWsClientComms(t *testing.T, mgr *Manager) (
	_client *Client, clientReceived chan []byte, serverReceived chan []byte) {
	var client *Client

	wg := sync.WaitGroup{}
	wg.Add(1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientCh := mgr.UpgradeClientCh(w, r)
		client = <-clientCh
		assert.NotNil(t, client)
		wg.Done()
	}))

	dialer := websocket.Dialer{}
	wsClient, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		t.Fatalf("Failed to open WebSocket connection: %v", err)
	}

	// Wait for client to be registered after being accessed by spoof client
	// this is hacky for testing purposes because of the way httptest.NewServer is used
	wg.Wait()
	assert.NotNil(t, client)

	// Create a channel to indicate the browser received a message
	clientReceivedMessagesChan := make(chan []byte, 1)
	go func() {
		for {
			_, message, err := wsClient.ReadMessage()
			if err != nil {
				fmt.Printf("ReadMessage Error: %v\n", err)
				close(clientReceivedMessagesChan)
				break
			}
			//fmt.Printf("Received Message: %s\n", string(message))
			clientReceivedMessagesChan <- message
		}
	}()

	// Create a channel to indicate the server received a message
	serverReceivedMessagesChan := make(chan []byte, 1)
	mgr.RegisterClientObserver(*client.ConnId, func(wsClient *Client, messageType int, message []byte) {
		serverReceivedMessagesChan <- message
	})

	return client, clientReceivedMessagesChan, serverReceivedMessagesChan
}

func helperGetOneFromChannelOrTimeout(t *testing.T, ch <-chan []byte) ([]byte, bool) {
	select {
	case res := <-ch:
		return res, true
	case <-time.After(3 * time.Second):
		return nil, false
	}
}
