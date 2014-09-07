package server

import (
	"errors"
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"io"
	"regexp"
	"strings"
	"time"
)

type DisconnectError struct {
}

func (e *DisconnectError) Error() string { return "received disconnect message" }

// TODO: haven't used this yet.
type Retryable struct {
	Id      string
	Payload interface{}
}

/*
goroutine (2)
	RunMaintenanceThread
	Run
 */
type Momonga struct {
	Topics        map[string]*Topic
	Queue         chan codec.Message
	OutGoingTable *util.MessageTable
	Qlobber       *util.Qlobber
	// TODO: improve this.
	Retain       map[string]*codec.PublishMessage
	Connections  map[string]*MmuxConnection
	SubscribeMap map[string]string
	RetryMap     map[string][]*Retryable
	ErrorChannel chan *Retryable
	System       System
	EnableSys    bool
}

func (self *Momonga) DisableSys() {
	self.EnableSys = false
}

func (self *Momonga) HasTopic(Topic string) bool {
	if _, ok := self.Topics[Topic]; ok {
		return true
	} else {
		return false
	}
}

func (self *Momonga) Terminate() {
}

func (self *Momonga) GetTopic(name string) (*Topic, error) {
	if self.HasTopic(name) {
		return self.Topics[name], nil
	}
	return nil, errors.New(fmt.Sprintf("topic %s does not exiist", name))
}

func (self *Momonga) CreateTopic(name string) (*Topic, error) {
	// TODO: This should be operate atomically
	self.Topics[name] = &Topic{
		Name:      name,
		Level:     0,
		QoS:       0,
		CreatedAt: time.Now(),
	}

	return self.Topics[name], nil
}

func (self *Momonga) SetupCallback() {
	self.OutGoingTable.SetOnFinish(func(id uint16, message codec.Message, opaque interface{}) {
		switch message.GetType() {
		case codec.PACKET_TYPE_PUBLISH:
			p := message.(*codec.PublishMessage)
			if p.QosLevel == 2 {
				ack := codec.NewPubcompMessage()
				ack.PacketIdentifier = p.PacketIdentifier
				// TODO: WHAAAT? I don't remember this
				//				if conn != nil {
				//					conn.WriteMessageQueue(ack)
				//				}
			}
			break
		default:
			log.Debug("1Not supported; %d", message.GetType())
		}
	})

	// For now
	if self.EnableSys {
		msg := codec.NewPublishMessage()
		msg.TopicName = "$SYS/broker/broker/version"
		msg.Payload = []byte("0.1.0")
		msg.Retain = 1
		self.Queue <- msg
	}
}

func (self *Momonga) handshake(conn Connection) (*MmuxConnection, error) {
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

	//log.Debug("CONNECT [%s]: %+v", conn.GetId(), conn)
	// TODO: version check
	//	for _, v := range self.Connections {
	//		log.Debug("  Connection: %s", v.GetId())
	//	}

	// TODO: implement authenticator

	// preserve messagen when will flag set
	if (p.Flag & 0x4) > 0 {
		conn.SetWillMessage(*p.Will)
	}

	if !p.CleanSession {
		conn.DisableClearSession()
	}

	var mux *MmuxConnection
	var ok bool
	session_exist := false

	if mux, ok = self.Connections[p.Identifier]; ok {
		mux.Attach(conn)
		session_exist = true
	} else {
		mux = NewMmuxConnection()
		mux.Identifier = p.Identifier
		mux.Attach(conn)
	}

	conn.SetKeepaliveInterval(int(p.KeepAlive))
	mux.SetKeepaliveInterval(int(p.KeepAlive))
	reply := codec.NewConnackMessage()

	if session_exist {
		// [MQTT-3.2.2-2] If the Server accepts a connection with CleanSession set to 0,
		// the value set in Session Present depends on whether the Server already has stored Session state
		// for the supplied client ID. If the Server has stored Session state,
		// it MUST set Session Present to 1 in the CONNACK packet.
		reply.Reserved |= 0x01
	}

	conn.WriteMessage(reply)
	mux.SetState(STATE_ACCEPTED)
	self.Connections[mux.GetId()] = mux

	if p.CleanSession {
		delete(self.RetryMap, mux.GetId())
		self.CleanSubscription(mux)

		// Should I remove remaining QoS1, QoS2 message at this time?
		mux.GetOutGoingTable().Clean()
	} else {
		// Okay, attach to existing session.
		tbl := mux.GetOutGoingTable()

		for _, c := range tbl.Hash {
			msgs := make([]codec.Message, 0)
			//check qos1, 2 message and resend to this client.
			if v, ok := c.Message.(*codec.PublishMessage); ok {
				if v.QosLevel > 0 {
					//mux.WriteMessageQueue(c.Message)
					msgs = append(msgs, c.Message)
				}
			}
			tbl.Clean()

			// TODO: improve this
			for _, v := range msgs {
				mux.WriteMessageQueue(v)
			}
		}
	}

	return mux, nil
}

