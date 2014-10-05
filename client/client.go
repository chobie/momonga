// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package client

import (
	"fmt"
	. "github.com/chobie/momonga/common"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	//	"io"
	"net"
	"sync"
	"time"
)

type Option struct {
	TransporterCallback func() (net.Conn, error)
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
	Keepalive           int // THIS IS REALLY TROBULESOME
	Logger              log.Logger
}

type Client struct {
	Connection      *MyConnection
	PublishCallback func(string, []byte)
	Option          Option
	CleanSession    bool
	Subscribed      map[string]int
	Mutex           sync.RWMutex
	Errors          chan error
	Kicker          *time.Timer
	wg              sync.WaitGroup
	offline         []codec.Message
	mu              sync.RWMutex
	once            sync.Once
	term            chan bool
	logger          log.Logger
	reconnect       bool
	count           int
}

func NewClient(opt Option) *Client {
	client := &Client{
		Option: Option{
			Magic:   []byte("MQTT"),
			Version: 4,
			// Memo: User have to set PacketIdentifier themselves
			Identifier: "momongacli",
			Keepalive:  10,
		},
		CleanSession: true,
		Subscribed:   make(map[string]int),
		Mutex:        sync.RWMutex{},
		Errors:       make(chan error, 128),
		offline:      make([]codec.Message, 8192),
		term:         make(chan bool, 256),
	}

	if opt.Logger != nil {
		client.logger = opt.Logger
	} else {
		client.logger = log.Global
	}

	client.Connection = NewMyConnection(&MyConfig{
		QueueSize:        8192,
		OfflineQueueSize: 1024,
		Keepalive:        60,
		WritePerSec:      10,
		Logger:           client.logger,
	})

	if len(opt.Magic) < 1 {
		opt.Magic = client.Option.Magic
	}
	if opt.Version == 0 {
		opt.Version = client.Option.Version
	}
	if len(opt.Identifier) < 1 {
		// generate random string
		suffix := util.GenerateId(23 - (len(client.Option.Identifier) + 1))
		opt.Identifier = fmt.Sprintf("%s-%s", client.Option.Identifier, suffix)
	}

	// TODO: should provide defaultOption function.
	client.Option = opt
	client.Connection.Keepalive = opt.Keepalive

	// required for client
	client.Connection.KeepLoop = true

	client.On("connack", func(result uint8) {
		if result == 0 {
			client.wg.Done()

			// reconnect
			if client.reconnect {
				for topic, qos := range client.Subscribed {
					client.Subscribe(topic, qos)
				}
			}
		}
	})

	client.On("error", func(err error) {
		if client.Connection.State == STATE_CONNECTED {
			client.Connection.Close()
			time.Sleep(time.Second)
			client.Connect()
		}
	}, true)

	go client.Loop()
	return client
}

func (self *Client) Connect() error {
	self.mu.Lock()
	defer self.mu.Unlock()

	if self.Connection.State == STATE_CONNECTING || self.Connection.State == STATE_CONNECTED {
		// 接続中, 試行中なのでなにもしない
		return nil
	}

	connection, err := self.Option.TransporterCallback()
	if err != nil {
		return err
	}

	if v, ok := connection.(*net.TCPConn); ok {
		// should enable keepalive.
		v.SetKeepAlive(true)
		v.SetNoDelay(true)
	}

	self.once = sync.Once{}
	self.Connection.SetMyConnection(connection)
	self.wg.Add(1)

	// send a connect message to MQTT Server
	msg := codec.NewConnectMessage()
	msg.Magic = self.Option.Magic
	msg.Version = uint8(self.Option.Version)
	msg.Identifier = self.Option.Identifier
	msg.CleanSession = self.CleanSession
	msg.KeepAlive = uint16(self.Option.Keepalive)

	if len(self.Option.WillTopic) > 0 {
		msg.Will = &codec.WillMessage{
			Topic:   self.Option.WillTopic,
			Message: string(self.Option.WillMessage),
			Retain:  self.Option.WillRetain,
			Qos:     uint8(self.Option.WillQos),
		}
	}

	if len(self.Option.UserName) > 0 {
		msg.UserName = []byte(self.Option.UserName)
	}

	if len(self.Option.Password) > 0 {
		msg.Password = []byte(self.Option.Password)
	}

	self.Connection.SetState(STATE_CONNECTING)
	self.Connection.WriteMessageQueue(msg)
	time.Sleep(time.Second)
	self.count += 1
	if self.count > 1 {
		self.reconnect = true
	}
	return nil
}

func (self *Client) WaitConnection() {
	if self.Connection.GetState() == STATE_CONNECTING {
		self.wg.Wait()
	}
}

// terminate means loop terminate
func (self *Client) Terminate() {
	self.mu.Lock()
	defer self.mu.Unlock()

	self.term <- true
}

func (self *Client) Loop() {
	// read loop
	// TODO: consider interface. for now, just print it.
	for {
		select {
		case <-self.term:
			return
		default:
			// TODO: move this function to connect (実際にReadするやつ)
			switch self.Connection.GetState() {
			case STATE_CONNECTED, STATE_CONNECTING:
				// ここでのエラーハンドリングは重複してしまうのでやらない。On["error"]に任せる
				_, e := self.Connection.ParseMessage()
				if e != nil {
					time.Sleep(time.Second)
				}
			case STATE_CLOSED:
				// TODO: implement exponential backoff
				time.Sleep(time.Second)
				self.Connect()
			default:
				time.Sleep(time.Second)
				self.Connect()
			}
		}
	}
}

func (self *Client) On(event string, callback interface{}, args ...bool) error {
	return self.Connection.On(event, callback, args...)
}

func (self *Client) Publish(TopicName string, Payload []byte, QoSLevel int) {
	self.publishCommon(TopicName, Payload, QoSLevel, false, nil)
}

func (self *Client) publishCommon(TopicName string, Payload []byte, QosLevel int, retain bool, opaque interface{}) {
	self.Connection.Publish(TopicName, Payload, QosLevel, retain, opaque)
}

func (self *Client) PublishWait(TopicName string, Payload []byte, QoSLevel int) error {
	if QoSLevel == 0 {
		return fmt.Errorf("QoS should be greater than 0.")
	}

	b := make(chan bool, 1)
	self.publishCommon(TopicName, Payload, QoSLevel, false, b)
	<-b
	close(b)

	return nil
}

func (self *Client) PublishWithRetain(TopicName string, Payload []byte, QoSLevel int) {
	self.publishCommon(TopicName, Payload, QoSLevel, true, nil)
}

func (self *Client) Subscribe(topic string, QoS int) error {
	self.Subscribed[topic] = QoS

	return self.Connection.Subscribe(topic, QoS)
}

func (self *Client) Unsubscribe(topic string) {
	self.Connection.Unsubscribe(topic)
}

func (self *Client) Disconnect() {
	self.Connection.Disconnect()
}

func (self *Client) SetRequestPerSecondLimit(limit int) {
	self.Connection.SetRequestPerSecondLimit(limit)
}
