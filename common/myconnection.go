// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package common

import (
	"bufio"
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"io"
	"net"
	"sync"
	"time"
)

const defaultBufferSize = 16 * 1024

type MyConnection struct {
	MyConnection     io.ReadWriteCloser
	Events           map[string]interface{}
	Queue            chan codec.Message
	OfflineQueue     []codec.Message
	MaxOfflineQueue  int
	InflightTable    *util.MessageTable
	SubscribeHistory map[string]int
	PingCounter      int
	Reconnect        bool
	Mutex            sync.RWMutex
	Kicker           *time.Timer
	Keepalive        int
	Id               string
	Qlobber          *util.Qlobber
	WillMessage      *codec.WillMessage
	SubscribedTopics map[string]int
	Opaque           interface{}
	Last             time.Time
	State            State
	CleanSession     bool
	Connected        bool
	Closed           chan bool
	Reader           *bufio.Reader
	Writer           *bufio.Writer
	KeepLoop         bool
	balancer         *util.Balancer
	logger           log.Logger
	MaxMessageSize   int
}

func (self *MyConnection) SetOpaque(opaque interface{}) {
	self.Opaque = opaque
}

func (self *MyConnection) GetOpaque() interface{} {
	return self.Opaque
}

type MyConfig struct {
	QueueSize        int
	MaxMessageSize   int
	OfflineQueueSize int
	Keepalive        int
	WritePerSec      int
	Logger           log.Logger
}

var defaultConfig = MyConfig{
	QueueSize:        8192,
	MaxMessageSize:   8192,
	OfflineQueueSize: 1024,
	Keepalive:        0,
	WritePerSec:      0,
}

func GetDefaultMyConfig() *MyConfig {
	return &defaultConfig
}

// TODO: どっかで綺麗にしたい
func NewMyConnection(conf *MyConfig) *MyConnection {
	if conf == nil {
		conf = &defaultConfig
	}

	c := &MyConnection{
		Events:           make(map[string]interface{}),
		Queue:            make(chan codec.Message, conf.QueueSize),
		OfflineQueue:     make([]codec.Message, 0),
		MaxOfflineQueue:  conf.OfflineQueueSize,
		InflightTable:    util.NewMessageTable(),
		SubscribeHistory: make(map[string]int),
		Mutex:            sync.RWMutex{},
		Qlobber:          util.NewQlobber(),
		SubscribedTopics: make(map[string]int),
		Last:             time.Now(),
		CleanSession:     true,
		Keepalive:        conf.Keepalive,
		State:            STATE_INIT,
		Closed:           make(chan bool),
		MaxMessageSize:   8192,
	}

	c.logger = log.Global
	if conf.Logger != nil {
		c.logger = conf.Logger
	}

	if conf.WritePerSec > 0 {
		c.balancer = &util.Balancer{
			PerSec: conf.WritePerSec,
		}
	}
	if conf.MaxMessageSize > 0 {
		c.MaxMessageSize = conf.MaxMessageSize
	}

	c.Events["connected"] = func() {
		c.State = STATE_CONNECTED
	}

	c.Events["connack"] = func(result uint8) {
		if result == 0 {
			c.SetState(STATE_CONNECTED)
			if c.Reconnect {
				for key, qos := range c.SubscribeHistory {
					c.Subscribe(key, qos)
				}
			}

			//TODO: このアホっぽい実装はあとでちゃんとなおす。なおしたい
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
			c.State = STATE_CLOSED
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

	// こっちに集約できるとClientが薄くなれる
	c.Events["publish"] = func(msg *codec.PublishMessage) {
		if msg.QosLevel == 1 {
			ack := codec.NewPubackMessage()
			ack.PacketIdentifier = msg.PacketIdentifier
			c.WriteMessageQueue(ack)
			c.logger.Debug("Send puback message to sender. [%s: %d]", c.GetId(), ack.PacketIdentifier)
		} else if msg.QosLevel == 2 {
			ack := codec.NewPubrecMessage()
			ack.PacketIdentifier = msg.PacketIdentifier
			c.WriteMessageQueue(ack)
			c.logger.Debug("Send pubrec message to sender. [%s: %d]", c.GetId(), ack.PacketIdentifier)
		}
	}

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

	c.Events["subscribe"] = func(p *codec.SubscribeMessage) {
	}

	c.Events["suback"] = func(messageId uint16, grunted int) {
		c.InflightTable.Remove(messageId)
	}

	c.Events["unsubscribe"] = func(messageId uint16, granted int, payload []codec.SubscribePayload) {
		for i := 0; i < len(payload); i++ {
			delete(c.SubscribeHistory, payload[i].TopicPath)
		}
	}

	// これはコネクション渡したほうがいいんではないだろうか。
	c.Events["pingreq"] = func() {
		// TODO: check Ping count periodically, abort MyConnection when the counter exceeded.
		c.PingCounter++
	}

	c.Events["pingresp"] = func() {
		// nothing to do.
		c.PingCounter--
	}

	c.Events["disconnect"] = func() {
		// nothing to do ?
		c.State = STATE_CLOSED
	}

	c.Events["error"] = func(err error) {
		//fmt.Printf("Error: %s\n", err)
	}

	c.Events["connect"] = func(msg *codec.ConnectMessage) {
	}

	c.Events["parsed"] = func() {
	}

	// Write Queue
	go func() {
		for {
			select {
			case msg := <-c.Queue:
				if c.GetState() == STATE_CONNECTED || c.GetState() == STATE_CONNECTING {
					if msg.GetType() == codec.PACKET_TYPE_PUBLISH {
						sb := msg.(*codec.PublishMessage)
						if sb.QosLevel < 0 {
							c.logger.Error("QoS under zero. %s: %#v", c.Id, sb)
							break
						}

						if sb.QosLevel > 0 {
							id := c.InflightTable.NewId()
							sb.PacketIdentifier = id
							c.InflightTable.Register(id, sb, nil)
						}
					}

					e := c.writeMessage(msg)
					if e != nil {
						if v, ok := c.Events["error"].(func(error)); ok {
							v(e)
						}
					}
					c.invalidateTimer()
				} else {
					c.OfflineQueue = append(c.OfflineQueue, msg)
				}

			case <-c.Closed:
				if c.KeepLoop {
					time.Sleep(time.Second)
				} else {
					return
				}
			}
		}
	}()

	return c
}

func (self *MyConnection) SetMyConnection(c io.ReadWriteCloser) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()
	if self.MyConnection != nil {
		self.Reconnect = true
	}

	self.MyConnection = c
	self.Writer = bufio.NewWriterSize(self.MyConnection, defaultBufferSize)
	self.Reader = bufio.NewReaderSize(self.MyConnection, defaultBufferSize)
	self.State = STATE_CONNECTED
}

func (self *MyConnection) Subscribe(topic string, QoS int) error {
	sb := codec.NewSubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{
		TopicPath:    topic,
		RequestedQos: uint8(QoS),
	})

	id := self.InflightTable.NewId()
	sb.PacketIdentifier = id
	self.InflightTable.Register(id, sb, nil)
	self.SubscribeHistory[topic] = QoS

	if v, ok := self.Events["subscribe"]; ok {
		if cb, ok := v.(func(*codec.SubscribeMessage, Connection)); ok {
			cb(sb, self)
		}
	}
	self.Queue <- sb
	return nil
}

