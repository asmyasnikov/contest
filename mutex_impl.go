package contest

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const chmLocked int32 = -1

type chMutex struct {
	state  int32
	mu     sync.Mutex
	ch     chan struct{}
	closed chan struct{}
}

func New() Mutex {
	c := make(chan struct{})
	close(c)
	return &chMutex{
		closed: c,
	}
}

func (m *chMutex) Lock() {
	if atomic.CompareAndSwapInt32(&m.state, 0, chmLocked) {
		return
	}

	m.lockSlow()
}

func (m *chMutex) LockChannel() <-chan struct{} {
	if atomic.CompareAndSwapInt32(&m.state, 0, chmLocked) {
		return m.closed
	}

	return make(chan struct{})
}

func (m *chMutex) Unlock() {
	if atomic.CompareAndSwapInt32(&m.state, chmLocked, 0) {
		m.closeCh()
		time.Sleep(time.Duration(rand.Intn(3000)) * time.Nanosecond)
		return
	}

	panic("chMutex: Unlock fail")
}

func (m *chMutex) lockSlow() {
	ch := m.getCh()
	for {
		if atomic.CompareAndSwapInt32(&m.state, 0, chmLocked) {
			return
		}
		<-ch
		ch = m.getCh()
	}
}

func (m *chMutex) getCh() <-chan struct{} {
	m.mu.Lock()
	if m.ch == nil {
		m.ch = make(chan struct{}, 1)
	}
	ret := m.ch
	m.mu.Unlock()
	return ret
}

func (m *chMutex) closeCh() {
	var cp chan struct{}
	m.mu.Lock()
	if m.ch != nil {
		cp = m.ch
		m.ch = nil
	}
	m.mu.Unlock()
	if cp != nil {
		close(cp)
	}
}
