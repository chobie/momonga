package client

import (
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"io"
	"sync"
	"time"
	"errors"
)

type Option struct {
	TransporterCallback func() (io.ReadWriteCloser, error)
	Magic               []byte
	Version             int
	Identifier          string
	Ticker              *time.Ticker
	TickerCallback      func(time.Time, *Client) error
	WillTopic           string
	WillMessage         []byte
	WillRetain          bool
	WillQos             int
	UserName            string
	Password            string
	Keepalive           int // これ面倒くさいな
}

type Client struct {
	connection       io.ReadWriteCloser
	PublishCallback  func(string, []byte)
	Option           Option
	Closed           chan bool
	SubscribeHistory map[string]int
	Connected        bool
	Queue            chan codec.Message
	ProcessQueue        chan codec.Message
	InflightTable    *util.MessageTable
	ClearSession     bool
	OfflineQueue     []codec.Message
	MaxOfflineQueue  int
	Mutex            sync.RWMutex
	Balancer         *util.Balancer
	Errors           chan error
	Kicker           *time.Timer
}

func NewClient(opt Option) *Client {
	client := &Client{
		Option: Option{
			Magic:      []byte("MQTT"),
			Version:    4,
			// Memo: User have to set Identifier themselves
			Identifier: "momonga-mqtt",
			Keepalive:  10,
			TickerCallback: func(timer time.Time, c *Client) error {
				c.Ping()
				return nil
			},
		},
		Closed:           make(chan bool, 1),
		SubscribeHistory: make(map[string]int),
		Queue:            make(chan codec.Message, 8192),
		ProcessQueue:        make(chan codec.Message, 8192),
		InflightTable:    util.NewMessageTable(),
		ClearSession:     true,
		OfflineQueue:     make([]codec.Message, 0),
		MaxOfflineQueue: 1000,
		Mutex:            sync.RWMutex{},
		Balancer: &util.Balancer{
			// Writeは10req/secぐらいにおさえようず。クソ実装対策
			PerSec: 10,
		},
		Errors: make(chan error, 128),
	}

	client.InflightTable.SetOnFinish(func(id uint16, message codec.Message, opaque interface{}) {
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

	if len(opt.Magic) < 1 {
		opt.Magic = client.Option.Magic
	}
	if opt.Version == 0 {
		opt.Version = client.Option.Version
	}
	if len(opt.Identifier) < 1 {
		opt.Identifier = client.Option.Identifier
	}

	client.Option = opt
	return client
}

func (self *Client) setupKicker() {
	if self.Kicker != nil {
		self.Kicker.Stop()
	}

	self.Kicker = time.AfterFunc(time.Second * time.Duration(self.Option.Keepalive), func() {
			fmt.Printf("_")
			self.Ping()
			self.Kicker.Reset(time.Second * time.Duration(self.Option.Keepalive))
	})
}

func (self *Client) EnableClearSession() {
	self.ClearSession = true
}

func (self *Client) DisableClearSession() {
	self.ClearSession = false
}

func (self *Client) SetClearSession(bval bool) {
	self.ClearSession = bval
}

func (self *Client) Connect() error {
	var err error
	self.connection, err = self.Option.TransporterCallback()
	if err != nil {
		return err
	}

	msg := codec.NewConnectMessage()
	msg.Magic = self.Option.Magic
	msg.Version = uint8(self.Option.Version)
	msg.Identifier = self.Option.Identifier

	msg.CleanSession = self.ClearSession
	if msg.CleanSession {
		msg.Flag |= 0x02
	}

	if len(self.Option.WillTopic) > 0 {
		msg.Flag |= 0x04
		msg.Will = &codec.WillMessage{
			Topic:   self.Option.WillTopic,
			Message: string(self.Option.WillMessage),
			Retain:  self.Option.WillRetain,
			Qos:     uint8(self.Option.WillQos),
		}
	}

	if len(self.Option.UserName) > 0 {
		msg.Flag |= 0x80
		msg.UserName = self.Option.UserName
	}

	if len(self.Option.Password) > 0 {
		msg.Flag |= 0x40
		msg.Password = self.Option.Password
	}

	b, _ := codec.Encode(msg)
	self.connection.Write(b)
	self.setConnected(true)

	// TODO: このアホっぽい実装はあとでちゃんとなおす
	if len(self.OfflineQueue) > 0 {
		self.Mutex.Lock()
		var targets []codec.Message
		for len(self.OfflineQueue) > 0 {
			targets = append(targets, self.OfflineQueue[0])
			self.OfflineQueue = self.OfflineQueue[1:]
		}
		self.Mutex.Unlock()

		for i := 0; i < len(targets); i++ {
			self.Queue <- targets[i]
		}
	}

	return nil
}

func (self *Client) Ping() {
	if !self.IsAlived() {
		return
	}

	sb := codec.NewPingreqMessage()
	self.Queue <- sb
}

func (self *Client) Subscribe(topic string, QoS int) error {
	sb := codec.NewSubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{
		TopicFilter:  topic,
		RequestedQos: uint8(QoS),
	})

	id := self.InflightTable.NewId()
	sb.Identifier = id
	self.InflightTable.Register(id, sb, nil)
	self.Queue <- sb

	self.SubscribeHistory[topic] = QoS
	return nil
}

// TODO: あんまり明示的に出したくないなー、と思いつつ
func (self *Client) Loop() {
	go func() {
		for {
			select {
			// Write Queue
			case msg := <-self.Queue:
				b2, _ := codec.Encode(msg)

				if self.IsAlived() {
					self.Balancer.Execute(func() {
						_, err := self.connection.Write(b2)
						if err != nil {
							self.Errors <- err
						}
					})
				} else {
					self.Mutex.Lock()
					self.OfflineQueue = append(self.OfflineQueue, msg)
					if len(self.OfflineQueue) > self.MaxOfflineQueue {
						self.OfflineQueue = self.OfflineQueue[len(self.OfflineQueue)-self.MaxOfflineQueue:]
					}
					self.Mutex.Unlock()
				}
				break
			}

			if self.Kicker != nil {
				self.Kicker.Reset(time.Second * time.Duration(self.Option.Keepalive))
			}
		}
	}()

	go func() {
		for {
			// Read Queueっつーか受け取ったMessageをどうするかって
			select {
			case c := <-self.ProcessQueue:
				// TODO: ここらへんごりごり書きたくない
				switch c.GetType() {
				case codec.MESSAGE_TYPE_PUBLISH:
					// TODO: PublishでQos > 0の場合Retryできるように
					// まずは保存、そっから配るようにする

					p := c.(*codec.PublishMessage)
					if self.PublishCallback != nil {
						self.PublishCallback(p.TopicName, p.Payload)
					}

					switch p.QosLevel {
					case 1:
						ack := codec.NewPubackMessage()
						ack.Identifier = p.Identifier
						self.Queue <- ack
					case 2:
						ack := codec.NewPubrecMessage()
						ack.Identifier = p.Identifier
						self.Queue <- ack
					}
					break

				case codec.MESSAGE_TYPE_CONNACK:
					r := c.(*codec.ConnackMessage)
					if r.ReturnCode != 0 {
						self.Errors <- errors.New("Connection aborted")
						continue
					}
					break

				case codec.MESSAGE_TYPE_PUBACK:
					p := c.(*codec.PubackMessage)
					self.InflightTable.Unref(p.Identifier)
					break

				case codec.MESSAGE_TYPE_PUBREC:
					p := c.(*codec.PubrecMessage)
					ack := codec.NewPubrelMessage()
					ack.Identifier = p.Identifier
					self.Queue <- ack
					break

				case codec.MESSAGE_TYPE_PUBREL:
					// PUBRELを受けるということはReceiverとして受けるということ
					p := c.(*codec.PubrelMessage)
					ack := codec.NewPubcompMessage()
					ack.Identifier = p.Identifier
					self.Queue <- ack

					self.InflightTable.Unref(ack.Identifier) // Unackknowleged
					break

				case codec.MESSAGE_TYPE_PUBCOMP:
					// PUBCOMPを受けるということはSenderとして受けるということ。
					p := c.(*codec.PubcompMessage)
					self.InflightTable.Unref(p.Identifier)
					break

				case codec.MESSAGE_TYPE_PINGRESP:
					// TODO: Pingの内部カウント下げる
					break

				case codec.MESSAGE_TYPE_SUBACK:
					p := c.(*codec.SubackMessage)
					self.InflightTable.Remove(p.Identifier)

					break

				case codec.MESSAGE_TYPE_UNSUBACK:
					p := c.(*codec.UnsubackMessage)
					mm, err := self.InflightTable.Get(p.Identifier)

					// このエラーは無視していい
					if err == nil {
						if v, ok := mm.(*codec.UnsubscribeMessage); ok {
							delete(self.SubscribeHistory, v.TopicName)
						}
					}
					self.InflightTable.Remove(p.Identifier)
					break

				default:
					self.Errors <- fmt.Errorf("Unhandled message: %d", c.GetType())
				}

			case <-self.Closed:
				// Memo: まえは何かしたかったんだよ
				break
			}

			if self.Kicker != nil {
				self.Kicker.Reset(time.Second * time.Duration(self.Option.Keepalive))
			}
		}
	}()

	// 実際にReadするやつ
	for {
		if self.Connected {
			c, err := codec.ParseMessage(self.connection)
			if err != nil {
				if err == io.EOF {
					self.ForceClose()
					continue
				} else {
					self.Errors <- err
				}
			}

			self.ProcessQueue <- c
		} else {
			// TODO: implement exponential backoff
			time.Sleep(time.Duration(3) * time.Second)
			err := self.Connect()

			if err != nil {
				self.Errors <- err
			} else {
				for t, qos := range self.SubscribeHistory {
					self.Subscribe(t, qos)
				}
			}
		}
	}

	// エラーをたんたんと流してくれる人。どーゆーinterfaceにしよっかなー
	go func() {
		for {
			select {
				case e := <- self.Errors:
					fmt.Printf("error: %s\n", e)
			}
		}
	}()
}

func (self *Client) Publish(TopicName string, Payload []byte, QoSLevel int) {
	self.publishCommon(TopicName, Payload, QoSLevel, false, nil)
}

func (self *Client) PublishWait(TopicName string, Payload []byte, QoSLevel int) error {
	if QoSLevel == 0 {
		return errors.New("QoS should be greater than 0.")
	}

	b := make(chan bool, 1)
	self.publishCommon(TopicName, Payload, QoSLevel, false, b)
	<- b

	return nil
}

func (self *Client) PublishWithRetain(TopicName string, Payload []byte, QoSLevel int) {
	self.publishCommon(TopicName, Payload, QoSLevel, true, nil)
}

func (self *Client) publishCommon(TopicName string, Payload []byte, QosLevel int, retain bool, opaque interface{}) {
	sb := codec.NewPublishMessage()
	sb.TopicName = TopicName
	sb.Payload = Payload
	sb.QosLevel = QosLevel

	if retain {
		sb.Retain = 1
	}

	if QosLevel > 0 {
		id := self.InflightTable.NewId()
		sb.Identifier = id
		self.InflightTable.Register(id, sb, opaque)
	}

	self.Queue <- sb
}

func (self *Client) SetPublishCallback(callback func(string, []byte)) {
	self.PublishCallback = callback
}

func (self *Client) Unsubscribe(topic string) {
	sb := codec.NewUnsubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{TopicFilter: topic})
	id := self.InflightTable.NewId()
	sb.Identifier = id
	self.InflightTable.Register(id, sb, nil)

	self.Queue <- sb

	return
}

func (self *Client) setConnected(bval bool) {
	self.Mutex.Lock()
	self.Connected = bval

	if !self.Connected {
		if self.Option.Keepalive > 0 {
			self.Kicker.Stop()
		}
	} else {
		if self.Option.Keepalive > 0 {
			self.setupKicker()
		}
	}
	self.Mutex.Unlock()
}

func (self *Client) IsAlived() bool {
	self.Mutex.RLock()
	connected := self.Connected
	self.Mutex.RUnlock()

	return connected
}

func (self *Client) SetRequestPerSecondLimit(limit int) {
	self.Balancer.PerSec = limit
}

func (self *Client) Disconnect() {
	sb := codec.NewDisconnectMessage()
	self.Queue <- sb
}

func (self *Client) Close() {
	// お行儀よく死ぬ
	self.Disconnect()
	self.ForceClose()
}

func (self *Client) ForceClose() {
	// Disconnectを送らずに死ぬ
	self.setConnected(false)

	self.connection.Close()
	self.Closed <- true
}
