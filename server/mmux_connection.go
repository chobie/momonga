package server

import (
	"bytes"
	"github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"net"
)

// MQTT Multiplexer Connection
type MmuxConnection struct {
	// Primary
	PrimaryConnection Connection
	OfflineQueue      []mqtt.Message
	Connections       map[string]Connection
	MaxOfflineQueue   int
	Identifier        string
	CleanSession      bool
	OutGoingTable     *util.MessageTable
}

func NewMmuxConnection() *MmuxConnection {
	conn := &MmuxConnection{
		OutGoingTable: util.NewMessageTable(),
		Connections:   map[string]Connection{},
	}

	return conn
}

func (self *MmuxConnection) GetId() string {
	return self.Identifier
}

func (self *MmuxConnection) Attach(conn Connection) {
	if len(self.Connections) == 0 {
		self.PrimaryConnection = conn
		self.CleanSession = conn.ShouldClearSession()

		if conn.ShouldClearSession() {
			self.OfflineQueue = self.OfflineQueue[:0]
		}

		if len(self.OfflineQueue) > 0 {
			//fmt.Printf("Process Offline Queue: Playback: %d", len(self.OfflineQueue))
			for i := 0; i < len(self.OfflineQueue); i++ {
				self.WriteMessageQueue(self.OfflineQueue[i])
			}
			self.OfflineQueue = self.OfflineQueue[:0]
		}
	}

	self.Connections[conn.GetId()] = conn
}

func (self *MmuxConnection) Detach(conn Connection) {
	if self.PrimaryConnection == conn {
		self.PrimaryConnection = nil
	}

	delete(self.Connections, conn.GetId())

	if len(self.Connections) == 0 {
		self.PrimaryConnection = nil
	} else {
		for _, v := range self.Connections {
			self.PrimaryConnection = v
			break
		}
	}
}

func (self *MmuxConnection) WriteMessage(request mqtt.Message) error {
	if self.PrimaryConnection == nil {
		return nil
	}
	return self.PrimaryConnection.WriteMessage(request)
}

func (self *MmuxConnection) WriteMessageQueue(request mqtt.Message) {
	if self.PrimaryConnection == nil {
		self.OfflineQueue = append(self.OfflineQueue, request)
		return
	}

	self.PrimaryConnection.WriteMessageQueue(request)
}
func (self *MmuxConnection) Close() {
	if self.PrimaryConnection == nil {
		return
	}
	self.PrimaryConnection.Close()
}
func (self *MmuxConnection) SetState(state State) {
	if self.PrimaryConnection == nil {
		return
	}
	self.PrimaryConnection.SetState(state)
}

func (self *MmuxConnection) GetState() State {
	if self.PrimaryConnection == nil {
		return STATE_DETACHED
	}

	return self.PrimaryConnection.GetState()
}

func (self *MmuxConnection) ResetState() {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.ResetState()
}

func (self *MmuxConnection) ReadMessage() (mqtt.Message, error) {
	if self.PrimaryConnection == nil {
		return nil, nil
	}

	return self.PrimaryConnection.ReadMessage()
}
func (self *MmuxConnection) GetSocket() net.Conn {
	if self.PrimaryConnection == nil {
		return nil
	}

	return self.PrimaryConnection.GetSocket()
}

func (self *MmuxConnection) SetSocket(sock net.Conn) {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.SetSocket(sock)

}
func (self *MmuxConnection) ClearBuffer() {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.ClearBuffer()
}
func (self *MmuxConnection) GetAddress() net.Addr {
	if self.PrimaryConnection == nil {
		return nil
	}

	return self.PrimaryConnection.GetAddress()

}
func (self *MmuxConnection) Write(reader *bytes.Reader) error {
	if self.PrimaryConnection == nil {
		return nil
	}

	return self.PrimaryConnection.Write(reader)
}

func (self *MmuxConnection) IsAlived() bool {
	if self.PrimaryConnection == nil {
		return true
	}

	return self.PrimaryConnection.IsAlived()
}

func (self *MmuxConnection) SetWillMessage(msg mqtt.WillMessage) {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.SetWillMessage(msg)
}

func (self *MmuxConnection) GetWillMessage() *mqtt.WillMessage {
	if self.PrimaryConnection == nil {
		return nil
	}
	return self.PrimaryConnection.GetWillMessage()
}

func (self *MmuxConnection) HasWillMessage() bool {
	if self.PrimaryConnection == nil {
		return false
	}
	return self.PrimaryConnection.HasWillMessage()

}

func (self *MmuxConnection) GetOutGoingTable() *util.MessageTable {
	return self.OutGoingTable
	//	if self.PrimaryConnection == nil {
	//		return nil
	//	}
	//
	//	return self.PrimaryConnection.GetOutGoingTable()
}

func (self *MmuxConnection) GetSubscribedTopicQos(topic string) int {
	if self.PrimaryConnection == nil {
		return 0
	}

	return self.PrimaryConnection.GetSubscribedTopicQos(topic)
}

func (self *MmuxConnection) GetSubscribedTopics() map[string]int {
	if self.PrimaryConnection == nil {
		return nil
	}

	return self.PrimaryConnection.GetSubscribedTopics()
}

func (self *MmuxConnection) AppendSubscribedTopic(topic string, qos int) {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.AppendSubscribedTopic(topic, qos)
}

func (self *MmuxConnection) RemoveSubscribedTopic(topic string) {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.RemoveSubscribedTopic(topic)
}

func (self *MmuxConnection) SetKeepaliveInterval(interval int) {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.SetKeepaliveInterval(interval)
}

func (self *MmuxConnection) DisableClearSession() {
	return
}

func (self *MmuxConnection) ShouldClearSession() bool {
	if self.PrimaryConnection == nil {
		return self.CleanSession
	}

	return self.PrimaryConnection.ShouldClearSession()
}
