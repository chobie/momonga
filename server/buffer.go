package server

import (
	"bytes"
)

//そもそもつかっているのかという
type Buffer struct {
	ReadBuffer *bytes.Buffer
	WriteBuffer *bytes.Buffer
}

func (self *Buffer) ClearBuffer() {
	self.ReadBuffer.Reset()
	self.WriteBuffer.Reset()
}
