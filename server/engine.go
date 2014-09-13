// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"bytes"
	_ "errors"
	"fmt"
	"github.com/chobie/momonga/datastore"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"regexp"
	"strings"
	"time"
	"sync"
	"encoding/hex"
	"encoding/binary"
	"encoding/json"
	"io"
)

type DisconnectError struct {
}

func (e *DisconnectError) Error() string { return "received disconnect message" }

// TODO: haven't used this yet.
type Retryable struct {
	Id      string
	Payload interface{}
}

type SubscribeSet struct {
	ClientId string `json:"client_id"`
	TopicFilter string `json:"topic_filter"`
	QoS int `json:"qos"`
}

func (self *SubscribeSet) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}

// QoS 1, 2 are available. but really suck implementation.
// reconsider qos design later.
func NewMomonga() *Momonga {
	engine := &Momonga{
		publishQueue:         make(chan *codec.PublishMessage, 8192),
		OutGoingTable: util.NewMessageTable(),
		Qlobber:       util.NewQlobber(),
		Connections:   map[string]*MmuxConnection{},
		RetryMap:      map[string][]*Retryable{},
		ErrorChannel:  make(chan *Retryable, 8192),
		Started: time.Now(),
		EnableSys: false,
		DataStore: datastore.NewMemstore(),
		LockPool: map[uint32]*sync.RWMutex{},
	}

	// initialize lock pool
	for i := 0; i < 64; i++ {
		engine.LockPool[uint32(i)] = &sync.RWMutex{}
	}

	return engine
}

/*
goroutine (2)
	RunMaintenanceThread
	Run
*/
type Momonga struct {
	publishQueue         chan *codec.PublishMessage
	OutGoingTable *util.MessageTable
	Qlobber       *util.Qlobber
	// TODO: improve this.
	Connections  map[string]*MmuxConnection
	RetryMap     map[string][]*Retryable
	ErrorChannel chan *Retryable
	System       System
	EnableSys    bool
	Started      time.Time
	DataStore    datastore.Datastore
	LockPool     map[uint32]*sync.RWMutex
}

func (self *Momonga) DisableSys() {
	self.EnableSys = false
}

func (self *Momonga) Terminate() {
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

		self.SendPublishMessage(msg)
	}
}

func (self *Momonga) CleanSubscription(conn Connection) {
	for t, v := range conn.GetSubscribedTopics() {
		self.Qlobber.Remove(t, v)
	}
}

func (self *Momonga) SendWillMessage(conn Connection) {
	will := conn.GetWillMessage()
	msg := codec.NewPublishMessage()
	msg.TopicName = will.Topic
	msg.Payload = []byte(will.Message)
	msg.QosLevel = int(will.Qos)

	self.SendPublishMessage(msg)
}

// TODO: wanna implement trie. but regexp works well.
// retain should persist their data. though, how do we fetch it efficiently...
func (self *Momonga) RetainMatch(topic string) []*codec.PublishMessage {
	var result []*codec.PublishMessage
	orig := topic

	// TODO: should validate illegal wildcards like /debug/#/hello
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

	// TODO:これどうにかしたいなー・・・。とは思いつつDBからめて素敵にやるってなるとあんまりいいアイデアない
	itr := self.DataStore.Iterator()
	for ; itr.Valid(); itr.Next() {
		k := string(itr.Key())

		if all && (len(k) > 0 && k[0:1] == "$") {
			// [MQTT-4.7.2-1] The Server MUST NOT match Topic Filters starting with a wildcard character (# or +)
			// with Topic Names beginning with a $ character
			// NOTE: Qlobber doesn't support this feature yet
			continue
		}

		if reg.MatchString(k) {
			data := itr.Value()
			p, _ := codec.ParseMessage(bytes.NewReader(data), 0)
			if v, ok := p.(*codec.PublishMessage); ok {
				result = append(result, v)
			}
		}
	}

	return result
}

