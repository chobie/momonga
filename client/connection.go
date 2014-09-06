package client

import (
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"io"
	"sync"
	"time"
)

type ConnectionState int

const (
	CONNECTION_STATE_CLOSED ConnectionState = iota
	CONNECTION_STATE_CONNECTING
	CONNECTION_STATE_CONNECTED
)

type Connection struct {
	Connection io.ReadWriteCloser
	Events map[string]interface{}
	Queue        chan codec.Message
	ProcessQueue chan codec.Message
	OfflineQueue    []codec.Message
	MaxOfflineQueue int
	InflightTable    *util.MessageTable
	SubscribeHistory map[string]int
	ConnectionState  ConnectionState
	PingCounter      int
	Reconnect        bool
	Balancer        *util.Balancer
	Mutex           sync.RWMutex
	Kicker          *time.Timer
	Keepalive       int
}

func NewConnection() *Connection {
	c := &Connection{
		Events:           make(map[string]interface{}),
		Queue:            make(chan codec.Message, 8192),
		ProcessQueue:     make(chan codec.Message, 8192),
		OfflineQueue:     make([]codec.Message, 0),
		MaxOfflineQueue:  1000,
		InflightTable:    util.NewMessageTable(),
		SubscribeHistory: make(map[string]int),
		Balancer: &util.Balancer{
			// Writeは10req/secぐらいにおさえようず。クソ実装対策
			PerSec: 10,
		},
		Mutex: sync.RWMutex{},
	}

	c.Events["connected"] = func() {
		c.ConnectionState = CONNECTION_STATE_CONNECTED
	}

	c.Events["connack"] = func(result uint8) {
		if result == 0 {
			c.ConnectionState = CONNECTION_STATE_CONNECTED
			if c.Reconnect {
				for key, qos := range c.SubscribeHistory {
					c.Subscribe(key, qos)
				}
			}

			//TODO: このアホっぽい実装はあとでちゃんとなおす
			if len(c.OfflineQueue) > 0 {
				c.Mutex.Lock()
				var targets []codec.Message
				for len(c.OfflineQueue) > 0 {
					targets = append(targets, c.OfflineQueue[0])
					c.OfflineQueue = c.OfflineQueue[1:]
				}
				c.Mutex.Unlock()

				for i := 0; i < len(targets); i++ {
					c.Queue <- targets[i]
				}
			}
			c.setupKicker()
		} else {
			c.ConnectionState = CONNECTION_STATE_CLOSED
		}
	}

	// for Wait API
	c.InflightTable.SetOnFinish(func(id uint16, message codec.Message, opaque interface{}) {
		if m, ok := message.(*codec.PublishMessage); ok {
			if m.QosLevel == 1 {
				if b, ok := opaque.(chan bool); ok {
					close(b)
				}
			} else if m.QosLevel == 2 {
				if b, ok := opaque.(chan bool); ok {
					close(b)
				}
			}
		}
	})

	c.Events["publish"] = func(message *codec.PublishMessage) {
		switch message.QosLevel {
		case 1:
			ack := codec.NewPubackMessage()
			ack.PacketIdentifier = message.PacketIdentifier
			c.Queue <- ack
		case 2:
			ack := codec.NewPubrecMessage()
			ack.PacketIdentifier = message.PacketIdentifier
			c.Queue <- ack
		}
	}

	// こっちに集約できるとClientが薄くなれる
	c.Events["puback"] = func(messageId uint16) {
		c.InflightTable.Unref(messageId)
	}

	c.Events["pubrec"] = func(messageId uint16) {
		ack := codec.NewPubrelMessage()
		ack.PacketIdentifier = messageId
		c.Queue <- ack
	}

	c.Events["pubrel"] = func(messageId uint16) {
		ack := codec.NewPubcompMessage()
		ack.PacketIdentifier = messageId
		c.Queue <- ack

		c.InflightTable.Unref(ack.PacketIdentifier) // Unackknowleged
	}

	c.Events["pubcomp"] = func(messageId uint16) {
		c.InflightTable.Unref(messageId)
	}

	c.Events["unsuback"] = func(messageId uint16) {
		mm, err := c.InflightTable.Get(messageId)
		if err == nil {
			if v, ok := mm.(*codec.UnsubscribeMessage); ok {
				delete(c.SubscribeHistory, v.TopicName)
			}
		}

		c.InflightTable.Remove(messageId)
	}

	c.Events["subscribe"] = func(uint16, int, []codec.SubscribePayload) {
	}

	c.Events["suback"] = func(messageId uint16, grunted int) {
		c.InflightTable.Remove(messageId)
	}

	c.Events["unsubscribe"] = func(messageId uint16, granted int, payload []*codec.SubscribePayload) {
		for i := 0; i < len(payload); i++ {
			delete(c.SubscribeHistory, payload[i].TopicPath)
		}
	}

	c.Events["pingreq"] = func() {
		// TODO: check Ping count periodically, abort connection when the counter exceeded.
		c.PingCounter++
	}

	c.Events["pingresp"] = func() {
		// nothing to do.
		c.PingCounter--
	}

	c.Events["disconnect"] = func() {
		// nothing to do ?
		c.ConnectionState = CONNECTION_STATE_CLOSED
	}

	c.Events["error"] = func(err error) {
		fmt.Printf("Error: %s\n", err)
	}

	// つながった瞬間からConnectedでよい
	// Write Queue
	go func() {
		for {
			select {
			case msg := <-c.Queue:

				if msg.GetType() == codec.PACKET_TYPE_PUBLISH {
					sb := msg.(*codec.PublishMessage)
					if sb.QosLevel > 0 {
						id := c.InflightTable.NewId()
						sb.PacketIdentifier = id
						c.InflightTable.Register(id, sb, nil)
					}
				}

				b2, _ := codec.Encode(msg)
				if c.GetConnectionState() == CONNECTION_STATE_CONNECTED {
					if c.Balancer.PerSec < 1 {
						_, err := c.Connection.Write(b2)
						if err != nil {
							if v, ok := c.Events["error"].(func(error)); ok {
								v(err)
							}
						}
					} else {
						c.Balancer.Execute(func() {
							_, err := c.Connection.Write(b2)
							if err != nil {
								if v, ok := c.Events["error"].(func(error)); ok {
									v(err)
								}
							}
						})
					}
					c.invalidateTimer()
				} else {
					c.OfflineQueue = append(c.OfflineQueue, msg)
					if len(c.OfflineQueue) > c.MaxOfflineQueue {
						c.OfflineQueue = c.OfflineQueue[len(c.OfflineQueue)-c.MaxOfflineQueue:]
					}
				}
				break
			}
		}
	}()

	// Process Queue (dispatch only)
	go func() {
		for {
			select {
			case message := <-c.ProcessQueue:
				switch message.GetType() {
				case codec.PACKET_TYPE_PUBLISH:
					p := message.(*codec.PublishMessage)
					if v, ok := c.Events["publish"]; ok {
						if cb, ok := v.(func(*codec.PublishMessage)); ok {
							cb(p)
						}
					}
					break

				case codec.PACKET_TYPE_CONNACK:
					p := message.(*codec.ConnackMessage)
					if v, ok := c.Events["connack"]; ok {
						if cb, ok := v.(func(uint8)); ok {
							cb(p.ReturnCode)
						}
					}
					break

				case codec.PACKET_TYPE_PUBACK:
					p := message.(*codec.PubackMessage)
					if v, ok := c.Events["puback"]; ok {
						if cb, ok := v.(func(uint16)); ok {
							cb(p.PacketIdentifier)
						}
					}
					break

				case codec.PACKET_TYPE_PUBREC:
					p := message.(*codec.PubrecMessage)
					if v, ok := c.Events["pubrec"]; ok {
						if cb, ok := v.(func(uint16)); ok {
							cb(p.PacketIdentifier)
						}
					}
					break

				case codec.PACKET_TYPE_PUBREL:
					// PUBRELを受けるということはReceiverとして受けるということ
					p := message.(*codec.PubrelMessage)
					if v, ok := c.Events["pubrel"]; ok {
						if cb, ok := v.(func(uint16)); ok {
							cb(p.PacketIdentifier)
						}
					}
					break

				case codec.PACKET_TYPE_PUBCOMP:
					// PUBCOMPを受けるということはSenderとして受けるということ。
					p := message.(*codec.PubcompMessage)
					if v, ok := c.Events["pubcomp"]; ok {
						if cb, ok := v.(func(uint16)); ok {
							cb(p.PacketIdentifier)
						}
					}
					break

				case codec.PACKET_TYPE_PINGRESP:
					if v, ok := c.Events["pingresp"]; ok {
						if cb, ok := v.(func()); ok {
							cb()
						}
					}
					break

				case codec.PACKET_TYPE_SUBACK:
					p := message.(*codec.SubackMessage)
					if v, ok := c.Events["suback"]; ok {
						if cb, ok := v.(func(uint16, int)); ok {
							cb(p.PacketIdentifier, 0)
						}
					}
					break

				case codec.PACKET_TYPE_UNSUBACK:
					p := message.(*codec.UnsubackMessage)
					if v, ok := c.Events["unsuback"]; ok {
						if cb, ok := v.(func(uint16)); ok {
							cb(p.PacketIdentifier)
						}
					}
					break
				default:
					fmt.Printf("Unhandled message: %+v\n", message)
				}
			}
		}
	}()

	return c
}

