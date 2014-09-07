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