func (self *MyConnection) setupKicker() {
	if self.Kicker != nil {
		self.Kicker.Stop()
	}

	if self.Keepalive > 0 && self.KeepLoop {
		self.Kicker = time.AfterFunc(time.Second*time.Duration(self.Keepalive), func() {
			self.Ping()
			self.Kicker.Reset(time.Second * time.Duration(self.Keepalive))
		})
	}
}

func (self *MyConnection) Ping() {
	if self.State == STATE_CLOSED {
		return
	}

	self.Queue <- codec.NewPingreqMessage()
}

func (self *MyConnection) On(event string, callback interface{}, args ...bool) error {
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
		}
		break
	case "connect":
		if c, ok := callback.(func(*codec.ConnectMessage)); ok {
			v := self.Events[event].(func(*codec.ConnectMessage))
			if override {
				self.Events[event] = c
			} else {
				self.Events[event] = func(p *codec.ConnectMessage) {
					v(p)
					c(p)
				}
			}
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
		}
		break
	case "subscribe":
		if cv, ok := callback.(func(*codec.SubscribeMessage)); ok {
			v := self.Events[event].(func(*codec.SubscribeMessage))
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func(p *codec.SubscribeMessage) {
					v(p)
					cv(p)
				}
			}
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
		}
		break
	case "unsubscribe":
		if cv, ok := callback.(func(uint16, int, []codec.SubscribePayload)); ok {
			v := self.Events[event].(func(uint16, int, []codec.SubscribePayload))
			if override {
				self.Events[event] = cv
			} else {
				self.Events[event] = func(messageId uint16, grunted int, payload []codec.SubscribePayload) {
					v(messageId, grunted, payload)
					cv(messageId, grunted, payload)
				}
			}
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
		}
		break
	case "pingreq", "pingresp", "disconnect", "parsed":
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
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
		} else {
			panic(fmt.Sprintf("%s callback signature is wrong", event))
		}
		break
	default:
		return fmt.Errorf("Not supported: %s", event)
	}

	return nil
}