func (self *Momonga) Subscribe(p *codec.SubscribeMessage, conn Connection) {
	log.Debug("Subscribe Message: [%s] %+v\n", conn.GetId(), p)

	ack := codec.NewSubackMessage()
	ack.PacketIdentifier = p.PacketIdentifier
	cn := conn.(*MmuxConnection)

	var retained []*codec.PublishMessage
	// どのレベルでlockするか
	qosBuffer := bytes.NewBuffer(nil)
	for _, payload := range p.Payload {
		// don't subscribe multiple time
		if cn.IsSubscribed(payload.TopicPath) {
			log.Error("Map exists. [%s:%s]", conn.GetId(), payload.TopicPath)
			continue
		}

		set := &SubscribeSet{
			TopicFilter: payload.TopicPath,
			ClientId: conn.GetId(),
			QoS: int(payload.RequestedQos),
		}
		binary.Write(qosBuffer, binary.BigEndian, payload.RequestedQos)

		self.Qlobber.Add(payload.TopicPath, set)
		conn.AppendSubscribedTopic(payload.TopicPath, set)
		retaines := self.RetainMatch(payload.TopicPath)

		if len(retaines) > 0 {
			for i := range retaines {
				log.Debug("Retains: %s", retaines[i].TopicName)
				id := conn.GetOutGoingTable().NewId()

				pp, _ := codec.CopyPublishMessage(retaines[i])
				pp.PacketIdentifier = id
				conn.GetOutGoingTable().Register(id, pp, conn)
				retained = append(retained, pp)
			}
		}
	}
	ack.Qos = qosBuffer.Bytes()


	// MEMO: we can reply directly, no need to route, persite message.
	log.Debug("Send Suback Message To: %s", conn.GetId())
	conn.WriteMessageQueue(ack)
	if len(retained) > 0 {
		log.Debug("Send retained Message To: %s", conn.GetId())
		for i := range retained {
			conn.WriteMessageQueue(retained[i])
		}
	}

}

func (self *Momonga) SendMessage(topic string, message []byte, qos int) {
	msg := codec.NewPublishMessage()
	msg.TopicName = topic
	msg.Payload = message
	msg.QosLevel = qos
	self.SendPublishMessage(msg)
}

func (self *Momonga) GetConnectionByClientId(clientId string) (*MmuxConnection, error){
	hash := util.MurmurHash([]byte(clientId))
	key := hash % 64

	self.LockPool[key].RLock()
	if cn, ok := self.Connections[clientId]; ok {
		self.LockPool[key].RUnlock()
		return cn, nil
	}

	self.LockPool[key].RUnlock()
	return nil, fmt.Errorf("not found")
}

func (self *Momonga) SetConnectionByClientId(clientId string, conn *MmuxConnection) {
	hash := util.MurmurHash([]byte(clientId))
	key := hash % 64

	self.LockPool[key].Lock()
	defer self.LockPool[key].Unlock()
	self.Connections[clientId] = conn
}

func (self *Momonga) SendPublishMessage(msg *codec.PublishMessage) {
	// Don't pass wrong message here. user should validate the message be ore using this API.
	if len(msg.TopicName) < 1 {
		return
	}

	// TODO: Have to persist retain message.
	if msg.Retain > 0 {
		if len(msg.Payload) == 0 {
			log.Debug("[DELETE RETAIN: %s]\n%s", msg.TopicName,hex.Dump([]byte(msg.TopicName)))

			err := self.DataStore.Del([]byte(msg.TopicName), []byte(msg.TopicName))
			if err != nil {
			    log.Error("Error: %s\n", err)
			}
			// これ配送したらおかしいべ
			log.Debug("Deleted retain: %s", msg.TopicName)
			// あれ、ackとかかえすんだっけ？
			return
		} else {
			buffer := bytes.NewBuffer(nil)
			codec.WriteMessageTo(msg, buffer)
			self.DataStore.Put([]byte(msg.TopicName), buffer.Bytes())
		}
	}

	// Publishで受け取ったMessageIdのやつのCountをとっておく
	// で、Pubackが帰ってきたらrefcountを下げて0になったらMessageを消す
	//log.Debug("TopicName: %s %s", m.TopicName, m.Payload)
	targets := self.Qlobber.Match(msg.TopicName)
	if msg.TopicName[0:1] == "#" {
		// TODO:  [MQTT-4.7.2-1] The Server MUST NOT match Topic Filters starting with a wildcard character
		// (# or +) with Topic Names beginning with a $ character
	}


	// list つくってからとって、だとタイミング的に居ない奴も出てくるんだよな。マジカオス
	// ここで必要なのは, connection(clientId), subscribed qosがわかればあとは投げるだけ
	// なんで、Qlobberがかえすのはであるべきなんだけど。これすっげー消しづらいのよね・・・
	// {
	//    Connection: Connection or client id
	//    QoS:
	// }
	// いやまぁエラーハンドリングちゃんとやってれば問題ない。
	// client idのほうがベターだな。Connectionを無駄に参照つけると後が辛い
	dp := make(map[string]bool)
	for i := range targets {
		var cn Connection
		var ok error

		myset := targets[i].(*SubscribeSet)
		clientId := myset.ClientId
		//clientId := targets[i].(string)

		// NOTE (from interoperability/client_test.py):
		//
		//   overlapping subscriptions. When there is more than one matching subscription for the same client for a topic,
		//   the server may send back one message with the highest QoS of any matching subscription, or one message for
		//   each subscription with a matching QoS.
		//
		// Currently, We choose one message for each subscription with a matching QoS.
		//
		if _, ok := dp[clientId]; ok {
			continue
		}
		dp[clientId] = true

		cn, ok = self.GetConnectionByClientId(clientId)
		if ok != nil {
			// どちらかというとClientが悪いと思うよ！
			// リスト拾った瞬間にはいたけど、その後いなくなったんだから配送リストからこれらは無視すべき
			log.Info("(%s can't fetch. already disconnected, or unsubscribed?)", clientId)
			continue
		}

		// 出来る限りコピーしないような方法を取りたいけど色々考えないといかん
		x, err := codec.CopyPublishMessage(msg)
		if err != nil {
			log.Error("COPY MESSAGE FAILED")
			continue
		}

		subscriberQos := myset.QoS
		// Downgrade QoS
		if subscriberQos < x.QosLevel {
			x.QosLevel = subscriberQos
		}

		if x.QosLevel < 0 {
			// もうこれはおきないべー、ってのを確認
			panic(fmt.Sprintf("qos under zero : %d(%s can't fetch. already unsubscribe or disconntected?)\n", x.QosLevel, clientId))
			continue
		}

		if x.QosLevel > 0 {
			// TODO: ClientごとにInflightTableを持つ
			// engineのOutGoingTableなのはとりあえず、ということ
			id := self.OutGoingTable.NewId()
			x.PacketIdentifier = id
			if sender, ok := x.Opaque.(Connection); ok {
				// TODO: ここ
				self.OutGoingTable.Register2(x.PacketIdentifier, x, len(targets), sender)
			}
		}
		
		x.Opaque = cn
		self.publishQueue <- x
	}
}

