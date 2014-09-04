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

type Pidgey struct {
	Topics map[string]*Topic
	Queue chan codec.Message
	OutGoingTable *util.MessageTable
	Qlobber *util.Qlobber
	// TODO: improve this.
	Retain map[string]*codec.PublishMessage
	SessionStore *SessionStore
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
			log.Debug("Not supported; %d", message.GetType())
		}
	})
}


func (self *Pidgey) handshake(conn Connection) error {
	msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	if msg.GetType() != codec.PACKET_TYPE_CONNECT {
		return errors.New("Invalid message")
	}
	p := msg.(*codec.ConnectMessage)
	log.Debug("CONNECT: %+v\n", p)

	if !(string(p.Magic) == "MQTT" || string(p.Magic) == "MQIsdp") {
		return errors.New("Invalid protocol")
	}

	// TODO: 認証部分をつける

	// TODO: Willがあればキープしておく
	if (p.Flag & 0x4 > 0) {
		log.Debug("This message has Will flag: %+v\n", p.Will)
		conn.SetWillMessage(*p.Will)
	}
	conn.SetKeepaliveInterval(int(p.KeepAlive))

	reply := codec.NewConnackMessage()
	// TODO: ClearSession Flag
	log.Debug("Send connack: %+v", reply)
	// Publishしかやっとらんかった
	//self.Queue <- reply
	err = conn.WriteMessage(reply)
	if err != nil {
		log.Debug("Error: %s", err)
	}
	conn.SetState(STATE_ACCEPTED)
	return nil
}

func (self *Pidgey) Handshake(conn Connection) error {
	err := self.handshake(conn)
	if err != nil {
		if err != io.EOF {
			msg := codec.NewDisconnectMessage()
			conn.WriteMessage(msg)
		}
		return err;
	}
	return nil
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
				fmt.Printf("Message Arrived(%d): spared to (%s): Retain: %d, %+v\n", m.QosLevel, m.TopicName, m.Retain, m.Payload)

				topic, err := self.GetTopic(m.TopicName)
				if err != nil{
					fmt.Printf("err: %s\n", err)
					//continue
				}

				// TODO: Retainがついてたら保持する
				//   Retainはサーバーが再起動しても保持しなければならない
				if m.Retain > 0 {
					fmt.Printf("Retain Flag\n")
					if len(m.Payload) == 0 {
						delete(self.Retain, m.TopicName)
					} else {
						self.Retain[m.TopicName] = m
						fmt.Printf("Retained message: %s: %+v %+v\n", m.TopicName, topic, topic.Retain)
					}
				}

				// Publishで受け取ったMessageIdのやつのCountをとっておく
				// で、Pubackが帰ってきたらrefcountを下げて0担ったらMessageを消す
				targets := self.Qlobber.Match(m.TopicName)
				fmt.Printf("targets; %+v\n", targets)
				if m.QosLevel > 0 {
					id := self.OutGoingTable.NewId()
					m.PacketIdentifier = id
					log.Debug("Count: %d", len(targets))
					if sender, ok := m.Opaque.(Connection); ok {
						self.OutGoingTable.Register2(m.PacketIdentifier, m, len(targets), sender)
					}
				}

				for i := range targets {
					targets[i].(*TcpConnection).WriteMessageQueue(m)
				}
				break
			default:
				log.Debug("ARE?: %+v", msg)
			}
		}
	}
}