func (self *MyConnection) Publish(TopicName string, Payload []byte, QosLevel int, retain bool, opaque interface{}) {
	sb := codec.NewPublishMessage()
	sb.TopicName = TopicName
	sb.Payload = Payload
	sb.QosLevel = QosLevel

	if retain {
		sb.Retain = 1
	}

	self.Queue <- sb
}

func (self *MyConnection) HasMyConnection() bool {
	if self.MyConnection == nil {
		return false
	}
	return true
}

func (self *MyConnection) ReadMessage() (codec.Message, error) {
	return self.ParseMessage()
}

func (self *MyConnection) ParseMessage() (codec.Message, error) {
	//	self.Mutex.RLock()
	//	defer self.Mutex.RUnlock()

	if self.Keepalive > 0 {
		if cn, ok := self.MyConnection.(net.Conn); ok {
			cn.SetReadDeadline(self.Last.Add(time.Duration(int(float64(self.Keepalive)*1.5)) * time.Second))
		}
	}

	if self.Reader == nil {
		panic("reader is null")
	}
	message, err := codec.ParseMessage(self.Reader, self.MaxMessageSize)
	if err != nil {
		self.logger.Debug(">>> Message: %s\n", err)
		if v, ok := self.Events["error"].(func(error)); ok {
			v(err)
		}
		return nil, err
	}

	self.logger.Debug("Read Message: [%s] %+v", message.GetTypeAsString(), message)
	if v, ok := self.Events["parsed"]; ok {
		if cb, ok := v.(func()); ok {
			cb()
		}
	}

	// 以下callbackを呼ぶだけのコード
	switch message.GetType() {
	case codec.PACKET_TYPE_PUBLISH:
		p := message.(*codec.PublishMessage)
		if v, ok := self.Events["publish"]; ok {
			if cb, ok := v.(func(*codec.PublishMessage)); ok {
				cb(p)
			}
		}
		break

	case codec.PACKET_TYPE_CONNACK:
		p := message.(*codec.ConnackMessage)
		if v, ok := self.Events["connack"]; ok {
			if cb, ok := v.(func(uint8)); ok {
				cb(p.ReturnCode)
			}
		}
		break

	case codec.PACKET_TYPE_PUBACK:
		p := message.(*codec.PubackMessage)
		if v, ok := self.Events["puback"]; ok {
			if cb, ok := v.(func(uint16)); ok {
				cb(p.PacketIdentifier)
			}
		}
		break

	case codec.PACKET_TYPE_PUBREC:
		p := message.(*codec.PubrecMessage)
		if v, ok := self.Events["pubrec"]; ok {
			if cb, ok := v.(func(uint16)); ok {
				cb(p.PacketIdentifier)
			}
		}
		break

	case codec.PACKET_TYPE_PUBREL:
		// PUBRELを受けるということはReceiverとして受けるということ
		p := message.(*codec.PubrelMessage)
		if v, ok := self.Events["pubrel"]; ok {
			if cb, ok := v.(func(uint16)); ok {
				cb(p.PacketIdentifier)
			}
		}
		break

	case codec.PACKET_TYPE_PUBCOMP:
		// PUBCOMPを受けるということはSenderとして受けるということ。
		p := message.(*codec.PubcompMessage)
		if v, ok := self.Events["pubcomp"]; ok {
			if cb, ok := v.(func(uint16)); ok {
				cb(p.PacketIdentifier)
			}
		}
		break

	case codec.PACKET_TYPE_PINGREQ:
		if v, ok := self.Events["pingreq"]; ok {
			if cb, ok := v.(func()); ok {
				cb()
			}
		}
		break

	case codec.PACKET_TYPE_PINGRESP:
		if v, ok := self.Events["pingresp"]; ok {
			if cb, ok := v.(func()); ok {
				cb()
			}
		}
		break

	case codec.PACKET_TYPE_SUBACK:
		p := message.(*codec.SubackMessage)
		if v, ok := self.Events["suback"]; ok {
			if cb, ok := v.(func(uint16, int)); ok {
				cb(p.PacketIdentifier, 0)
			}
		}
		break

	case codec.PACKET_TYPE_UNSUBACK:
		p := message.(*codec.UnsubackMessage)
		if v, ok := self.Events["unsuback"]; ok {
			if cb, ok := v.(func(uint16)); ok {
				cb(p.PacketIdentifier)
			}
		}
		break

	case codec.PACKET_TYPE_CONNECT:
		p := message.(*codec.ConnectMessage)
		if v, ok := self.Events["connect"]; ok {
			if cb, ok := v.(func(*codec.ConnectMessage)); ok {
				cb(p)
			}
		}
		break

	case codec.PACKET_TYPE_SUBSCRIBE:
		p := message.(*codec.SubscribeMessage)
		if v, ok := self.Events["subscribe"]; ok {
			if cb, ok := v.(func(*codec.SubscribeMessage)); ok {
				cb(p)
			}
		}
		break
	case codec.PACKET_TYPE_DISCONNECT:
		if v, ok := self.Events["disconnect"]; ok {
			if cb, ok := v.(func()); ok {
				cb()
			}
		}
		break
	case codec.PACKET_TYPE_UNSUBSCRIBE:
		p := message.(*codec.UnsubscribeMessage)
		if v, ok := self.Events["unsubscribe"]; ok {
			if cb, ok := v.(func(uint16, int, []codec.SubscribePayload)); ok {
				cb(p.PacketIdentifier, 0, p.Payload)
			}
		}
		break
	default:
		self.logger.Error("Unhandled message: %+v\n", message)
	}

	self.invalidateTimer()
	self.Last = time.Now()
	return message, err
}

