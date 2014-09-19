// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"encoding/json"
	"io"
)

type DisconnectMessage struct {
	FixedHeader
}

func (self DisconnectMessage) WriteTo(w io.Writer) (int64, error) {
	var fsize = 0
	size, err := self.FixedHeader.writeTo(fsize, w)
	if err != nil {
		return 0, err
	}

	return int64(fsize) + size, nil
}

func (self *DisconnectMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
