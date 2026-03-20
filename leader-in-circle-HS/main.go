package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
)

type MsgType int

const (
	OUT MsgType = iota
	IN
)

type Message struct {
	UID       int
	Distance  int
	Hop       int
	Direction int // -1 left, +1 right
	Type      MsgType
}

type Process struct {
	uid       int
	active    bool
	leftIn    chan Message
	rightIn   chan Message
	leftOut   chan Message
	rightOut  chan Message

	inCount   map[int]int // phase -> number of IN messages received
}

var messageCount int64 = 0
var mu sync.Mutex

func send(ch chan Message, msg Message) {
	mu.Lock()
	messageCount++
	mu.Unlock()
	ch <- msg
}

func (p *Process) run(wg *sync.WaitGroup, leader chan int) {
	defer wg.Done()

	for {
		select {
		case msg := <-p.leftIn:
			p.handle(msg, p.rightOut, leader)
		case msg := <-p.rightIn:
			p.handle(msg, p.leftOut, leader)
		}
	}
}

func (p *Process) handle(msg Message, out chan Message, leader chan int) {
	switch msg.Type {

	case OUT:
		if msg.UID == p.uid {
			leader <- p.uid
			return
		}

		if msg.UID > p.uid {
			if msg.Hop < msg.Distance {
				msg.Hop++
				send(out, msg)
			} else {
				// reached max distance → send IN back
				send(out, Message{
					UID:       msg.UID,
					Distance:  msg.Distance,
					Hop:       0,
					Direction: -msg.Direction,
					Type:      IN,
				})
			}
		} else {
			p.active = false
		}

	case IN:
		if msg.UID == p.uid {
			phase := int(math.Log2(float64(msg.Distance)))
			p.inCount[phase]++

			if p.inCount[phase] == 2 {
				// survived this phase
				// do nothing, next phase will start externally
			}
		} else {
			send(out, msg)
		}
	}
}

func main() {
	n := 10
	processes := make([]*Process, n)

	uids := rand.Perm(1000)[:n]

	channels := make([]chan Message, n)
	for i := 0; i < n; i++ {
		channels[i] = make(chan Message, 100)
	}

	for i := 0; i < n; i++ {
		processes[i] = &Process{
			uid:      uids[i],
			active:   true,
			leftIn:   channels[(i-1+n)%n],
			rightIn:  channels[(i+1)%n],
			leftOut:  channels[i],
			rightOut: channels[i],
			inCount:  make(map[int]int),
		}
	}

	var wg sync.WaitGroup
	leaderChan := make(chan int, 1)

	for _, p := range processes {
		wg.Add(1)
		go p.run(&wg, leaderChan)
	}

	maxPhases := int(math.Ceil(math.Log2(float64(n))))
	rounds := 0

	for phase := 0; phase < maxPhases; phase++ {
		distance := int(math.Pow(2, float64(phase)))
		rounds++

		for _, p := range processes {
			if p.active {
				send(p.leftOut, Message{
					UID:       p.uid,
					Distance:  distance,
					Hop:       1,
					Direction: -1,
					Type:      OUT,
				})
				send(p.rightOut, Message{
					UID:       p.uid,
					Distance:  distance,
					Hop:       1,
					Direction: 1,
					Type:      OUT,
				})
			}
		}

		select {
		case leader := <-leaderChan:
			fmt.Println("Leader UID:", leader)
			fmt.Println("Rounds:", rounds)
			fmt.Println("Messages:", messageCount)
			return
		default:
			// continue
		}
	}
}