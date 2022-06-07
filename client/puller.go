package client

import (
	"sync"
)

// Puller channel to receive result
type Puller struct {
	center   *regCenter
	ch       chan *Message
	chClosed bool
	key      string
	closed   bool
	mutex    sync.Mutex
}

// Close return puller to regCenter
func (puller *Puller) Close() {
	if puller.center == nil || puller.isClosed() {
		return
	}

	puller.setClosed(true)
	puller.mutex.Lock()
	puller.chClosed = true
	puller.mutex.Unlock()
	puller.center.unregister(puller)
}

// Pull pull messge from chan
func (puller *Puller) Pull() <-chan *Message {
	return puller.ch
}

func (puller *Puller) isClosed() bool {
	puller.mutex.Lock()
	defer puller.mutex.Unlock()
	return puller.closed
}

func (puller *Puller) setClosed(state bool) {
	puller.mutex.Lock()
	puller.closed = state
	puller.mutex.Unlock()
}

func (puller *Puller) closeChann() {
	puller.mutex.Lock()
	defer puller.mutex.Unlock()

	if puller.chClosed {
		return
	}
	puller.chClosed = true
	close(puller.ch)

}

func (puller *Puller) isChannClosed() bool {
	puller.mutex.Lock()
	defer puller.mutex.Unlock()
	return puller.chClosed
}
