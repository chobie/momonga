package mqtt

type Message interface {
	GetType() MessageType
}
