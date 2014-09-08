package server

import (
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
)


type Handler struct{
	Engine *Momonga
	Connection Connection
}

func NewHandler(conn Connection, engine *Momonga) *Handler {
	hndr := &Handler{
		Engine: engine,
		Connection: conn,
	}
	if cn, ok := conn.(*MyConnection); ok {
		// Defaultの動作ではなんともいえないから上書きが必要なもの
		cn.On("parsed", hndr.Parsed, true)
		cn.On("connect", hndr.HandshakeInternal, true)
		cn.On("disconnect", hndr.Disconnect, true)

		cn.On("publish", hndr.Publish, true)
		cn.On("subscribe", hndr.Subscribe, true)
		cn.On("unsubscribe", hndr.Unsubscribe, true)

		cn.On("pingreq", hndr.Pingreq, true)

		// Defaultの動作で大丈夫なもの(念のため)
		cn.On("puback", hndr.Puback, true)
		cn.On("pubrec", hndr.Pubrec, true)
		cn.On("pubrel", hndr.Pubrel, true)
		cn.On("pubcomp", hndr.Pubcomp, true)
	}


	return hndr
}

func (self *Handler) Parsed() {
	self.Engine.System.Broker.Messages.Received++
}

func (self *Handler) Pubcomp(messageId uint16) {
	//pubcompを受け取る、ということはserverがsender
	log.Debug("Received Pubcomp Message from %s", self.Connection.GetId())

	self.Engine.OutGoingTable.Unref(messageId)
	self.Connection.GetOutGoingTable().Unref(messageId)
}

func (self *Handler) Pubrel(messageId uint16) {
	ack := codec.NewPubcompMessage()
	ack.PacketIdentifier = messageId
	self.Connection.WriteMessageQueue(ack)
	log.Debug("Send pubcomp message to sender. [%s: %d]", self.Connection.GetId(), messageId)
}

func (self *Handler) Pubrec(messageId uint16) {
	ack := codec.NewPubrelMessage()
	ack.PacketIdentifier = messageId

	self.Connection.WriteMessageQueue(ack)
	self.Connection.GetOutGoingTable().Unref(messageId)
}

func (self *Handler) Puback(messageId uint16) {
	log.Debug("Received Puback Message from [%s: %d]", self.Connection.GetId(), messageId)

	// TODO: これのIDは内部的なの？
	self.Engine.OutGoingTable.Unref(messageId)
	self.Connection.GetOutGoingTable().Unref(messageId)
}

func (self *Handler) Unsubscribe(messageId uint16, granted int, payloads []*codec.SubscribePayload) {
	log.Debug("Received unsubscribe from [%s]: %s\n", self.Connection.GetId(), messageId)
	for _, payload := range payloads {
		ack := codec.NewUnsubackMessage()
		ack.PacketIdentifier = messageId

		self.Connection.WriteMessageQueue(ack)
		self.Engine.Qlobber.Remove(payload.TopicPath, self.Connection)
	}
}

func (self *Handler) Disconnect() {
	log.Debug("Received disconnect from %s", self.Connection.GetId())
	if cn, ok := self.Connection.(*MyConnection); ok {
		cn.Disconnect()
	}

	self.Engine.System.Broker.Clients.Connected--
	//return &DisconnectError{}
}

func (self *Handler) Pingreq(conn Connection) {
	r := codec.NewPingrespMessage()
	conn.WriteMessageQueue(r)
}

func (self *Handler) Publish(p *codec.PublishMessage) {
	//log.Debug("Received Publish Message: %s: %+v", p.PacketIdentifier, p)
	conn := self.Connection
	if !self.Engine.HasTopic(p.TopicName) {
		self.Engine.CreateTopic(p.TopicName)
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

	self.Engine.Queue <- p
}

func (self *Handler) Subscribe(p *codec.SubscribeMessage) {
	conn := self.Connection
	log.Debug("Subscribe Message: [%s] %+v\n", conn.GetId(), p)

	ack := codec.NewSubackMessage()
	ack.PacketIdentifier = p.PacketIdentifier
	var retained []*codec.PublishMessage
	for _, payload := range p.Payload {
		// don't subscribe multiple time
		if _, exists := self.Engine.SubscribeMap[conn.GetId()]; exists {
			log.Debug("Map exists. [%s:%s]", conn.GetId(), payload.TopicPath)
			continue
		}

		self.Engine.SubscribeMap[conn.GetId()] = payload.TopicPath

		self.Engine.Qlobber.Add(payload.TopicPath, conn)
		conn.AppendSubscribedTopic(payload.TopicPath, int(payload.RequestedQos))

		// NOTE: We don't use topic for sending any messages. this is for statistic
		if !self.Engine.HasTopic(payload.TopicPath) {
			self.Engine.CreateTopic(payload.TopicPath)
		}

		retaines := self.Engine.RetainMatch(payload.TopicPath)
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

	if len(retained) > 0 {
		log.Debug("Send retained Message To: %s", conn.GetId())
		for i := range retained {
			conn.WriteMessageQueue(retained[i])
		}
	}
}

func (self *Handler) HandshakeInternal(p *codec.ConnectMessage) {
	log.Debug("handshaking: %s", p.Identifier)
	var conn *MyConnection
	var ok bool

	if conn, ok = self.Connection.(*MyConnection); !ok {
		log.Debug("wrong sequence.")
		self.Connection.Close()
		return
	}

	if conn.Connected == true {
		log.Debug("wrong sequence.")
		self.Connection.Close()
		return
	}

	if !(string(p.Magic) == "MQTT" || string(p.Magic) == "MQIsdp") {
		log.Debug("magic is not expected: %s  %+v\n", string(p.Magic), p)
		self.Connection.Close()
		return
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
	session_exist := false

	reply := codec.NewConnackMessage()
	if session_exist {
		// [MQTT-3.2.2-2] If the Server accepts a connection with CleanSession set to 0,
		// the value set in Session Present depends on whether the Server already has stored Session state
		// for the supplied client ID. If the Server has stored Session state,
		// it MUST set Session Present to 1 in the CONNACK packet.
		reply.Reserved |= 0x01
	}
	conn.WriteMessageQueue(reply)

	if mux, ok = self.Engine.Connections[p.Identifier]; ok {
		mux.Attach(conn)
		session_exist = true
		log.Debug("Attach to mux[%s]", mux.GetId())
	} else {
		mux = NewMmuxConnection()
		mux.Identifier = p.Identifier
		mux.Attach(conn)
	}

	conn.SetId(p.Identifier)
	conn.SetKeepaliveInterval(int(p.KeepAlive))
	mux.SetKeepaliveInterval(int(p.KeepAlive))

	mux.SetState(STATE_CONNECTED)
	self.Engine.Connections[mux.GetId()] = mux

	if p.CleanSession {
		delete(self.Engine.RetryMap, mux.GetId())
		self.Engine.CleanSubscription(mux)

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

	if cn, ok := self.Connection.(*MyConnection); ok {
		cn.Connected = true
	}
	self.Connection = mux
	log.Debug("handshake Successful: %s", p.Identifier)

	self.Engine.System.Broker.Clients.Connected++
}
