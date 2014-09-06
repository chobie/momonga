package server

import (
	"time"
	"errors"
	"fmt"
	"io"
	"strings"
	"regexp"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
)

type DisconnectError struct {
}
func (e *DisconnectError) Error() string { return "received disconnect message" }

type Retryable struct {
	Id string
	Payload interface{}
}

type Pidgey struct {
	Topics map[string]*Topic
	Queue chan codec.Message
	OutGoingTable *util.MessageTable
	Qlobber *util.Qlobber
	// TODO: improve this.
	Retain map[string]*codec.PublishMessage
	SessionStore *SessionStore
	Connections map[string]*MmuxConnection
	SubscribeMap map[string]string
	RetryMap map[string][]*Retryable
	ErrorChannel chan *Retryable
}

func (self *Pidgey) HasTopic(Topic string) bool {
	if _, ok := self.Topics[Topic]; ok {
		return true
	} else {
		return false
	}
}

func (self *Pidgey) GetTopic(name string) (*Topic, error) {
	if self.HasTopic(name) {
		return self.Topics[name], nil
	}
	return nil, errors.New(fmt.Sprintf("topic %s does not exiist", name))
}

func (self *Pidgey) CreateTopic(name string) (*Topic, error) {
	// TODO: Atomicにしておく
	self.Topics[name] = &Topic{
		Name: name,
		Level: 0,
		QoS: 0,
		CreatedAt: time.Now(),
	}

	return self.Topics[name], nil
}

func (self *Pidgey) SetupCallback() {
	self.OutGoingTable.SetOnFinish(func(id uint16, message codec.Message, opaque interface{}) {
		// ここでQoS2の処理する？
		log.Debug("Message: id: %d, %+v", id, message)

		switch (message.GetType()) {
		case codec.PACKET_TYPE_PUBLISH:
			p := message.(*codec.PublishMessage)
			if p.QosLevel == 2 {
				ack := codec.NewPubcompMessage()
				ack.PacketIdentifier = p.PacketIdentifier
				// TODO: あれ、なんだっけこれ？
//				if conn != nil {
//					conn.WriteMessageQueue(ack)
//				}
			}
			break
		default:
			log.Debug("1Not supported; %d", message.GetType())
		}
	})
}


func (self *Pidgey) handshake(conn Connection) (*MmuxConnection, error) {
	msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	if msg.GetType() != codec.PACKET_TYPE_CONNECT {
		return nil, errors.New("Invalid message")
	}
	p := msg.(*codec.ConnectMessage)

	if !(string(p.Magic) == "MQTT" || string(p.Magic) == "MQIsdp") {
		return nil, errors.New("Invalid protocol")
	}
	log.Debug("CONNECT [%s]: %+v", conn.GetId(), conn)
	// TODO: version check
	for _, v := range self.Connections {
		log.Debug("  Connection: %s", v.GetId())
	}


	// TODO: 認証部分をつける

	// preserve messagen when will flag set
	if (p.Flag & 0x4) > 0 {
		log.Debug("This message has Will flag: %+v\n", p.Will)
		conn.SetWillMessage(*p.Will)
	}

	if !p.CleanSession {
		conn.DisableClearSession()
	}

	var mux *MmuxConnection
	var ok bool

	if mux, ok = self.Connections[p.Identifier]; ok {
		mux.Attach(conn)
	} else {
		mux = &MmuxConnection{
			Identifier: p.Identifier,
			Connections: map[string]Connection{},
		}
		mux.Attach(conn)
	}

	conn.SetKeepaliveInterval(int(p.KeepAlive))
	mux.SetKeepaliveInterval(int(p.KeepAlive))
	reply := codec.NewConnackMessage()
	// うーん。どうしよっかな。
	conn.WriteMessage(reply)

	mux.SetState(STATE_ACCEPTED)
	self.Connections[mux.GetId()] = mux

	if p.CleanSession {
		// Retry MapもClear
		delete(self.RetryMap, mux.GetId())
		self.CleanHoge(mux)
	}

	return mux, nil
}

func (self *Pidgey) Handshake(conn Connection) (*MmuxConnection, error) {
	mux, err := self.handshake(conn)

	if err != nil {
		if err != io.EOF {
			msg := codec.NewDisconnectMessage()
			conn.WriteMessage(msg)
		}
		return nil, err;
	}
	return mux, nil
}