func (self *Momonga) Handshake(conn Connection) (*MmuxConnection, error) {
	mux, err := self.handshake(conn)

	if err != nil {
		if err != io.EOF {
			msg := codec.NewDisconnectMessage()
			conn.WriteMessage(msg)
		}
		return nil, err
	}
	return mux, nil
}

func (self *Momonga) CleanSubscription(conn Connection) {
	for t, _ := range conn.GetSubscribedTopics() {
		if conn.ShouldClearSession() {
			self.Qlobber.Remove(t, conn)
		}
	}
	if conn.ShouldClearSession() {
		delete(self.SubscribeMap, conn.GetId())
	}
}

func (self *Momonga) SendWillMessage(conn Connection) {
	will := conn.GetWillMessage()
	msg := codec.NewPublishMessage()
	msg.TopicName = will.Topic
	msg.Payload = []byte(will.Message)
	msg.QosLevel = int(will.Qos)
	self.Queue <- msg
}

func (self *Momonga) HandleRequest(conn Connection) error {
	if conn.GetState() == STATE_DETACHED {
		return nil
	}

	err := self.handle(conn)
	if err != nil {
		self.CleanSubscription(conn)
		if _, ok := err.(*DisconnectError); !ok {
			if conn.HasWillMessage() {
				self.SendWillMessage(conn)
			}
		}
	}
	return err
}

