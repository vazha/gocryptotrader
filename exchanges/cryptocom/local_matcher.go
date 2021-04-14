package cryptocom

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Match
type Match struct {
	m map[interface{}]chan UserOrderResponse
	sync.Mutex
}

// NewMatch returns a new matcher
func NewMatch() *Match {
	return &Match{
		m: make(map[interface{}]chan UserOrderResponse),
	}
}

// Incoming matches with request, disregarding the returned payload
func (m *Match) Incoming(signature interface{}) bool {
	return m.IncomingWithData(signature, UserOrderResponse{})
}

// IncomingWithData matches with requests and takes in the returned payload, to
// be processed outside of a stream processing routine
func (m *Match) IncomingWithData(signature interface{}, data UserOrderResponse) bool {
	s := fmt.Sprint(signature)
	//fmt.Printf("MATCHERs FOUND: %+v\n", m.m)
	timer1 := time.NewTimer(time.Second * 6)
	for { // wait for REST finish
		m.Lock()
		ch, ok := m.m[s]
		m.Unlock()
		if ok {
			timer := time.NewTimer(time.Second * 3)
			select {
			case ch <- data:
			case <-timer.C:
				fmt.Println("IncomingWithData STOP timer for", s)
				timer.Stop()
				return false
			}
			return true
		}

		select {
		case <-timer1.C:
			fmt.Println("IncomingWithData STOP timer1 for", s)
			timer1.Stop()
			return false
		default:
			time.Sleep(time.Millisecond * 100)
			//fmt.Println("matcher, waiting rest for ", s)
		}
	}
}

// Sets the signature response channel for incoming data
func (m *Match) set(signature interface{}) (matcher, error) {
	var ch chan UserOrderResponse
	m.Lock()

	s := fmt.Sprint(signature)
	_, ok := m.m[s]
	if ok {
		m.Unlock()
		return matcher{}, errors.New("signature collision")
	}
	// This is buffered so we don't need to wait for receiver.
	ch = make(chan UserOrderResponse, 1)
	m.m[s] = ch
	m.Unlock()

	return matcher{
		C:   ch,
		sig: s,
		m:   m,
	}, nil
}

// matcher defines a payload matching return mechanism
type matcher struct {
	C   chan UserOrderResponse
	sig interface{}
	m   *Match
}

// Cleanup closes underlying channel and deletes signature from map
func (m *matcher) Cleanup() {
	m.m.Lock()
	close(m.C)
	delete(m.m.m, m.sig)
	m.m.Unlock()
}