func (self *Connection) SetConnection(c io.ReadWriteCloser) {
	if self.Connection != nil {
		self.Reconnect = true
	}

	self.ConnectionState = CONNECTION_STATE_CONNECTED
	self.Connection = c
}

func (self *Connection) Subscribe(topic string, QoS int) error {
	sb := codec.NewSubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{
		TopicPath:  topic,
		RequestedQos: uint8(QoS),
	})

	id := self.InflightTable.NewId()
	sb.PacketIdentifier = id
	self.InflightTable.Register(id, sb, nil)
	self.SubscribeHistory[topic] = QoS

	if v, ok := self.Events["subscribe"]; ok {
		if cb, ok := v.(func(uint16, int, []codec.SubscribePayload)); ok {
			cb(sb.PacketIdentifier, 0, sb.Payload)
		}
	}
	self.Queue <- sb
	return nil
}

func (self *Connection) setupKicker() {
	if self.Kicker != nil {
		self.Kicker.Stop()
	}

	if self.Keepalive > 0 {
		self.Kicker = time.AfterFunc(time.Second * time.Duration(self.Keepalive), func() {
				self.Ping()
				self.Kicker.Reset(time.Second * time.Duration(self.Keepalive))
			})
	}
}

func (self *Connection) Ping() {
	if self.ConnectionState == CONNECTION_STATE_CLOSED {
		return
	}

	self.Queue <- codec.NewPingreqMessage()
}

