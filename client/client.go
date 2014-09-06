package client

import (
	"errors"
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"io"
	"sync"
	"time"
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
	Keepalive           int // THIS IS REALLY TROBULESOME
}

type Client struct {
	Connection      *Connection
	PublishCallback func(string, []byte)
	Option          Option
	CleanSession    bool
	Mutex           sync.RWMutex
	Errors          chan error
	Kicker          *time.Timer
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
		Connection: NewConnection(),
		CleanSession:    true,
		Mutex:           sync.RWMutex{},
		Errors: make(chan error, 128),
	}

	if len(opt.Magic) < 1 {
		opt.Magic = client.Option.Magic
	}
	if opt.Version == 0 {
		opt.Version = client.Option.Version
	}
	if len(opt.Identifier) < 1 {
		// generate random string
		suffix := util.GenerateId(23 - (len(client.Option.Identifier)+1))
		opt.Identifier = fmt.Sprintf("%s-%s", client.Option.Identifier, suffix)
	}

	// TODO: should provide defaultOption function.
	client.Option = opt
	client.Connection.Keepalive = opt.Keepalive
	return client
}

func (self *Client) EnableCleanSession() {
	self.CleanSession = true
}

func (self *Client) DisableCleanSession() {
	self.CleanSession = false
}

func (self *Client) SetCleanSession(bval bool) {
	self.CleanSession = bval
}

func (self *Client) getConnectionState() ConnectionState {
	if self.Connection == nil {
		return CONNECTION_STATE_CLOSED
	}
	return self.Connection.GetConnectionState()
}

func (self *Client) Connect() error {
	if self.getConnectionState() > CONNECTION_STATE_CLOSED {
		return nil
	}
	connection, err := self.Option.TransporterCallback()
	if err != nil {
		return err
	}
	self.Connection.SetConnection(connection)

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
		msg.UserName = self.Option.UserName
	}

	if len(self.Option.Password) > 0 {
		msg.Password = self.Option.Password
	}

	self.Connection.WriteMessageQueue(msg)
	return nil
}

func (self *Client) Subscribe(topic string, QoS int) error {
	if self.Connection == nil {
		return errors.New("error")
	}
	return self.Connection.Subscribe(topic, QoS)
}

func (self *Client) On(event string, callback interface{}, args ...bool) error {
	return self.Connection.On(event, callback, args...)
}

// TODO: What is the good API?
func (self *Client) Loop() {

	// TODO: move this function to connect (実際にReadするやつ)
	for {
		switch (self.getConnectionState()) {
		case CONNECTION_STATE_CONNECTED:
			_, err := self.Connection.ParseMessage()
			if err != nil {
				if err == io.EOF {
					self.ForceClose()
					continue
				} else {
					self.Errors <- err
				}
			}
		case CONNECTION_STATE_CLOSED:
			// TODO: implement exponential backoff
			time.Sleep(time.Second * 3)

			err := self.Connect()
			if err != nil {
				self.Errors <- err
			}
		}
	}

	// TODO: consider interface. for now, just print it.
	go func() {
		for {
			select {
			case e := <-self.Errors:
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
	<-b

	return nil
}

func (self *Client) PublishWithRetain(TopicName string, Payload []byte, QoSLevel int) {
	self.publishCommon(TopicName, Payload, QoSLevel, true, nil)
}

func (self *Client) publishCommon(TopicName string, Payload []byte, QosLevel int, retain bool, opaque interface{}) {
	self.Connection.Publish(TopicName, Payload, QosLevel, retain, opaque)
}

func (self *Client) SetPublishCallback(callback func(string, []byte)) {
	self.PublishCallback = callback
}

func (self *Client) Unsubscribe(topic string) {
	self.Connection.Unsubscribe(topic)
}

func (self *Client) SetRequestPerSecondLimit(limit int) {
	self.Connection.Balancer.PerSec = limit
}

func (self *Client) Disconnect() {
	self.Connection.Disconnect()
}

func (self *Client) Close() {
	self.Disconnect()
	self.ForceClose()
}

func (self *Client) ForceClose() {
	// quit without disconnect message
	self.Connection.Close()
}
