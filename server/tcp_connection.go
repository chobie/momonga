package server

import (
	"net"
	"bytes"
	"io"
	"time"
	"github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"fmt"
)


// Anonymous(?) connection
type TcpConnection struct {
	Socket net.Conn
	Address net.Addr
	Buffer *Buffer
	Connected time.Time
	State State
	yield func(conn Connection, time time.Time)
	WillMessage *mqtt.WillMessage
	OutGoingTable *util.MessageTable
	SubscribedTopics []string
	WriteQueue chan mqtt.Message
	WriteQueueFlag chan bool
	Last time.Time
	KeepaliveInterval int
	ClearSession bool
}

func (self *TcpConnection) SetWillMessage(will mqtt.WillMessage) {
	self.WillMessage = &will
}

func (self *TcpConnection) GetWillMessage() *mqtt.WillMessage {
	return self.WillMessage
}

func (self *TcpConnection)HasWillMessage() bool {
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
	self.Buffer.ReadBuffer.Reset()
	self.Buffer.WriteBuffer.Reset()
}

func NewTcpConnection(socket net.Conn, server Server, retry chan *Retryable, yield func(conn Connection, time time.Time)) Connection {
	conn := &TcpConnection{
		Socket: socket,
		Address: socket.RemoteAddr(),
		Connected: time.Now(),
		yield: yield,
		OutGoingTable: util.NewMessageTable(),
		WriteQueue: make(chan mqtt.Message, 8192),
		WriteQueueFlag: make(chan bool, 1),
		KeepaliveInterval: 0,
		Last: time.Now(),
		ClearSession: true,
	}

	readMessage := make([]byte, 0, MAX_REQUEST_SIZE)
	writeMessage := make([]byte, 0, MAX_REQUEST_SIZE)

	buffer := &Buffer{}
	buffer.ReadBuffer = bytes.NewBuffer(readMessage)
	buffer.WriteBuffer = bytes.NewBuffer(writeMessage)
	conn.Buffer = buffer

	go func() {
		for {
			select {
			case m := <- conn.WriteQueue:
				data, err :=  mqtt.Encode(m)
				if err != nil {
					// まずここでエラーはでないだろう
					panic(fmt.Sprintf("Unexpected encode error: %s", err))
					continue
				}

				fmt.Printf("CLIENT WRITE: %+v\n", m)
				err = conn.Write(bytes.NewReader(data))
				if err != nil {
					fmt.Printf("CLIENT WRITE ERROR\n")
					// Qos1, Qos2はEngineに戻さないといかんけど配送先とか考えるととても面倒くさい。
					if v, ok := m.(*mqtt.PublishMessage); ok {
						switch (v.QosLevel) {
						case 1, 2:
							// TODO: これはこれで違うんだよな。とりあえずおいているだけ
							retry <- &Retryable{
								Id: conn.GetId(),
								Payload: m,
							}
						}
					}
					continue
				} else {
					fmt.Printf("CLIENT WRITE SUCCEEDED\n")
				}
			case <- conn.WriteQueueFlag:
				// TODO: なにがしたかったんだっけか。ああ、殺したかったんだ
				return
			}
		}
	}()
	return conn
}

func (self *TcpConnection) GetSubscribedTopics() []string {
	return self.SubscribedTopics
}

func (self *TcpConnection) AppendSubscribedTopic(topic string) {
	self.SubscribedTopics = append(self.SubscribedTopics, topic)
}

func (self *TcpConnection) RemoveSubscribedTopic(topic string) {
	offset := -1
	for i, v := range self.SubscribedTopics {
		if v == topic {
			offset = i
			break
		}
	}

	self.SubscribedTopics = append(self.SubscribedTopics[:offset], self.SubscribedTopics[:offset+1]...)
}

func (self *TcpConnection) GetSocket() net.Conn {
	return self.Socket
}

func (self *TcpConnection) SetSocket(conn net.Conn) {
	self.Socket = conn
}

func (self *TcpConnection) ClearBuffer() {
	self.Buffer.ClearBuffer()
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
		self.Socket.SetReadDeadline(self.Last.Add(time.Duration(int(float64(self.KeepaliveInterval) * 2)) * time.Second))
	}
	result, err := mqtt.ParseMessage(self.Socket)
	self.Last = time.Now()
	return result, err
}

func (self *TcpConnection) WriteMessageQueue(request mqtt.Message) {
	self.WriteQueue <- request
}

func (self *TcpConnection) WriteMessage(msg mqtt.Message) (error){
	data, err :=  mqtt.Encode(msg)
	if err != nil {
		return err
	}

	result := self.Write(bytes.NewReader(data))
	self.Last = time.Now()
	return result
}

func (self *TcpConnection) Write(reader *bytes.Reader) (error){
	var err error

	self.Buffer.WriteBuffer.Reset()
	defer func() {
		self.Buffer.WriteBuffer.Reset();
	}()

	// TODO: これどっしよっかなー。
	//conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = io.Copy(self.Socket, reader);
	if err != nil {
		return err
	}
	return nil
}

func (self *TcpConnection) GetId() string{
	return self.Socket.RemoteAddr().String()
}

func (self *TcpConnection) Close() {
//	log.Debug("[TcpConnection Closed]")
	self.Socket.Close()

	// TODO
	//self.Server.RemoveConnection(self)
}

func (self *TcpConnection) DisableClearSession() {
	self.ClearSession = false
}

func (self *TcpConnection) ShouldClearSession() bool {
	return self.ClearSession
}