// below methods are intend to maintain engine itself (remove needless connection, dispatch queue).
func (self *Momonga) RunMaintenanceThread() {
	for {
		// TODO: implement $SYS here.
		//		log.Debug("Current Conn: %d", len(self.Connections))
		//		for i := range self.Connections {
		//			log.Debug("  %+v", self.Connections[i])
		//		}

		//		select {
		//			case tuple := <- self.SysUpdateRequest:
		//		default:
		// TODO: だれかがsubscribeしてる時だけ出力する
		// TODO: implement whole stats
		if self.EnableSys {
			now := time.Now()
			self.System.Broker.Broker.Uptime = int(now.Sub(self.Started) / 1e9)
			self.SendMessage("$SYS/broker/broker/uptime", []byte(fmt.Sprintf("%d", self.System.Broker.Broker.Uptime)), 0)
			self.SendMessage("$SYS/broker/broker/time", []byte(fmt.Sprintf("%d", now.Unix())), 0)
			self.SendMessage("$SYS/broker/clients/connected", []byte(fmt.Sprintf("%d", self.System.Broker.Clients.Connected)), 0)
			self.SendMessage("$SYS/broker/messages/received", []byte(fmt.Sprintf("%d", self.System.Broker.Messages.Received)), 0)
			self.SendMessage("$SYS/broker/messages/sent", []byte(fmt.Sprintf("%d", self.System.Broker.Messages.Sent)), 0)
			self.SendMessage("$SYS/broker/messages/stored", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/messages/publish/dropped", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/messages/retained/count", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/messages/inflight", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/clients/total", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/clients/maximum", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/clients/disconnected", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/load/bytes/sent", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/load/bytes/received", []byte(fmt.Sprintf("%d", 0)), 0)
			self.SendMessage("$SYS/broker/subscriptions/count", []byte(fmt.Sprintf("%d", 0)), 0)
		}

		time.Sleep(time.Second)
	}
}

func (self *Momonga) Work() {
	// TODO: improve this
	for {
		select {
		// this is kind of a Write Queue.
		// 個々を並列化するためにはlockとかをきちんとやらないといけない
		case m := <-self.publishQueue:
			// NOTE: ここは単純にdestinationに対して送る、だけにしたい
			// 時間がかかる処理をやってはいけない

			op := m.Opaque
			var cn Connection
			var ok bool

			if op == nil {
				log.Error("AREEEEEE")
				continue
			}

			if cn, ok = m.Opaque.(Connection); ok {
				cn.WriteMessageQueue(m)
				self.System.Broker.Messages.Sent++
			} else {
				log.Error("Opaque is not set")
			}
		case <-self.ErrorChannel:
			///self.RetryMap[r.Id] = append(self.RetryMap[r.Id], r)
			log.Debug("ADD RETRYABLE MAP. But we don't do anything")
			break
		// TODO: 止めるやつつける
		}
	}
}

