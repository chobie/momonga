package server

import (
	"bytes"
	"fmt"
	"github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"io"
	"net"
	"time"
)

// TODO: use client/connection. it's easy to extend.
// Anonymous(?) connection
type TcpConnection struct {
	Socket            net.Conn
	Address           net.Addr
	Connected         time.Time
	State             State
	WillMessage       *mqtt.WillMessage
	OutGoingTable     *util.MessageTable
	SubscribedTopics  map[string]int
	WriteQueue        chan mqtt.Message
	WriteQueueFlag    chan bool
	Last              time.Time
	KeepaliveInterval int
	ClearSession      bool
	Qlobber           *util.Qlobber
}
/*
OKだとおもうの
func (self *TcpConnection) GetState() State {
func (self *TcpConnection) SetState(s State) {
func (self *TcpConnection) ResetState() {
func (self *TcpConnection) SetKeepaliveInterval(interval int) {

func (self *TcpConnection) DisableClearSession() {
func (self *TcpConnection) ShouldClearSession() bool {
	ClearSession -> CleanSession

上位に持たせたい?
func (self *TcpConnection) SetWillMessage(will mqtt.WillMessage) {
func (self *TcpConnection) GetWillMessage() *mqtt.WillMessage {
func (self *TcpConnection) HasWillMessage() bool {
func (self *TcpConnection) AppendSubscribedTopic(topic string, qos int) {
func (self *TcpConnection) RemoveSubscribedTopic(topic string) {

func (self *TcpConnection) GetSubscribedTopicQos(topic string) int {
func (self *TcpConnection) GetSubscribedTopics() map[string]int {

未処理
func (self *TcpConnection) GetOutGoingTable() *util.MessageTable {
	GetInflightTable

func (self *TcpConnection) IsAlived() bool {
func (self *TcpConnection) GetId() string {
func (self *TcpConnection) Close() {
func NewTcpConnection(socket net.Conn, retry chan *Retryable) Connection {

-- 基本は新しい方を尊重させたい
func (self *Connection) SetConnection(c io.ReadWriteCloser) {
func (self *Connection) Subscribe(topic string, QoS int) error {
func (self *Connection) setupKicker() {
func (self *Connection) Ping() {
func (self *Connection) On(event string, callback interface{}, args ...bool) error {
func (self *Connection) GetConnectionState() ConnectionState {
func (self *Connection) Publish(TopicName string, Payload []byte, QosLevel int, retain bool, opaque interface{}) {
func (self *Connection) HasConnection() bool {
func (self *Connection) ParseMessage() (codec.Message, error) {
	func (self *TcpConnection) ReadMessage() (mqtt.Message, error) {からかえる

func (self *Connection) Read(p []byte) (int, error) {
func (self *Connection) Write(b []byte) (int, error) {
	func (self *TcpConnection) WriteMessageQueue(request mqtt.Message) {
	func (self *TcpConnection) WriteMessage(msg mqtt.Message) error {
	func (self *TcpConnection) Write(reader *bytes.Reader) error {
	Writeだけであとはチャンネルに直接渡そう?

func (self *Connection) Close() error {
func (self *Connection) Disconnect() {
func (self *Connection) Unsubscribe(topic string) {
func (self *Connection) invalidateTimer() {
*/

func (self *TcpConnection) SetWillMessage(will mqtt.WillMessage) {
	self.WillMessage = &will
}

func (self *TcpConnection) GetWillMessage() *mqtt.WillMessage {
	return self.WillMessage
}

func (self *TcpConnection) HasWillMessage() bool {
	if self.WillMessage == nil {
		return false
	}
	return true
}

func (self *TcpConnection) GetState() State {
	return self.State
}

func (self *TcpConnection) SetState(s State) {
	self.State = s
}

func (self *TcpConnection) GetOutGoingTable() *util.MessageTable {
	return self.OutGoingTable
}

func (self *TcpConnection) SetKeepaliveInterval(interval int) {
	self.KeepaliveInterval = interval
}

func (self *TcpConnection) ResetState() {
}

