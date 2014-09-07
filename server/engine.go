package server

import (
	"errors"
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
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
