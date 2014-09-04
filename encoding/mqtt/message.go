package mqtt

type Message interface {
	GetType() PacketType
	GetTypeAsString() string
}
