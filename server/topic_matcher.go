// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"io"
)

type TopicMatcher interface {
	// TODO: should force []*SubscribeSet
	Match(Topic string) []interface{}

	Add(Topic string, Value interface{})

	Remove(Topic string, val interface{})

	Dump(writer io.Writer)
}
