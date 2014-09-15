// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package common

import (
	"encoding/json"
)

type SubscribeSet struct {
	ClientId    string `json:"client_id"`
	TopicFilter string `json:"topic_filter"`
	QoS         int    `json:"qos"`
}

func (self *SubscribeSet) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
