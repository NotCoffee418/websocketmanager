# WebSocket Manager

WebSocket Manager is a Go library that simplifies managing WebSocket connections. It offers functionalities like upgrading HTTP connections to WebSocket, managing clients, broadcasting messages, and more.

This library is an abstraction on top of the [gorilla/websocket](https://github.com/gorilla/websocket) library.

## Installation

Install the package using `go get`:

```bash
go get github.com/NotCoffee418/websocketmanager
```

Add the package to your code:

```go
import "github.com/NotCoffee418/websocketmanager"
```

## Usage

Here's how to use WebSocket Manager in your Go application.

### Initialize Manager

Create a new instance of the Manager type using one of the following methods:

```go
// Create a manager with default settings
wsManager := websocketmanager.NewDefaultManager()

// Create a manager with customized settings
wsManager := websocketmanager.NewBuilder().
              WithReadBufferSize(2048).
              WithWriteBufferSize(2048).
              WithClientCleanupDisabled().
              Build()
```

### Upgrade HTTP Connection to WebSocket

Upgrade an incoming HTTP request to a WebSocket connection:

```go
clientCh := wsManager.UpgradeClientCh(w, r)
client := <-clientCh
```

### Assign Groups

To categorize clients into groups, use the `AssignGroups` method:

```go
wsManager.AssignGroups(*client.ConnId, 1, 2)
```

### Register Observers

To observe messages from a specific client, register an observer function:

```go
wsManager.RegisterClientObserver(*client.ConnId, func(wsClient *websocketmanager.Client, messageType int, message []byte) {
    // Handle incoming message
})
```

### Send Messages

To send messages, you have the following options:

- Broadcast a message to all clients
  ```go
  wsManager.BroadcastMessage(messageType, message)
  ```

- Send a message to a specific group of clients
  ```go
  wsManager.SendMessageToGroup(groupID, messageType, message)
  ```

- Send a message to a specific client
  ```go
  wsManager.SendMessageToUser(clientUUID, messageType, message)
  ```

### Manage Clients

You can manually unregister clients or retrieve specific clients by their UUID:

- To unregister a client:
  ```go
  wsManager.Unregister(clientUUID)
  ```

- To get a specific client:
  ```go
  client, err := wsManager.GetWebSocketClient(clientUUID)
  ```

### Cleanup

By default, WebSocket Manager will automatically clean up inactive clients. You can disable this during the initialization step if needed.

## Contributing

Contributions are welcome. Feel free to open a pull request or issue on GitHub.

## License

This project is licensed under the MIT License.

For more information, please refer to the [LICENSE](LICENSE) file in the repository.