func (self *Pidgey) HandleRequest(conn Connection) error {
	err := self.handle(conn)

	if err != nil {
		if conn.HasWillMessage() {
			will := conn.GetWillMessage()
			msg := codec.NewPublishMessage()
			msg.TopicName = will.Topic
			msg.Payload = []byte(will.Message)
			msg.QosLevel = int(will.Qos)
			self.Queue <- msg
		}

		// エラーなんで接続前にsubscriberからはずす
		for _, t := range conn.GetSubscribedTopics() {
//			topic, err := self.GetTopic(t)
//			if err != nil {
//				continue
//			}
//			topic.RemoveConnection(conn)
			self.Qlobber.Remove(t, conn)
		}
		return err
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
		log.Debug("Received Publish Message: %+v", p)
		log.Debug("message: %s", string(p.Payload))

		if !self.HasTopic(p.TopicName) {
			self.CreateTopic(p.TopicName)
		}

		if p.QosLevel == 1 {
			ack := codec.NewPubackMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			log.Debug("Send puback message to sender.")
		} else if p.QosLevel == 2 {
			ack := codec.NewPubrecMessage()
			ack.PacketIdentifier = p.PacketIdentifier
			conn.WriteMessageQueue(ack)
			log.Debug("Send pubrec message to sender.")
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
		log.Debug("Received Puback Message: %+v", p)

		// TODO: これのIDは内部的なの？
		self.OutGoingTable.Unref(p.PacketIdentifier)

		// TODO: Refcounting
		// PUBACKが帰ってくるのはServer->ClientでQoS1で送った時だけ。
		// PUBACKが全員分かえってきたらメッセージを消してもいい
		break

	case codec.PACKET_TYPE_PUBREC:
		p := msg.(*codec.PubrecMessage)
		log.Debug("Received Pubrec Message: %+v", p)
		ack := codec.NewPubrelMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		conn.WriteMessageQueue(ack)
		break

	case codec.PACKET_TYPE_PUBREL:
		log.Debug("Received Pubrel Message: %+v", msg)
		p := msg.(*codec.PubrelMessage)
		ack := codec.NewPubcompMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		conn.WriteMessageQueue(ack)
		log.Debug("Send pubcomp message to sender.")
		break

	case codec.PACKET_TYPE_PUBCOMP:
		log.Debug("Received Pubcomp Message: %+v", msg)
		p := msg.(*codec.PubcompMessage)
		self.OutGoingTable.Unref(p.PacketIdentifier)
		break

	case codec.PACKET_TYPE_SUBSCRIBE:
		p := msg.(*codec.SubscribeMessage)
		log.Debug("Subscribe Message: %+v\n", p)

		ack := codec.NewSubackMessage()
		ack.PacketIdentifier = p.PacketIdentifier
		var retained []*codec.PublishMessage
		for _, payload := range p.Payload {
			var topic *Topic

			self.Qlobber.Add(payload.TopicPath, conn)
			conn.AppendSubscribedTopic(payload.TopicPath)

			// TODO: これはAtomicにさせたいなー、とおもったり
			if !self.HasTopic(payload.TopicPath) {
				topic, _ = self.CreateTopic(payload.TopicPath)
			} else {
				topic, _ = self.GetTopic(payload.TopicPath)
			}

			retaines := self.RetainMatch(payload.TopicPath)
			log.Debug("retaines: %+v\n", retaines)
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
		log.Debug("Send Suback Message")
		conn.WriteMessageQueue(ack)

		log.Debug("Fanout Retained messages: %d", len(retained))
		for i := range retained {
			conn.WriteMessage(retained[i])
		}
		break

	case codec.PACKET_TYPE_UNSUBSCRIBE:
		p := msg.(*codec.UnsubscribeMessage)
		log.Debug("Received unsubscribe: %+v\n", p)
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
		return errors.New("Received Disconnect request: Closed")
	default:
		fmt.Printf("Not supported: %d\n", msg.GetType())
		// MEMO: これは接続切ってもいんじゃね？
		break
	}

	return nil
}

// これTrieにしたいんだけどめんどい
func (self *Pidgey) RetainMatch(topic string) []*codec.PublishMessage {
	var result []*codec.PublishMessage

	topic = strings.Replace(topic, "+", "[^/]+", -1)
	topic = strings.Replace(topic, "#", ".*", -1)

	fmt.Printf("topic: %s\n", topic)
	reg, err := regexp.Compile(topic)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	for k, v := range self.Retain {
		if reg.MatchString(k) {
			result = append(result, v)
		}
	}

	return result
}