func (self *Connection) On(event string, callback interface{}, args ...bool) error {
	override := false
	if len(args) > 0 {
		override = args[0]
	}

	switch event {
	case "connected":
		if c, ok := callback.(func()); ok {
			v := self.Events[event].(func())

			if override {
				self.Events[event] = c
			} else {
				self.Events[event] = func() {
					v()
					c()
				}
			}
		}
		break
	case "connack":
		if c, ok := callback.(func(uint8)); ok {
			v := self.Events[event].(func(uint8))

			if override {
				self.Events[event] = c
			} else {
				self.Events[event] = func(result uint8) {
					v(result)
					c(result)
				}
			}
		}
		break
	case "publish":
		if c, ok := callback.(func(*codec.PublishMessage)); ok {
			v := self.Events[event].(func(*codec.PublishMessage))

			if override {
				self.Events[event] = c
			} else {
				self.Events[event] = func(message *codec.PublishMessage) {
					v(message)
					c(message)
				}
			}
		}
		break
	case "puback", "pubrec", "pubrel", "pubcomp", "unsuback":
		if c, ok := callback.(func(uint16)); ok {
			v := self.Events[event].(func(uint16))
			if override {
				self.Events[event] = c
			} else {
				self.Events[event] = func(messageId uint16) {
					v(messageId)
					c(messageId)
				}
			}
		}
		break
	case "subscribe":
		if cv, ok := callback.(func(uint16, int, []codec.SubscribePayload)); ok {
			v := self.Events[event].(func(uint16, int, []codec.SubscribePayload))
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func(messageId uint16, qos int, payload []codec.SubscribePayload) {
					v(messageId, qos, payload)
					cv(messageId, qos, payload)
				}
			}
		}
	case "suback":
		if cv, ok := callback.(func(uint16, int)); ok {
			v := self.Events[event].(func(uint16, int))
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func(messageId uint16, grunted int) {
					v(messageId, grunted)
					cv(messageId, grunted)
				}
			}
		}
		break
	case "unsubscribe":
		if cv, ok := callback.(func(uint16, int, []*codec.SubscribePayload)); ok {
			v := self.Events[event].(func(uint16, int, []*codec.SubscribePayload))
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func(messageId uint16, grunted int, payload []*codec.SubscribePayload) {
					v(messageId, grunted, payload)
					cv(messageId, grunted, payload)
				}
			}
		}
		break
	case "pingreq", "pingresp", "disconnect":
		if cv, ok := callback.(func()); ok {
			v := self.Events[event].(func())
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func() {
					v()
					cv()
				}
			}
		}
		break
	case "error":
		if cv, ok := callback.(func(error)); ok {
			v := self.Events[event].(func(error))
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func(err error) {
					v(err)
					cv(err)
				}
			}
		}
		break
	default:
		return fmt.Errorf("Not supported: %s", event)
	}

	return nil
}

