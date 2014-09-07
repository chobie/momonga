package server

import (
	"github.com/chobie/momonga/encoding/mqtt"
	"time"
)

// Topicは一種のQueueっちゅーこと。TopicにQueue持たせるべき?subscribeでWildcardできるからちょっと違うんだよなぁ
type Topic struct {
	Level     int
	Name      string
	QoS       int
	CreatedAt time.Time
	Retain    *mqtt.PublishMessage
}
