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
	Keepalive           int
}

type Client struct {
	connection       io.ReadWriteCloser
	PublishCallback  func(string, []byte)
	Option           Option
	Closed           chan bool
	SubscribeHistory map[string]int
	Connected        bool
	Queue            chan codec.Message
	ReadQueue        chan codec.Message
	InflightTable    *util.MessageTable
	ClearSession     bool
	OfflineQueue     []codec.Message
	MaxOfflineQueue  int
	Mutex            sync.Mutex
	Balancer         *util.Balancer
	Errors           chan error
}

func NewClient(opt Option) *Client {
	client := &Client{
		Option: Option{
			Magic:      []byte("MQTT"),
			Version:    4,
			Identifier: "mqtt-chobie",
			Keepalive:  10,
			TickerCallback: func(timer time.Time, c *Client) error {
				c.Ping()
				return nil
			},
		},
		Closed:           make(chan bool, 1),
		SubscribeHistory: make(map[string]int),
		Queue:            make(chan codec.Message, 8192),
		ReadQueue:        make(chan codec.Message, 8192),
		InflightTable:    util.NewMessageTable(),
		ClearSession:     true,
		OfflineQueue:     make([]codec.Message, 0),
		MaxOfflineQueue: 1000,
		Mutex:            sync.Mutex{},
		Balancer: &util.Balancer{
			// Writeは10req/secぐらいにおさえようず
			PerSec: 10,
		},
		Errors: make(chan error, 128),
	}

	if len(opt.Magic) < 1 {
		opt.Magic = client.Option.Magic
	}
	if opt.Version == 0 {
		opt.Version = client.Option.Version
	}
	if len(opt.Identifier) < 1 {
		opt.Identifier = client.Option.Identifier
	}

	// TODO: TickerはPrivateにしておきたいかな
	// 最後の行動からKeepalive秒たったときにFireするからTickerではない。
	if opt.Keepalive > 0 {
		opt.Ticker = time.NewTicker((time.Duration)(opt.Keepalive) * time.Second)
		if opt.TickerCallback == nil {
			opt.TickerCallback = client.Option.TickerCallback
		}
	}
	client.Option = opt
	return client
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
	self.Mutex.Lock()
	self.Connected = true
	self.Mutex.Unlock()

	// TODO: (´・ω・｀)
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
	self.Mutex.Lock()
	connected := self.Connected
	self.Mutex.Unlock()
	if !connected {
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

func (self *Client) Loop() {
	go func() {
		for {
			select {
			// Write Queue
			case msg := <-self.Queue:
				// TODO: PublishでQos > 0の場合Retryできるように
				// というか、まずは保存、そっから配る
				b2, _ := codec.Encode(msg)

				self.Mutex.Lock()
				connected := self.Connected
				self.Mutex.Unlock()

				if connected {
					self.Balancer.Execute(func() {
						_, err := self.connection.Write(b2)
						if err != nil {
							self.Errors <- err
						}
					})
				} else {
					self.OfflineQueue = append(self.OfflineQueue, msg)
					if len(self.OfflineQueue) > self.MaxOfflineQueue {
						self.OfflineQueue = self.OfflineQueue[len(self.OfflineQueue)-self.MaxOfflineQueue:]
					}
				}
				break
			}
		}
	}()

	go func() {
		for {
			// Read Queue
			select {
			case c := <-self.ReadQueue:
				switch c.GetType() {
				case codec.MESSAGE_TYPE_PUBLISH:
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
						return
					}

					// TODO: これここでやるべきかなぁ
					go func(closed chan bool) {
						select {
						case <-closed:
							fmt.Printf("damepo2")
							return
						default:
							if self.Option.Ticker != nil {
								for t := range self.Option.Ticker.C {
									self.Option.TickerCallback(t, self)
								}
							}
						}
					}(self.Closed)
					break
				case codec.MESSAGE_TYPE_PUBACK:
					//p := c.(*codec.PubackMessage)
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

					self.InflightTable.Remove(ack.Identifier) // Unackknowleged
					break
				case codec.MESSAGE_TYPE_PUBCOMP:
					// PUBCOMPを受けるということはSenderとして受けるということ。
					p := c.(*codec.PubcompMessage)
					self.InflightTable.Remove(p.Identifier)
					break
				case codec.MESSAGE_TYPE_PINGRESP:
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

			self.ReadQueue <- c
		} else {
			// Exponential backoff
			time.Sleep(time.Duration(3) * time.Second)
			err := self.Connect()

			if err != nil {
				self.Errors <- err
			} else {
				for t, qos := range self.SubscribeHistory {
					// TODO: QoSをちゃんと指定する
					self.Subscribe(t, qos)
				}
			}
		}
	}

	// エラーをたんたんと流してくれる人
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
	self.publishCommon(TopicName, Payload, QoSLevel, false)
}

func (self *Client) PublishWait(TopicName string, Payload []byte, QoSLevel int) {
	// ってあるとつかいやすい？
	//self.publishCommon(TopicName, Payload, QoSLevel, false)
}

func (self *Client) PublishWithRetain(TopicName string, Payload []byte, QoSLevel int) {
	self.publishCommon(TopicName, Payload, QoSLevel, true)
}

func (self *Client) publishCommon(TopicName string, Payload []byte, QosLevel int, retain bool) {
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
		self.InflightTable.Register(id, sb, nil)
	}

	self.Queue <- sb
}

func (self *Client) SetPublishCallback(callback func(string, []byte)) {
	self.PublishCallback = callback
}

func (self *Client) Unsubscribe(topic string) error {
	sb := codec.NewUnsubscribeMessage()
	sb.Payload = append(sb.Payload, codec.SubscribePayload{TopicFilter: topic})
	id := self.InflightTable.NewId()
	sb.Identifier = id
	self.InflightTable.Register(id, sb, nil)

	self.Queue <- sb
	return nil
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
	self.Mutex.Lock()
	self.Connected = false
	self.Mutex.Unlock()

	self.connection.Close()
	self.Closed <- true
}
