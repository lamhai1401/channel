package client

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lamhai1401/gologs/logs"
	"golang.org/x/net/context"
)

const maxMimumReadBuffer = 1024 * 1024 * 2
const maxMimumWriteBuffer = 1024 * 1024 * 2
const VSN = "1.0.0"

const (
	ConnConnecting = "connecting"
	ConnOpen       = "open"
	ConnClosing    = "closing"
	Connclosed     = "closed"
)

type Connection struct {
	ctx    context.Context
	cancel func()

	sock     Socket
	ref      refMaker
	center   *regCenter
	msgs     chan *Message
	isClosed bool // check close
	mutex    sync.RWMutex
	status   string
}

func Connect(_url string, args url.Values) (*Connection, error) {
	surl, err := url.Parse(_url)

	if err != nil {
		return nil, err
	}

	if !surl.IsAbs() {
		return nil, errors.New("url should be absolute")
	}

	oscheme := surl.Scheme
	switch oscheme {
	case "ws":
		break
	case "wss":
		break
	case "http":
		surl.Scheme = "ws"
	case "https":
		surl.Scheme = "wss"
	default:
		return nil, errors.New("schema should be http or https")
	}

	surl.Path = path.Join(surl.Path, "websocket")
	surl.RawQuery = args.Encode()

	// originURL := fmt.Sprintf("%s://%s", surl.Scheme, surl.Host)
	originURL := fmt.Sprintf("%s://%s%s?%s", surl.Scheme, surl.Host, surl.Path, surl.RawQuery)
	// socketURL := surl.String()
	// fmt.Println(socketURL==originURL)
	// wconn, err := websocket.Dial(socketURL, "", originURL)
	// if err != nil {
	// 	return nil, err
	// }

	var wsConn *websocket.Conn

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(getTimeout())*time.Second)
		defer cancel()

		dialer := websocket.Dialer{
			Proxy:             http.ProxyFromEnvironment,
			HandshakeTimeout:  30 * time.Second,
			EnableCompression: true,
			ReadBufferSize:    maxMimumReadBuffer,
			WriteBufferSize:   maxMimumWriteBuffer,
		}

		wsConn, _, err = dialer.DialContext(ctx, originURL, nil)
		if err != nil {
			if isTimeoutError(err) {
				logs.Warn("*** Connection timeout. Try to reconnect")
				wsConn = nil
				err = nil
				ctx = nil
				continue
			} else {
				return nil, err
			}
		} else {
			break
		}
	}

	err = wsConn.SetCompressionLevel(6)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn := &Connection{
		ctx:    ctx,
		cancel: cancel,
		sock:   &WSocket{conn: wsConn},
		center: newRegCenter(),
		msgs:   make(chan *Message, 1000),
		status: ConnOpen,
	}

	conn.start()

	return conn, nil
}

func isTimeoutError(err error) bool {
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

const all = ""

// OnMessage receive all message on connection
func (conn *Connection) OnMessage() *Puller {
	return conn.center.register(all)
}

func (conn *Connection) push(msg *Message) error {
	return conn.sock.Send(msg)
}

func (conn *Connection) heartbeatLoop() {
	msg := &Message{
		Topic:   "phoenix",
		Event:   "heartbeat",
		Payload: "",
	}
	for {
		select {
		case <-time.After(30000 * time.Millisecond):
			// Set the message reference right before we send it:
			msg.Ref = conn.ref.makeRef()
			conn.sock.Send(msg)
		case <-conn.ctx.Done():
			return
		}
	}
}

func (conn *Connection) pullLoop() {
	for {
		msg, err := conn.sock.Recv()
		if err != nil {
			conn.msgs <- nil
			fmt.Printf("%s\n", err)
			close(conn.msgs)
			conn.closeAllPullers()
			return
		}
		select {
		case <-conn.ctx.Done():
			return
		case conn.msgs <- msg:
		}
	}
}

func (conn *Connection) coreLoop() {
	for {
		select {
		case <-conn.ctx.Done():
			return
		case msg, ok := <-conn.msgs:
			if !ok || msg == nil {
				return
			}
			conn.dispatch(msg)
		}
	}
}

func (conn *Connection) start() {
	go conn.pullLoop()
	go conn.coreLoop()
	go conn.heartbeatLoop()
}

func (conn *Connection) Close() error {
	conn.setClose(true)
	conn.cancel()
	return conn.sock.Close()
}

func (conn *Connection) pushToChan(puller *Puller, msg *Message) {
	select {
	case puller.ch <- msg:
	case <-time.After(10 * time.Millisecond):
	}
}

func (conn *Connection) pushToChans(wg *sync.WaitGroup, pullers []*Puller, msg *Message) {
	for _, puller := range pullers {
		go conn.pushToChan(puller, msg)
	}
	wg.Done()
}

func (conn *Connection) closePullers(pullers []*Puller) {
	for _, puller := range pullers {
		close(puller.ch)
	}
}

func (conn *Connection) closeAllPullers() {

	conn.center.RLock()
	defer conn.center.RUnlock()

	for _, pullers := range conn.center.regs {
		for _, puller := range pullers {
			close(puller.ch)
		}
	}

}

func (conn *Connection) dispatch(msg *Message) {
	var wg sync.WaitGroup
	wg.Add(4)
	go conn.pushToChans(&wg, conn.center.getPullers(all), msg)
	go conn.pushToChans(&wg, conn.center.getPullers(toKey(msg.Topic, "", "")), msg)
	go conn.pushToChans(&wg, conn.center.getPullers(toKey(msg.Topic, msg.Event, "")), msg)
	go conn.pushToChans(&wg, conn.center.getPullers(toKey("", "", msg.Ref)), msg)
	wg.Wait()
}

func (conn *Connection) checkClose() bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.isClosed
}

func (conn *Connection) setClose(state bool) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.isClosed = state
}
