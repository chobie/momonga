// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"time"
)

// Topic is a kind of queue. this is just a container. I don't use this for queueing.
type Topic struct {
	Level     int
	Name      string
	QoS       int
	CreatedAt time.Time
}
