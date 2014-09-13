// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package util

import (
	"time"
)

type Balancer struct {
	Prior           int64
	Start           int64
	Total           float64
	ElapsedUseconds int64
	SleepUsec       int64
	PerSec          int
}

func (self *Balancer) Execute(callback func()) {
	self.Start = time.Now().UnixNano() / 1e3
	if self.Prior > 0 {
		self.ElapsedUseconds = self.Start - self.Prior

		exec := float64(self.ElapsedUseconds) * float64(self.PerSec/1000000)
		self.Total -= exec
		if self.Total < 0.0 {
			self.Total = 0.0
		}
	}
	self.Total += 1

	callback()

	self.SleepUsec = int64(float64(self.Total) * float64(1000000/self.PerSec))
	if self.SleepUsec < (10000 / 10) {
		// go ahead!
		self.Prior = self.Start
		return
	}

	// hey, let's take a rest.
	time.Sleep(time.Duration(self.SleepUsec * 1000))
	self.Prior = time.Now().UnixNano() / 1e3
	self.ElapsedUseconds = self.Prior - self.Start
	self.Total = float64(self.SleepUsec-self.ElapsedUseconds) * float64(self.PerSec) / float64(1000000)
}