func (self *Connection) GetConnectionState() ConnectionState {
	return self.ConnectionState
}

func (self *Connection) Publish(TopicName string, Payload []byte, QosLevel int, retain bool, opaque interface{}) {
	sb := codec.NewPublishMessage()
	sb.TopicName = TopicName
	sb.Payload = Payload
	sb.QosLevel = QosLevel

	if retain {
		sb.Retain = 1
	}

	self.Queue <- sb
}

func (self *Connection) HasConnection() bool {
	if self.Connection == nil {
		return false
	}
	return true
}

func (self *Connection) ParseMessage() (codec.Message, error) {
	message, err := codec.ParseMessage(self.Connection)

	if err == nil {
		self.ProcessQueue <- message
	} else {
		if v, ok := self.Events["error"]; ok {
			if cb, ok := v.(func(error)); ok {
				cb(err)
			}
		}
	}

	return message, err
}

func (self *Connection) Read(p []byte) (int, error) {
	return self.Connection.Read(p)
}

func (self *Connection) Write(b []byte) (int, error) {
	return self.Connection.Write(b)
}

func (self *Connection) Close() error {
	self.ConnectionState = CONNECTION_STATE_CLOSED
	return self.Connection.Close()
}

func (self *Connection) WriteMessageQueue(request codec.Message) {
	self.Queue <- request
}

func (self *Connection) Disconnect() {
	self.Queue <- codec.NewDisconnectMessage()
}

func (self *Connection) Unsubscribe(topic string) {
	sb := codec.NewUnsubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{TopicPath: topic})
	id := self.InflightTable.NewId()
	sb.PacketIdentifier = id
	self.InflightTable.Register(id, sb, nil)

	self.Queue <- sb
}

func (self *Connection) invalidateTimer() {
	if self.Kicker != nil {
		self.Kicker.Reset(time.Second * time.Duration(self.Keepalive))
	}
}
