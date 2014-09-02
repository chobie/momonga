package server

import (
	"time"
	"github.com/chobie/momonga/encoding/mqtt"
)

// Topicは一種のQueueっちゅーこと。TopicにQueue持たせるべき
type Topic struct {
	Level int
	Name string
	QoS int
	CreatedAt time.Time
	Retain *mqtt.PublishMessage
}