func NewTcpConnection(socket net.Conn, retry chan *Retryable) Connection {
	conn := &TcpConnection{
		Socket:            socket,
		Address:           socket.RemoteAddr(),
		Connected:         time.Now(),
		OutGoingTable:     util.NewMessageTable(),
		WriteQueue:        make(chan mqtt.Message, 8192),
		WriteQueueFlag:    make(chan bool, 1),
		KeepaliveInterval: 0,
		Last:              time.Now(),
		ClearSession:      true,
		SubscribedTopics:  make(map[string]int),
		Qlobber:           util.NewQlobber(),
	}

	go func() {
		for {
			select {
			case m := <-conn.WriteQueue:
				data, err := mqtt.Encode(m)
				if err != nil {
					// まずここでエラーはでないだろう
					panic(fmt.Sprintf("Unexpected encode error: %s", err))
					continue
				}

				err = conn.Write(bytes.NewReader(data))
				if err != nil {
					// Qos1, Qos2はEngineに戻さないといかんけど配送先とか考えるととても面倒くさい。
					if v, ok := m.(*mqtt.PublishMessage); ok {
						switch v.QosLevel {
						case 1, 2:
							// TODO: これはこれで違うんだよな。とりあえずおいているだけ
							retry <- &Retryable{
								Id:      conn.GetId(),
								Payload: m,
							}
						}
					}
					continue
				}
			case <-conn.WriteQueueFlag:
				// MEMO: Terminate goroutine
				return
			}
		}
	}()

	return conn
}

func (self *TcpConnection) GetSubscribedTopicQos(topic string) int {
	v := self.Qlobber.Match(topic)
	if len(v) > 0 {
		if r, ok := v[0].(int); ok {
			return r
		}
	}
	return -1
	//	if qos, ok := self.SubscribedTopics[topic]; ok {
	//		return qos
	//	}
	//	return -1
}

func (self *TcpConnection) GetSubscribedTopics() map[string]int {
	return self.SubscribedTopics
}

func (self *TcpConnection) AppendSubscribedTopic(topic string, qos int) {
	self.SubscribedTopics[topic] = qos
	self.Qlobber.Add(topic, qos)
}

func (self *TcpConnection) RemoveSubscribedTopic(topic string) {
	self.Qlobber.Remove(topic, nil)

	if _, ok := self.SubscribedTopics[topic]; ok {
		delete(self.SubscribedTopics, topic)
	}
}

func (self *TcpConnection) GetAddress() net.Addr {
	return self.Address
}

func (self *TcpConnection) IsAlived() bool {
	return self.Socket != nil
}

//func (self *TcpConnection) readBuffer() (mqtt.Message, error) {
//	return mqtt.ParseMessage(self.Socket)
//}
//
func (self *TcpConnection) ReadMessage() (mqtt.Message, error) {
	if self.KeepaliveInterval > 0 {
		self.Socket.SetReadDeadline(self.Last.Add(time.Duration(int(float64(self.KeepaliveInterval)*2)) * time.Second))
	}

	result, err := mqtt.ParseMessage(self.Socket, 8192)
	self.Last = time.Now()
	return result, err
}

func (self *TcpConnection) WriteMessageQueue(request mqtt.Message) {
	self.WriteQueue <- request
}

func (self *TcpConnection) WriteMessage(msg mqtt.Message) error {
	data, err := mqtt.Encode(msg)
	if err != nil {
		return err
	}

	result := self.Write(bytes.NewReader(data))
	self.Last = time.Now()
	return result
}

func (self *TcpConnection) Write(reader *bytes.Reader) error {
	var err error

	// TODO: これどっしよっかなー。
	//conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = io.Copy(self.Socket, reader)
	if err != nil {
		return err
	}
	return nil
}

func (self *TcpConnection) GetId() string {
	return self.Socket.RemoteAddr().String()
}

func (self *TcpConnection) Close() {
	//	log.Debug("[TcpConnection Closed]")
	self.Socket.Close()
	self.WriteQueueFlag <- true

	// TODO
	//self.Server.RemoveConnection(self)
}

func (self *TcpConnection) DisableClearSession() {
	self.ClearSession = false
}

func (self *TcpConnection) ShouldClearSession() bool {
	return self.ClearSession
}
