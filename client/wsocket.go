package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	websocket "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
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
	conn  net.Conn
	mutex sync.RWMutex
}

// Send implments Socket.Send
func (ws *WSocket) Send(msg *Message) error {
	if conn := ws.getConn(); conn != nil {
		// ws.mutex.Lock()
		// defer ws.mutex.Unlock()

		// marshal msg
		// result, err := json.Marshal(msg)
		// if err != nil {
		// 	return err
		// }
		bin, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			return err
		}
		err = wsutil.WriteClientMessage(conn, websocket.OpText, bin)
		if err != nil {
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
		data, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(data, &msg)
		if err != nil {
			return nil, err
		}
		return msg, nil
	}
	return msg, fmt.Errorf("current connection is nil")
}

func (ws *WSocket) getConn() net.Conn {
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
