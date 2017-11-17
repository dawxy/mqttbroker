package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/vizee/echo"
)

const (
	topicIdle = iota
	topicLive
	topicDown
)

var topics struct {
	mu  sync.RWMutex
	tps map[string]*Topic

	idMu      sync.Mutex
	messageID uint16
}

func clearTopic() {
	for range time.Tick(clearTopicInterval) {
		var (
			tps  []*Topic
			down []string
		)
		topics.mu.Lock()
		for name, topic := range topics.tps {
			if topic == nil || topic.IsShutdown() {
				down = append(down, name)
			} else {
				tps = append(tps, topic)
			}
		}
		for _, name := range down {
			delete(topics.tps, name)
		}
		topics.mu.Unlock()

		echo.Info("alive", echo.Int("topic", len(tps)), echo.Int32("con", atomic.LoadInt32(&liveConn)))
		for _, topic := range tps {
			topic.clearClients()
		}
	}
}

func getMessageID() (id uint16) {
	topics.idMu.Lock()
	topics.messageID++
	id = topics.messageID
	topics.idMu.Unlock()
	return
}

func getTopic(name string) (t *Topic) {
	topics.mu.RLock()
	t = topics.tps[name]
	if t != nil {
		topics.mu.RUnlock()
		return
	}
	topics.mu.RUnlock()
	return
}

func addOrGetTopic(name string) (t *Topic) {
	t = getTopic(name)
	if t != nil {
		return
	}
	tmp := newTopic(name)
	topics.mu.Lock()
	t = topics.tps[name]
	if t != nil {
		topics.mu.Unlock()
		return
	}
	t = tmp
	topics.tps[name] = t
	topics.mu.Unlock()
	return
}

type Topic struct {
	name string

	mu      sync.RWMutex
	state   int32
	clients map[string]*Conn
}

func newTopic(name string) *Topic {
	return &Topic{
		state:   topicIdle,
		name:    name,
		clients: make(map[string]*Conn),
	}
}

func (t *Topic) clearClients() {
	var (
		down []string
	)
	t.mu.Lock()
	for name, c := range t.clients {
		if c == nil || c.IsShutdown() {
			down = append(down, name)
		}
	}
	for _, name := range down {
		delete(t.clients, name)
	}
	sum := len(t.clients)
	t.mu.Unlock()
	if sum == 0 {
		t.shutdown()
	}
	echo.Info("topic", echo.String("name", t.name), echo.Int("clients", sum))
}

func (t *Topic) shutdown() {
	t.mu.Lock()
	if t.state == topicDown {
		t.mu.Unlock()
		return
	}
	t.state = topicDown
	var clients []*Conn
	for _, c := range t.clients {
		clients = append(clients, c)
	}
	t.mu.Unlock()

	for _, c := range clients {
		c.shutdown(fmt.Errorf("clear err"))
	}
	topics.mu.Lock()
	delete(topics.tps, t.name)
	topics.mu.Unlock()
	echo.Info("topic shutdown", echo.String("name", t.name))
}

func (t *Topic) IsShutdown() bool {
	t.mu.RLock()
	down := t.state == topicDown
	t.mu.RUnlock()
	return down
}

// TODO: 会有大量订阅时需要,限制最大订阅数,且 LRU 等优化
func (t *Topic) Subscribe(c *Conn) {
	t.mu.Lock()
	if t.state == topicDown {
		t.mu.Unlock()
		return
	}
	t.clients[c.ClientIdentifier] = c
	if t.state == topicIdle {
		t.state = topicLive
	}
	t.mu.Unlock()
}

func (t *Topic) UnSubscribe(c *Conn) {
	t.mu.Lock()
	if t.state == topicDown {
		t.mu.Unlock()
		return
	}
	delete(t.clients, c.ClientIdentifier)
	if len(t.clients) == 0 {
		t.state = topicIdle
	}
	t.mu.Unlock()
}

func (t *Topic) Broadcast(pack *packets.PublishPacket) {
	var (
		clients []*Conn
	)
	t.mu.RLock()
	if t.state == topicIdle || t.state == topicDown {
		t.mu.RUnlock()
		return
	}
	for _, c := range t.clients {
		if c == nil || c.IsShutdown() {
			continue
		}
		clients = append(clients, c)
	}
	t.mu.RUnlock()
	pack.MessageID = getMessageID()
	for _, c := range clients {
		c.Write(pack)
	}
}

func init() {
	topics.tps = make(map[string]*Topic)
	go clearTopic()
}