func (self *Pidgey) Run() {
	for {
		select {
			// QoSごとにチャンネル持ってると便利？
			// これはようはWriteQueueってやつだ
		case msg := <- self.Queue:
			switch (msg.GetType()) {
			case codec.PACKET_TYPE_PUBLISH:
				m := msg.(*codec.PublishMessage)
				log.Debug("sending PUBLISH [id:%s, lvl:%d]", m.PacketIdentifier, m.QosLevel)

				//topic, err := self.GetTopic(m.TopicName)
//				if err != nil{
//					fmt.Printf("err: %s\n", err)
//					//continue
//				}

				// TODO: Retainはサーバーが再起動しても保持しなければならない
				if m.Retain > 0 {
					if len(m.Payload) == 0 {
						delete(self.Retain, m.TopicName)
					} else {
						self.Retain[m.TopicName] = m
					}
				}

				// Publishで受け取ったMessageIdのやつのCountをとっておく
				// で、Pubackが帰ってきたらrefcountを下げて0になったらMessageを消す
				targets := self.Qlobber.Match(m.TopicName)
				log.Debug("TopicName: %s", m.TopicName)
				if m.QosLevel > 0 {
					// TODO: ClientごとにInflightTableを持つ
					id := self.OutGoingTable.NewId()
					m.PacketIdentifier = id

					if sender, ok := m.Opaque.(Connection); ok {
						self.OutGoingTable.Register2(m.PacketIdentifier, m, len(targets), sender)
					}
				}

				for i := range targets {
					log.Debug("sending publish message to %s [%s %s %d %d]", targets[i].(Connection).GetId(), m.TopicName, m.Payload, m.PacketIdentifier, m.QosLevel)
					targets[i].(Connection).WriteMessageQueue(m)
				}
				break
			default:
				log.Debug("ARE?: %+v", msg)
			}
		case r := <- self.ErrorChannel:
			self.RetryMap[r.Id] = append(self.RetryMap[r.Id], r)
			log.Debug("ADD RETRYABLE MAP")
			break
		}
	}
}

func (self *Pidgey) CleanHoge(conn Connection) {
	for _, t := range conn.GetSubscribedTopics() {
		if conn.ShouldClearSession() {
			self.Qlobber.Remove(t, conn)
		}
	}
	if conn.ShouldClearSession() {
		delete(self.SubscribeMap, conn.GetId())
	}
}

func (self *Pidgey) SendWillMessage(conn Connection) {
	will := conn.GetWillMessage()
	msg := codec.NewPublishMessage()
	msg.TopicName = will.Topic
	msg.Payload = []byte(will.Message)
	msg.QosLevel = int(will.Qos)
	log.Debug("%s => %s", msg.TopicName, msg.Payload)
	self.Queue <- msg
}

func (self *Pidgey) HandleRequest(conn Connection) error {
	if conn.GetState() == STATE_DETACHED {
		return nil
	}

	err := self.handle(conn)
	if err != nil {
		self.CleanHoge(conn)
		if _, ok := err.(*DisconnectError); !ok {
			if conn.HasWillMessage() {
				log.Debug("Okay, Send Will Message")
				self.SendWillMessage(conn)
			}
		}
	}
	return err
}

