// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"fmt"
	. "github.com/chobie/momonga/common"
	"github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"sync"
)

type DummyPlug struct {
	Identity string
	Switch   chan bool
	message  chan mqtt.Message
	Running  bool
	stop     chan bool
	mutex    sync.RWMutex
	engine   *Momonga
}

func NewDummyPlug(engine *Momonga) *DummyPlug {
	d := &DummyPlug{
		Identity: "dummy",
		Switch:   make(chan bool),
		message:  make(chan mqtt.Message, 256),
		stop:     make(chan bool, 1),
		engine:   engine,
	}

	go d.Run()
	return d
}

func (self *DummyPlug) Stop() {
	self.stop <- true
}

func (self *DummyPlug) Run() {
	for {
		select {
		case <-self.stop:
			return
		case b := <-self.Switch:
			self.Running = b
		case m := <-self.message:
			if self.Running {
				// メッセージが来たらengineのAPIをたたけばOK
				switch m.GetType() {
				case mqtt.PACKET_TYPE_CONNECT:
				case mqtt.PACKET_TYPE_CONNACK:
				case mqtt.PACKET_TYPE_PUBLISH:
					// TODO:
					if p, ok := m.(*mqtt.PublishMessage); ok {
						fmt.Printf("%s\n", p)
						switch p.QosLevel {
						case 1:
							log.Debug("[DUMMY] Received Publish Message from [%d]. reply puback", p.PacketIdentifier)
							self.engine.OutGoingTable.Unref(p.PacketIdentifier)
						case 2:
							log.Debug("[DUMMY] Received Publish Message from [%d]. reply pubrec", p.PacketIdentifier)
							self.engine.OutGoingTable.Unref(p.PacketIdentifier)
							rel := mqtt.NewPubrelMessage()
							rel.PacketIdentifier = p.PacketIdentifier
							// NOTE: client have to reply pubrec to the server
							self.message <- rel
						}
					}
				case mqtt.PACKET_TYPE_DISCONNECT:
					// TODO: なの？
				case mqtt.PACKET_TYPE_SUBSCRIBE:
				case mqtt.PACKET_TYPE_SUBACK:
				case mqtt.PACKET_TYPE_UNSUBSCRIBE:
				case mqtt.PACKET_TYPE_UNSUBACK:
				case mqtt.PACKET_TYPE_PINGRESP:
				case mqtt.PACKET_TYPE_PINGREQ:
				case mqtt.PACKET_TYPE_PUBACK:
					// TODO: (nothign to do)
				case mqtt.PACKET_TYPE_PUBREC:
					// TODO: (nothign to do)
				case mqtt.PACKET_TYPE_PUBREL:
					// TODO:
					if p, ok := m.(*mqtt.PubrelMessage); ok {
						log.Debug("[DUMMY] Received Pubrel Message from [%d]. send pubcomp", p.PacketIdentifier)
						self.engine.OutGoingTable.Unref(p.PacketIdentifier)

						cmp := mqtt.NewPubcompMessage()
						cmp.PacketIdentifier = p.PacketIdentifier
						// NOTE: client have to reply pubcomp to the server
						self.message <- cmp
					}
				case mqtt.PACKET_TYPE_PUBCOMP:
					// TODO: (nothing)
				default:
					return
				}
			} else {
				// discards message
			}
		}
	}
}

func (self *DummyPlug) WriteMessageQueue(request mqtt.Message) {
	switch request.GetType() {
	case mqtt.PACKET_TYPE_CONNECT:
	case mqtt.PACKET_TYPE_CONNACK:
	case mqtt.PACKET_TYPE_PUBLISH:
		self.message <- request
	case mqtt.PACKET_TYPE_DISCONNECT:
		self.message <- request
	case mqtt.PACKET_TYPE_SUBSCRIBE:
	case mqtt.PACKET_TYPE_SUBACK:
	case mqtt.PACKET_TYPE_UNSUBSCRIBE:
	case mqtt.PACKET_TYPE_UNSUBACK:
	case mqtt.PACKET_TYPE_PINGRESP:
	case mqtt.PACKET_TYPE_PINGREQ:
	case mqtt.PACKET_TYPE_PUBACK:
		self.message <- request
	case mqtt.PACKET_TYPE_PUBREC:
		self.message <- request
	case mqtt.PACKET_TYPE_PUBREL:
		self.message <- request
	case mqtt.PACKET_TYPE_PUBCOMP:
		self.message <- request
	default:
		return
	}
}

func (self *DummyPlug) WriteMessageQueue2(msg []byte) {
	return
}

func (self *DummyPlug) Close() error {
	return nil
}

func (self *DummyPlug) SetState(State) {
}

func (self *DummyPlug) GetState() State {
	return STATE_CONNECTED
}
func (self *DummyPlug) ResetState() {
}

func (self *DummyPlug) ReadMessage() (mqtt.Message, error) {
	return nil, nil
}

func (self *DummyPlug) IsAlived() bool {
	return true
}

func (self *DummyPlug) SetWillMessage(mqtt.WillMessage) {
	panic("strange state")
}

func (self *DummyPlug) GetWillMessage() *mqtt.WillMessage {
	panic("strange state")
	return nil
}

func (self *DummyPlug) HasWillMessage() bool {
	panic("strange state")
	return false
}

func (self *DummyPlug) GetOutGoingTable() *util.MessageTable {
	return nil
}

func (self *DummyPlug) GetSubscribedTopics() map[string]*SubscribeSet {
	panic("strange state")
	return nil
}

func (self *DummyPlug) AppendSubscribedTopic(string, *SubscribeSet) {
	panic("strange state")
	return
}
func (self *DummyPlug) RemoveSubscribedTopic(string) {
	panic("strange state")
	return
}

func (self *DummyPlug) SetKeepaliveInterval(int) {
	panic("strange state")
	return
}

func (self *DummyPlug) GetId() string {
	return self.Identity
}
func (self *DummyPlug) GetRealId() string {
	return self.Identity
}

func (self *DummyPlug) SetId(id string) {
	self.Identity = id
}

func (self *DummyPlug) DisableClearSession() {
	return
}

func (self *DummyPlug) ShouldClearSession() bool {
	return false
}

func (self *DummyPlug) GetGuid() util.Guid {
	return util.Guid(0)
}

func (self *DummyPlug) SetGuid(id util.Guid) {
}
