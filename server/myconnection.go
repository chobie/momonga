package server

import (
	"fmt"
	log "github.com/chobie/momonga/logger"
	codec "github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"io"
	"sync"
	"time"
	"net"
	"encoding/hex"
)


type MyConnection struct {
	MyConnection       io.ReadWriteCloser
	Events           map[string]interface{}
	Queue            chan codec.Message
	OfflineQueue     []codec.Message
	MaxOfflineQueue  int
	InflightTable    *util.MessageTable
	SubscribeHistory map[string]int
	PingCounter      int
	Reconnect        bool
	Balancer         *util.Balancer
	Mutex            sync.RWMutex
	Kicker           *time.Timer
	Keepalive        int
	Id               string
	Qlobber          *util.Qlobber
	WillMessage       *codec.WillMessage
	SubscribedTopics  map[string]int
	Opaque interface{}
	Last time.Time
	State State
	CleanSession bool
	Connected bool
	Closed chan bool
}

func (self *MyConnection) SetOpaque(opaque interface{}) {
	self.Opaque = opaque
}

func (self *MyConnection) GetOpaque() interface{} {
	return self.Opaque
}

func NewMyConnection() *MyConnection {
	c := &MyConnection{
		Events:           make(map[string]interface{}),
		Queue:            make(chan codec.Message, 8192),
		OfflineQueue:     make([]codec.Message, 0),
		MaxOfflineQueue:  1000,
		InflightTable:    util.NewMessageTable(),
		SubscribeHistory: make(map[string]int),
		Balancer: &util.Balancer{
			// Writeは10req/secぐらいにおさえようず。クソ実装対策
			PerSec: 10,
		},
		Mutex: sync.RWMutex{},
		Qlobber: util.NewQlobber(),
		SubscribedTopics: make(map[string]int),
		Last: time.Now(),
		CleanSession: true,
		Keepalive: 0,
		State: STATE_INIT,
		Closed: make(chan bool),
	}

	c.Events["connected"] = func() {
		c.State = STATE_CONNECTED
	}

	c.Events["connack"] = func(result uint8) {
		if result == 0 {
			c.State = STATE_CONNECTED
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

				if msg.GetType() == codec.PACKET_TYPE_PUBLISH {
					sb := msg.(*codec.PublishMessage)
					if sb.QosLevel > 0 {
						id := c.InflightTable.NewId()
						sb.PacketIdentifier = id
						c.InflightTable.Register(id, sb, nil)
					}
				}

				c.WriteMessage(msg)
				c.invalidateTimer()
				break
			case <- c.Closed:
				return
			}
		}
	}()

	return c
}

func (self *MyConnection) SetMyConnection(c io.ReadWriteCloser) {
	if self.MyConnection != nil {
		self.Reconnect = true
	}

	self.State = STATE_CONNECTED
	self.MyConnection = c
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

	if self.Keepalive > 0 {
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
	if self.Keepalive > 0 {
		if cn, ok := self.MyConnection.(net.Conn); ok {
			cn.SetReadDeadline(self.Last.Add(time.Duration(int(float64(self.Keepalive) * 1.5)) * time.Second))
		}
	}

	message, err := codec.ParseMessage(self.MyConnection, 8192)
	if err == nil {
		//log.Debug("Read Message: [%s] %+v", message.GetTypeAsString(), message)

		if v, ok := self.Events["parsed"]; ok {
			if cb, ok := v.(func()); ok {
				cb()
			}
		}
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
			log.Error("Unhandled message: %+v\n", message)
		}
	} else {
		log.Debug(">>> Message: %s, %+v\n", err, message)
		if v, ok := self.Events["error"]; ok {
			if cb, ok := v.(func(error)); ok {
				cb(err)
			}
		}
	}

	self.Last = time.Now()
	return message, err
}

func (self *MyConnection) Read(p []byte) (int, error) {
	return self.MyConnection.Read(p)
}

func (self *MyConnection) Write(b []byte) (int, error) {
	return self.MyConnection.Write(b)
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
	log.Debug("Disconnect Operation")
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

func (self *MyConnection) DisableClearSession() {
	self.CleanSession = false
}

func (self *MyConnection) ShouldClearSession() bool {
	return self.CleanSession
}

func (self *MyConnection) GetOutGoingTable() *util.MessageTable {
	return self.InflightTable
}

func (self *MyConnection) SetState(state State) {
	self.State = state
}

func (self *MyConnection) GetState() State {
	return self.State
}

func (self *MyConnection) ResetState() {
	self.State = 0
}

func (self *MyConnection) GetSubscribedTopicQos(topic string) int {
	v := self.Qlobber.Match(topic)
	if len(v) > 0 {
		if r, ok := v[0].(int); ok {
			return r
		}
	}
	return -1
}

func (self *MyConnection) GetSubscribedTopics() map[string]int {
	return self.SubscribedTopics
}

func (self *MyConnection) AppendSubscribedTopic(topic string, qos int) {
	self.SubscribedTopics[topic] = qos
	self.Qlobber.Add(topic, qos)
}

func (self *MyConnection) RemoveSubscribedTopic(topic string) {
	self.Qlobber.Remove(topic, nil)

	if _, ok := self.SubscribedTopics[topic]; ok {
		delete(self.SubscribedTopics, topic)
	}
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

func (self *MyConnection) WriteMessage(msg codec.Message) error {
	log.Debug("Write Message [%s]: %+v", msg.GetTypeAsString(), msg)

	data, err := codec.Encode(msg)
	if err != nil {
		log.Debug("ERROR: %s", err)
		return err
	}

	remaining := len(data)
	offset := 0

	log.Debug("[WRITE: %s] %+v\n%s\n", msg.GetTypeAsString(), msg, hex.Dump(data))

	for offset < remaining {
		size, err := self.Write(data[offset:])
		if err != nil {
			if nerr, ok := err.(net.Error); ok {
				if !nerr.Temporary() {
					log.Debug("NOT TEMP ERROR")
					return nerr
				}
			}
			if err.Error() == "use of closed network connection" {
				return err
			}

			log.Error("WRITE ERROR: %s", err)
		}
		offset += size
	}

	self.Last = time.Now()
	return nil
}
