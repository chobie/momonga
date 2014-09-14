package client

import (
	"fmt"
	codec "github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"io"
	"sync"
	"time"
	"net"
	log "github.com/chobie/momonga/logger"
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
	term	chan bool
	wg  sync.WaitGroup
	offline []codec.Message
	mu sync.RWMutex
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
		Connection:   NewMyConnection(),
		CleanSession: true,
		Subscribed: make(map[string]int),
		Mutex:        sync.RWMutex{},
		Errors:       make(chan error, 128),
		term: make(chan bool, 1),
		offline: make([]codec.Message, 8192),
	}

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

	client.On("connack", func(result uint8) {
		if result == 0 {
			client.wg.Done()

			// reconnect
			for topic, qos := range client.Subscribed {
				client.Subscribe(topic, qos)
			}
		}
	})
	client.On("error", func(err error) {
			if err == io.EOF {
				client.Terminate()
				log.Debug("UNEXPECTED TERMINATE. retry")
				client.Connect()
			}
	})
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
		log.Error("Error; %s", err)
		time.AfterFunc(time.Second, func() {
				self.Connect()
		})
		return err
	}

	go self.Loop()
	self.Connection.SetMyConnection(connection)

	self.Connection.KeepLoop = true

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

	self.Connection.State = STATE_CONNECTING
	self.wg.Add(1)
	self.Connection.WriteMessageQueue(msg)
	log.Debug("CONNECT PROCESS")
	return nil
}

func (self *Client) WaitConnection() {
	if self.Connection.State == STATE_CONNECTING {
		self.wg.Wait()
	}
}

func (self *Client) Terminate() {
	self.mu.Lock()
	defer self.mu.Unlock()

	if self.Connection.State != STATE_CLOSED {
		self.term <- true
		self.Connection.Close()
	}
}

func (self *Client) Loop() {
	// TODO: consider interface. for now, just print it.
	for {
		select {
		case e := <-self.Errors:
			log.Error("ERROR: %s\n", e)
		case <- self.term:
			// Close
			return
		default:
		// TODO: move this function to connect (実際にReadするやつ)
			switch self.Connection.State {
			case STATE_CONNECTED, STATE_CONNECTING:
				_, err := self.Connection.ParseMessage()
				if err != nil {
					if nerr, ok := err.(net.Error); ok {
						if nerr.Timeout() {
							// Closing connection.
							self.Terminate()
						} else if nerr.Temporary() {
							//log.Info("Temporary Error: %s", err)
							self.Terminate()
						} else {
							fmt.Printf("damepo")
						}
					} else if err == io.EOF {
						self.Terminate()
					} else if err.Error() == "use of closed network connection" {
						self.Terminate()
					} else {
						self.Errors <- err
					}
				}
			case STATE_CLOSED:
				// TODO: implement exponential backoff
				log.Debug("Sleep")
				time.Sleep(time.Second * 3)

				err := self.Connect()
				if err != nil {
					self.Errors <- err
				}
			default:
				log.Debug("SLEEP(unknown)")
				time.Sleep(time.Second)
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
