package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/vizee/echo"
)

const (
	SateNew = iota
	StateAlive
	StateDown
)

var liveConn int32

type Conn struct {
	conn             net.Conn
	ClientIdentifier string
	state            int32
	cheof            chan struct{}
	chr              chan packets.ControlPacket
	chw              chan packets.ControlPacket
	err              error
}

func newConn(conn net.Conn) *Conn {
	atomic.AddInt32(&liveConn, 1)
	return &Conn{
		conn:  conn,
		state: SateNew,
		cheof: make(chan struct{}),
		chr:   make(chan packets.ControlPacket, 256),
		chw:   make(chan packets.ControlPacket, 256),
	}
}

func (c *Conn) shutdown(err error) {
	if atomic.SwapInt32(&c.state, StateDown) >= StateDown {
		return
	}
	c.err = err
	c.conn.Close()
	close(c.cheof)
	atomic.AddInt32(&liveConn, -1)
	if err != nil {
		echo.Warn("conn shutdown", echo.String("client", c.ClientIdentifier), echo.Errval(err))
	}
}

func (c *Conn) IsShutdown() bool {
	return atomic.LoadInt32(&c.state) == StateDown
}

func (c *Conn) readloop() {
	for atomic.LoadInt32(&c.state) < StateDown {
		c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		pack, err := packets.ReadPacket(c.conn)
		if err != nil {
			c.shutdown(err)
			return
		}
		echo.Debug("get", echo.Var("pack", pack))
		select {
		case c.chr <- pack:
		case <-c.cheof:
			break
		}
	}
}

func (c *Conn) writeloop() {
	for atomic.LoadInt32(&c.state) < StateDown {
		select {
		case pack := <-c.chw:
			err := pack.Write(c.conn)
			if err != nil {
				c.shutdown(err)
			}
			echo.Debug("write", echo.Var("pack", pack))
		case <-c.cheof:
			break
		}
	}
}

// Write 直接发送一个包
func (c *Conn) Write(pack packets.ControlPacket) bool {
	if atomic.LoadInt32(&c.state) == StateDown {
		return false
	}
	select {
	case c.chw <- pack:
		return true
	case <-c.cheof:
	default:
		c.shutdown(fmt.Errorf("write chan full"))
		return false
	}
	return false
}

func (c *Conn) connected() {
	go c.readloop()
	go c.writeloop()
	atomic.StoreInt32(&c.state, StateAlive)
}

func (c *Conn) onHand(pack packets.ControlPacket) {
	switch pack.(type) {
	case *packets.PingreqPacket:
		if !c.Write(packets.NewControlPacket(packets.Pingresp)) {
			return
		}
	case *packets.DisconnectPacket:
		c.shutdown(nil)
		return

	case *packets.SubscribePacket:
		data := pack.(*packets.SubscribePacket)
		ack := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
		for i, topic := range data.Topics {
			t := addOrGetTopic(topic)
			t.Subscribe(c)
			qos := data.Qoss[i]
			if qos > 1 { // TODO: 目前只支持 qos <= 1
				qos = 1
			}
			ack.ReturnCodes = append(ack.ReturnCodes, qos)
		}
		ack.MessageID = data.MessageID
		if !c.Write(ack) {
			return
		}
	case *packets.UnsubscribePacket:
		data := pack.(*packets.UnsubscribePacket)
		ack := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
		for _, topic := range data.Topics {
			t := getTopic(topic)
			if t != nil {
				t.UnSubscribe(c)
			}
			// TODO: 目前只支持 qos = 0, qos = 1暂未保存
		}
		ack.MessageID = data.MessageID
		if !c.Write(ack) {
			return
		}

	case *packets.PublishPacket:
		data := pack.(*packets.PublishPacket)
		if data.Qos == 1 {
			ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
			ack.MessageID = data.MessageID
			if !c.Write(ack) {
				return
			}
		}
		t := getTopic(data.TopicName)
		if t == nil {
			echo.Debug("topic not exis", echo.String("topic", data.TopicName))
			return
		}
		t.Broadcast(data)
	}
}

func (c *Conn) serve() {
	c.conn.SetWriteDeadline(time.Now().Add(connectTimeout))
	cp, err := packets.ReadPacket(c.conn)
	if err != nil {
		c.shutdown(err)
		return
	}
	c.conn.SetWriteDeadline(time.Time{})
	p, ok := cp.(*packets.ConnectPacket)
	if !ok {
		c.shutdown(fmt.Errorf("packet not connect"))
		return
	}
	c.ClientIdentifier = p.ClientIdentifier
	// TODO: p.CleanSession
	c.connected()
	if !c.Write(packets.NewControlPacket(packets.Connack)) {
		return
	}
	echo.Debug("ok", echo.Var("v", p))

	for {
		select {
		case pack := <-c.chr:
			c.onHand(pack)
		case <-c.cheof:
			return
		}

	}
}
