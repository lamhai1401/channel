package client

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Wss constant
const (
	// Time allowed to write a message to the peer.
	writeWait = 30 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
)

// Socket interface for easy test
type Socket interface {
	Send(*Message) error
	Recv() (*Message, error)
	Close() error
}

// WSocket Socket implementation for web socket
type WSocket struct {
	conn  *websocket.Conn
	mutex sync.RWMutex
}

// Send implments Socket.Send
func (ws *WSocket) Send(msg *Message) error {
	// return websocket.JSON.Send(ws.conn, msg)
	if conn := ws.getConn(); conn != nil {
		if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			return err
		}
		ws.mutex.Lock()
		defer ws.mutex.Unlock()

		// marshal msg
		result, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if err := conn.WriteMessage(websocket.TextMessage, result); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("current connection is nil")
}

// Close implements Socket.Close
func (ws *WSocket) Close() error {
	return ws.conn.Close()
}

// Recv implements Socket.Recv
func (ws *WSocket) Recv() (*Message, error) {
	var msg *Message
	if conn := ws.getConn(); conn != nil {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				return nil, fmt.Errorf("websocket closeGoingAway error: %v", err)
			}
			return nil, fmt.Errorf("recv err: %v", err)
		}

		if len(resp) == 0 {
			return msg, nil
		}

		err = json.Unmarshal(resp, &msg)
		if err != nil {
			return nil, fmt.Errorf("signaler Unmarshal err: %v", err)
		}
		return msg, nil
	}
	return msg, fmt.Errorf("current connection is nil")
	// return msg, websocket.JSON.Receive(ws.conn, msg)
}

func (ws *WSocket) getConn() *websocket.Conn {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return ws.conn
}

func getTimeout() int {
	i := 18
	if interval := os.Getenv("WSS_TIME_OUT"); interval != "" {
		j, err := strconv.Atoi(interval)
		if err == nil {
			i = j
		}
	}
	return i
}