func (self *Momonga) Run() {
	go self.RunMaintenanceThread()

	// TODO: use config
	for i := 0; i < 4; i++ {
		go self.Work()
	}
}


func (self *Momonga) Handshake(p *codec.ConnectMessage, conn *MyConnection) *MmuxConnection {
	log.Debug("handshaking: %s", p.Identifier)

	if conn.Connected == true {
		log.Error("wrong sequence. (connect twice)")
		conn.Close()
		return nil
	}

	if !(string(p.Magic) == "MQTT" || string(p.Magic) == "MQIsdp") {
		log.Error("magic is not expected: %s  %+v\n", string(p.Magic), p)
		conn.Close()
		return nil
	}

	//log.Debug("CONNECT [%s]: %+v", conn.GetId(), conn)
	// TODO: check version
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
	var err error

	reply := codec.NewConnackMessage()
	if mux, err = self.GetConnectionByClientId(p.Identifier); err == nil {
		// [MQTT-3.2.2-2] If the Server accepts a connection with CleanSession set to 0,
		// the value set in Session Present depends on whether the Server already has stored Session state
		// for the supplied client ID. If the Server has stored Session state,
		// it MUST set Session Present to 1 in the CONNACK packet.
		reply.Reserved |= 0x01
	}

	// CONNACK MUST BE FIRST RESPONSE
	// clean周りはAttachでぜんぶやるべきでは
	conn.WriteMessageQueue(reply)
	if mux != nil {
		log.Debug("Attach to mux[%s]", mux.GetId())

		conn.SetId(p.Identifier)
		conn.SetKeepaliveInterval(int(p.KeepAlive))
		mux.SetKeepaliveInterval(int(p.KeepAlive))
		mux.SetState(STATE_CONNECTED)
		mux.DisableClearSession()

		if conn.ShouldClearSession() {
			self.CleanSubscription(mux)
		}
		mux.Attach(conn)
	} else {
		mux = NewMmuxConnection()
		mux.SetId(p.Identifier)
		mux.Attach(conn)
		self.SetConnectionByClientId(p.Identifier, mux)

		conn.SetId(p.Identifier)
		conn.SetKeepaliveInterval(int(p.KeepAlive))
		mux.SetKeepaliveInterval(int(p.KeepAlive))
		mux.SetState(STATE_CONNECTED)

		log.Debug("Starting new mux[%s]", mux.GetId())
	}

	if p.CleanSession {
		delete(self.RetryMap, mux.GetId())
	} else {
		// Okay, attach to existing session.
		tbl := mux.GetOutGoingTable()

		// これは途中の再送処理
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

	conn.Connected = true
	log.Debug("handshake Successful: %s", p.Identifier)
	self.System.Broker.Clients.Connected++
	return mux
}

func (self *Momonga) Unsubscribe(messageId uint16, granted int, payloads []codec.SubscribePayload, conn Connection) {
	log.Info("UNSUBSCRIBE:")
	ack := codec.NewUnsubackMessage()
	ack.PacketIdentifier = messageId

	topics := conn.GetSubscribedTopics()
	for _, payload := range payloads {
		if v, ok := topics[payload.TopicPath]; ok {
			self.Qlobber.Remove(payload.TopicPath, v)
		}
	}
	conn.WriteMessageQueue(ack)
}


func (self *Momonga) HandleConnection(conn Connection) {
	hndr := NewHandler(conn, self)

	for {
		_, err := conn.ReadMessage()
		if conn.GetState() == STATE_CLOSED {
			err = &DisconnectError{}
		}

		if err != nil {
			log.Debug("DISCONNECT: %s", conn.GetId())
			// ここでmyconnがかえる場合はhandshake前に死んでる
			//self.Engine.CleanSubscription(conn)
			var ok bool
			var mux *MmuxConnection

			if mux, ok = self.Connections[conn.GetId()]; ok {
			}

			if _, ok := err.(*DisconnectError); !ok {
				if conn.HasWillMessage() {
					self.SendWillMessage(conn)
				}

				if err == io.EOF {
					// nothing to do
				} else {
					log.Error("Handle Connection Error: %s", err)
				}
			}

			if mux != nil {
				mux.Detach(conn)

				if mux.ShouldClearSession() {
					self.CleanSubscription(mux)
					delete(self.Connections, mux.GetId())
				}
			}

			conn.Close()
			hndr.Close()
			return
		}
	}
}
