// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
)

// Handler dispatches messages which sent by client.
// this struct will be use client library soon.
//
// とかいいつつ、ackとかはhandlerで返してねーとか立ち位置分かりづらい
// Engine側でMQTTの基本機能を全部やれればいいんだけど、そうすると
// client library別にしないと無理なんだよなー。
// 目指すところとしては、基本部分はデフォルトのHandlerで動くから
// それで動かないところだけうわがいてね！って所。
// Handler自体は受け渡ししかやらんのでlockしなくて大丈夫なはず
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

func (self *Handler) Close() {
	self.Engine = nil
	self.Connection = nil
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

func (self *Handler) Unsubscribe(messageId uint16, granted int, payloads []codec.SubscribePayload) {
	log.Debug("Received unsubscribe from [%s]: %s\n", self.Connection.GetId(), messageId)
	self.Engine.Unsubscribe(messageId, granted, payloads, self.Connection)
}

func (self *Handler) Disconnect() {
	log.Debug("Received disconnect from %s", self.Connection.GetId())
	if cn, ok := self.Connection.(*MyConnection); ok {
		cn.Disconnect()
	}

	self.Engine.System.Broker.Clients.Connected--
	//return &DisconnectError{}
}

func (self *Handler) Pingreq() {
	r := codec.NewPingrespMessage()
	self.Connection.WriteMessageQueue(r)
}

func (self *Handler) Publish(p *codec.PublishMessage) {
	//log.Info("Received Publish Message: %s: %+v", p.PacketIdentifier, p)
	conn := self.Connection

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
	// Server / ClientはそれぞれMessageTableが違う
	if p.QosLevel > 0 {
		// TODO: と、いうことはメッセージの deep コピーが簡単にできるようにしないとだめ
		// 色々考えると面倒だけど、ひとまずはフルコピーでやっとこう
		//			id := conn.GetOutGoingTable().NewId()
		//			p.PacketIdentifier = id
		conn.GetOutGoingTable().Register(p.PacketIdentifier, p, conn)
		p.Opaque = conn
	}

	go self.Engine.SendPublishMessage(p)
}

func (self *Handler) Subscribe(p *codec.SubscribeMessage) {
	self.Engine.Subscribe(p, self.Connection)
}

func (self *Handler) HandshakeInternal(p *codec.ConnectMessage) {
	var conn *MyConnection
	var ok bool

	if conn, ok = self.Connection.(*MyConnection); !ok {
		log.Debug("wrong sequence.")
		self.Connection.Close()
		return
	}

	mux := self.Engine.Handshake(p, conn)
	if mux != nil {
		self.Connection = mux
	}
}