func (self *Pidgey) handle(conn Connection) error {
	msg, err := conn.ReadMessage()
	if err != nil {
		log.Debug("Error: %s", err)
		return err
	}

	// TODO: Roleとかのflagつけて同一のにしちゃってもいいきもしたけどこみいってくると面倒という
	switch (msg.GetType()) {

	case codec.PACKET_TYPE_PUBLISH:
		p := msg.(*codec.PublishMessage)
		log.Debug("Received Publish Message[%s] : %+v", conn.GetId(), p)

		if !self.HasTopic(p.TopicName) {
			self.CreateTopic(p.TopicName)
		}

		if p.QosLevel == 1 {
			ack := codec.NewPubackMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			log.Debug("Send puback message to sender. [%s: %d]", conn.GetId(), ack.PacketIdentifier)
		} else if p.QosLevel == 2 {
			ack := codec.NewPubrecMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			log.Debug("Send pubrec message to sender. [%s: %d]", conn.GetId(), ack.PacketIdentifier)
		}

		// TODO: QoSによっては適切なMessageIDを追加する
		// Server / ClientはそれぞれMessageTableが違うの？
		if p.QosLevel > 0 {
//			id := conn.GetOutGoingTable().NewId()
//			p.PacketIdentifier = id
			conn.GetOutGoingTable().Register(p.PacketIdentifier, p, conn)
			p.Opaque = conn
		}

		self.Queue <- p
		break

	case codec.PACKET_TYPE_PUBACK:
		p := msg.(*codec.PubackMessage)
		log.Debug("Received Puback Message from [%s: %d]", conn.GetId(), p.PacketIdentifier)

		// TODO: これのIDは内部的なの？
		self.OutGoingTable.Unref(p.PacketIdentifier)

		// TODO: Refcounting
		// PUBACKが帰ってくるのはServer->ClientでQoS1で送った時だけ。
		// PUBACKが全員分かえってきたらメッセージを消してもいい
		break

	case codec.PACKET_TYPE_PUBREC:
		p := msg.(*codec.PubrecMessage)
		log.Debug("Received Pubrec Message from [%s: %d]", conn.GetId(), p.PacketIdentifier)
		ack := codec.NewPubrelMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		conn.WriteMessageQueue(ack)
		break

	case codec.PACKET_TYPE_PUBREL:
		p := msg.(*codec.PubrelMessage)
		log.Debug("Received Pubrel Message: [%s: %d]", conn.GetId(), p.PacketIdentifier)
		ack := codec.NewPubcompMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		conn.WriteMessageQueue(ack)
		log.Debug("Send pubcomp message to sender. [%s: %d]", conn.GetId(), ack.PacketIdentifier)
		break

	case codec.PACKET_TYPE_PUBCOMP:
		log.Debug("Received Pubcomp Message from %s: %+v", conn.GetId(), msg)
		p := msg.(*codec.PubcompMessage)
		self.OutGoingTable.Unref(p.PacketIdentifier)
		break

	case codec.PACKET_TYPE_SUBSCRIBE:
		p := msg.(*codec.SubscribeMessage)
		log.Debug("Subscribe Message: [%s] %+v\n", conn.GetId(), p)

		ack := codec.NewSubackMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		var retained []*codec.PublishMessage
		for _, payload := range p.Payload {
			var topic *Topic

			// don't subscribe multiple time
			if _, exists := self.SubscribeMap[conn.GetId()]; exists {
				log.Debug("Map exists. [%s:%s]", conn.GetId(), payload.TopicPath)
				continue
			}

			self.SubscribeMap[conn.GetId()] = payload.TopicPath

			self.Qlobber.Add(payload.TopicPath, conn)
			conn.AppendSubscribedTopic(payload.TopicPath)

			// TODO: これはAtomicにさせたいなー、とおもったり。
			// というかTopicは実装上もうつかってないので消していいや
			if !self.HasTopic(payload.TopicPath) {
				topic, _ = self.CreateTopic(payload.TopicPath)
			} else {
				topic, _ = self.GetTopic(payload.TopicPath)
			}

			retaines := self.RetainMatch(payload.TopicPath)
			if len(retaines) > 0 {
				log.Debug("Publish Retained message: %+v", topic.Retain)

				for i := range retaines {
					pp := codec.NewPublishMessage()
					id := conn.GetOutGoingTable().NewId()

					pp.PacketIdentifier = id
					pp.Payload = retaines[i].Payload
					pp.TopicName = retaines[i].TopicName
					pp.QosLevel = retaines[i].QosLevel
					pp.Retain = retaines[i].Retain

					conn.GetOutGoingTable().Register(id, pp, conn)
					retained = append(retained, pp)
				}
			}
		}
		log.Debug("Send Suback Message To: %s", conn.GetId())
		conn.WriteMessageQueue(ack)

		for i := range retained {
			conn.WriteMessage(retained[i])
		}
		break

	case codec.PACKET_TYPE_UNSUBSCRIBE:
		p := msg.(*codec.UnsubscribeMessage)
		log.Debug("Received unsubscribe from [%s]: %+v\n", conn.GetId(), p)
		for _, payload := range p.Payload {
			log.Debug("sent Unsuback message")

			ack := codec.NewUnsubackMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			self.Qlobber.Remove(payload.TopicPath, conn)
		}
		break

	case codec.PACKET_TYPE_PINGREQ:
		//p := msg.(*codec.PingreqMessage)
		r := codec.NewPingrespMessage()
		conn.WriteMessageQueue(r)
		break

	case codec.PACKET_TYPE_DISCONNECT:
		log.Debug("Received disconnect from %s", conn.GetId())
		return &DisconnectError{}
	default:
		fmt.Printf("2Not supported: %d\n", msg.GetType())
		return errors.New("Invalid protocol sequence")
	}

	return nil
}

// これTrieにしたいんだけどめんどい
func (self *Pidgey) RetainMatch(topic string) []*codec.PublishMessage {
	var result []*codec.PublishMessage

	topic = strings.Replace(topic, "+", "[^/]+", -1)
	topic = strings.Replace(topic, "#", ".*", -1)

	reg, err := regexp.Compile(topic)
	if err != nil {
		fmt.Printf("Regexp Error: %s", err)
	}

	for k, v := range self.Retain {
		if reg.MatchString(k) {
			result = append(result, v)
		}
	}

	return result
}