func (self *Momonga) handle(conn Connection) error {
	msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	// TODO: it would be better if we can share this with client library. but it's bother me
	switch msg.GetType() {
	case codec.PACKET_TYPE_PUBLISH:
		p := msg.(*codec.PublishMessage)
		//log.Debug("Received Publish Message[%s] : %+v", conn.GetId(), p)

		if !self.HasTopic(p.TopicName) {
			self.CreateTopic(p.TopicName)
		}

		if p.QosLevel == 1 {
			ack := codec.NewPubackMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			//log.Debug("Send puback message to sender. [%s: %d]", conn.GetId(), ack.PacketIdentifier)
		} else if p.QosLevel == 2 {
			ack := codec.NewPubrecMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			//log.Debug("Send pubrec message to sender. [%s: %d]", conn.GetId(), ack.PacketIdentifier)
		}

		// TODO: QoSによっては適切なMessageIDを追加する
		// Server / ClientはそれぞれMessageTableが違う
		if p.QosLevel > 0 {
			// TODO: と、いうことはメッセージの deep コピーが簡単にできるようにしないとだめ
			// 色々考えると面倒だけど、ひとまずはフルコピーでやっとこう
			//			id := conn.GetOutGoingTable().NewId()
			//			p.PacketIdentifier = id
			conn.GetOutGoingTable().Register(p.PacketIdentifier, p, conn)
			p.Opaque = conn
		}

		self.Queue <- p
		break

	case codec.PACKET_TYPE_PUBACK:
		//pubackを受け取る、ということはserverがsender
		p := msg.(*codec.PubackMessage)
		//log.Debug("Received Puback Message from [%s: %d]", conn.GetId(), p.PacketIdentifier)

		// TODO: これのIDは内部的なの？
		self.OutGoingTable.Unref(p.PacketIdentifier)

		conn.GetOutGoingTable().Unref(p.PacketIdentifier)
		// TODO: Refcounting
		// PUBACKが帰ってくるのはServer->ClientでQoS1で送った時だけ。
		// PUBACKが全員分かえってきたらメッセージを消してもいい。実装自体はcallbackでやってるのでここでは下げるだけ
		break

	case codec.PACKET_TYPE_PUBREC:
		//pubrecを受け取る、ということはserverがsender
		p := msg.(*codec.PubrecMessage)
		//log.Debug("Received Pubrec Message from [%s: %d]", conn.GetId(), p.PacketIdentifier)

		ack := codec.NewPubrelMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		conn.WriteMessageQueue(ack)
		conn.GetOutGoingTable().Unref(p.PacketIdentifier)

		break

	case codec.PACKET_TYPE_PUBREL:
		//pubrelを受け取る、ということはserverがreceiver
		p := msg.(*codec.PubrelMessage)
		//log.Debug("Received Pubrel Message: [%s: %d]", conn.GetId(), p.PacketIdentifier)

		ack := codec.NewPubcompMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		conn.WriteMessageQueue(ack)
		//log.Debug("Send pubcomp message to sender. [%s: %d]", conn.GetId(), ack.PacketIdentifier)
		break

	case codec.PACKET_TYPE_PUBCOMP:
		//pubcompを受け取る、ということはserverがsender
		log.Debug("Received Pubcomp Message from %s: %+v", conn.GetId(), msg)
		p := msg.(*codec.PubcompMessage)
		self.OutGoingTable.Unref(p.PacketIdentifier)
		conn.GetOutGoingTable().Unref(p.PacketIdentifier)
		break

	case codec.PACKET_TYPE_SUBSCRIBE:
		p := msg.(*codec.SubscribeMessage)
		log.Debug("Subscribe Message: [%s] %+v\n", conn.GetId(), p)

		ack := codec.NewSubackMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		var retained []*codec.PublishMessage
		for _, payload := range p.Payload {
			// don't subscribe multiple time
			if _, exists := self.SubscribeMap[conn.GetId()]; exists {
				log.Debug("Map exists. [%s:%s]", conn.GetId(), payload.TopicPath)
				continue
			}

			self.SubscribeMap[conn.GetId()] = payload.TopicPath

			self.Qlobber.Add(payload.TopicPath, conn)
			conn.AppendSubscribedTopic(payload.TopicPath, int(payload.RequestedQos))

			// NOTE: We don't use topic for sending any messages. this is for statistic
			if !self.HasTopic(payload.TopicPath) {
				self.CreateTopic(payload.TopicPath)
			}

			retaines := self.RetainMatch(payload.TopicPath)
			if len(retaines) > 0 {
				for i := range retaines {
					id := conn.GetOutGoingTable().NewId()

					pp, _ := codec.CopyPublishMessage(retaines[i])
					pp.PacketIdentifier = id
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
		log.Error("Received not supported message [%d]. probably invalid sequence", msg.GetType())
		return errors.New("Invalid protocol sequence")
	}

	return nil
}

// TODO: wanna implement trie. but regexp works well.
func (self *Momonga) RetainMatch(topic string) []*codec.PublishMessage {
	var result []*codec.PublishMessage
	orig := topic

	topic = strings.Replace(topic, "+", "[^/]+", -1)
	topic = strings.Replace(topic, "#", ".*", -1)

	reg, err := regexp.Compile(topic)
	if err != nil {
		fmt.Printf("Regexp Error: %s", err)
	}

	all := false
	if string(orig[0:1]) == "#" || string(orig[0:1]) == "+" {
		all = true
	}

	for k, v := range self.Retain {
		if all && (len(k) > 0 && k[0:1] == "$") {
			// [MQTT-4.7.2-1] The Server MUST NOT match Topic Filters starting with a wildcard character (# or +)
			// with Topic Names beginning with a $ character
			// NOTE: Qlobber doesn't support this feature yet
			continue
		}
		if reg.MatchString(k) {
			result = append(result, v)
		}
	}

	return result
}


// below methods are intend to maintain engine itself (remove needless connection, dispatch queue).
func (self *Momonga) RunMaintenanceThread() {
	for {
		// TODO: implement $SYS here.
//		log.Debug("Current Conn: %d", len(self.Connections))
//		for i := range self.Connections {
//			log.Debug("  %+v", self.Connections[i])
//		}
		time.Sleep(time.Second)
	}
}

func (self *Momonga) Run() {
	go self.RunMaintenanceThread()

	// TODO: improve this
	for {
		select {
			// this is kind of a Write Queue
		case msg := <-self.Queue:
			switch msg.GetType() {
			case codec.PACKET_TYPE_PUBLISH:
				// NOTE: ここは単純にdestinationに対して送る、だけにしたい

				m := msg.(*codec.PublishMessage)
				log.Debug("sending PUBLISH [id:%d, lvl:%d]", m.PacketIdentifier, m.QosLevel)
				// TODO: Have to persist retain message.
				if m.Retain > 0 {
					if len(m.Payload) == 0 {
						delete(self.Retain, m.TopicName)
					} else {
						self.Retain[m.TopicName] = m
					}
				}

				// Publishで受け取ったMessageIdのやつのCountをとっておく
				// で、Pubackが帰ってきたらrefcountを下げて0になったらMessageを消す
				log.Debug("TopicName: %s %s", m.TopicName, m.Payload)
				targets := self.Qlobber.Match(m.TopicName)

				if m.TopicName[0:1] == "#" {
					// TODO:  [MQTT-4.7.2-1] The Server MUST NOT match Topic Filters starting with a wildcard character
					// (# or +) with Topic Names beginning with a $ character
				}

				for i := range targets {
					cn := targets[i].(Connection)
					x, err := codec.CopyPublishMessage(m)
					if err != nil {
						continue
					}

					subscriberQos := cn.GetSubscribedTopicQos(m.TopicName)

					// Downgrade QoS
					if subscriberQos < x.QosLevel {
						x.QosLevel = subscriberQos
					}
					if x.QosLevel > 0 {
						// TODO: ClientごとにInflightTableを持つ
						// engineのOutGoingTableなのはとりあえず、ということ
						id := self.OutGoingTable.NewId()
						x.PacketIdentifier = id
						if sender, ok := x.Opaque.(Connection); ok {
							self.OutGoingTable.Register2(x.PacketIdentifier, x, len(targets), sender)
						}
					}
					log.Debug("sending publish message to %s [%s %s %d %d]", targets[i].(Connection).GetId(), x.TopicName, x.Payload, x.PacketIdentifier, x.QosLevel)
					cn.WriteMessageQueue(x)
				}
				break
			default:
				log.Debug("WHAAAAAT?: %+v", msg)
			}
		case r := <-self.ErrorChannel:
			self.RetryMap[r.Id] = append(self.RetryMap[r.Id], r)
			log.Debug("ADD RETRYABLE MAP. But we don't do anything")
			break
		}
	}
}