func (self *MyConnection) Read(p []byte) (int, error) {
	return self.Reader.Read(p)
}

func (self *MyConnection) Write(b []byte) (int, error) {
	return self.Writer.Write(b)
}

func (self *MyConnection) Close() error {
	self.State = STATE_CLOSED
	self.Closed <- true

	return self.MyConnection.Close()
}

func (self *MyConnection) WriteMessageQueue(request codec.Message) {
	self.Queue <- request
}

func (self *MyConnection) Disconnect() {
	self.logger.Debug("Disconnect Operation")
	self.Close()
	//self.Queue <- codec.NewDisconnectMessage()
}

func (self *MyConnection) Unsubscribe(topic string) {
	sb := codec.NewUnsubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{TopicPath: topic})
	id := self.InflightTable.NewId()
	sb.PacketIdentifier = id
	self.InflightTable.Register(id, sb, nil)

	self.Queue <- sb
}

func (self *MyConnection) invalidateTimer() {
	if self.Kicker != nil {
		self.Kicker.Reset(time.Second * time.Duration(self.Keepalive))
	}
}

func (self *MyConnection) SetId(id string) {
	self.Id = id
}

func (self *MyConnection) GetRealId() string {
	return self.MyConnection.(net.Conn).RemoteAddr().String()
}

func (self *MyConnection) GetId() string {
	return self.Id
}

func (self *MyConnection) SetKeepaliveInterval(interval int) {
	self.Keepalive = interval
}

func (self *MyConnection) DisableCleanSession() {
	self.CleanSession = false
}

func (self *MyConnection) ShouldCleanSession() bool {
	return self.CleanSession
}

func (self *MyConnection) GetOutGoingTable() *util.MessageTable {
	return self.InflightTable
}

func (self *MyConnection) SetState(state State) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()
	self.State = state
}

func (self *MyConnection) GetState() State {
	self.Mutex.RLock()
	defer self.Mutex.RUnlock()

	return self.State
}

func (self *MyConnection) ResetState() {
	self.State = 0
}

func (self *MyConnection) GetSubscribedTopics() map[string]*SubscribeSet {
	panic("deprecated")
	return nil
}

func (self *MyConnection) AppendSubscribedTopic(topic string, set *SubscribeSet) {
	panic("deprecated")
}

func (self *MyConnection) RemoveSubscribedTopic(topic string) {
	panic("deprecated")
}

func (self *MyConnection) SetWillMessage(will codec.WillMessage) {
	self.WillMessage = &will
}

func (self *MyConnection) GetWillMessage() *codec.WillMessage {
	return self.WillMessage
}

func (self *MyConnection) HasWillMessage() bool {
	if self.WillMessage == nil {
		return false
	}
	return true
}

func (self *MyConnection) IsAlived() bool {
	return true
}

func (self *MyConnection) writeMessage(msg codec.Message) error {
	self.logger.Debug("Write Message [%s]: %+v", msg.GetTypeAsString(), msg)
	var e error
	self.Mutex.Lock()

	self.Last = time.Now()
	if self.balancer != nil && self.balancer.PerSec > 0 {
		self.balancer.Execute(func() {
			_, e = codec.WriteMessageTo(msg, self.Writer)
		})
	} else {
		_, e = codec.WriteMessageTo(msg, self.Writer)
	}

	if e != nil {
		self.Writer.Flush()
		self.Mutex.Unlock()
		return e
	}

	self.Writer.Flush()
	self.Last = time.Now()
	self.Mutex.Unlock()
	return nil
}

func (self *MyConnection) SetRequestPerSecondLimit(limit int) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()
	if self.balancer == nil {
		self.balancer = &util.Balancer{
			PerSec: limit,
		}
	} else {
		self.balancer.PerSec = limit
	}
}
