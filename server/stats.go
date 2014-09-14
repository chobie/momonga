// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"expvar"
)

var sys_broker_clients_connected *expvar.Int
var sys_broker_broker_uptime *expvar.Int
var sys_broker_messages_received *expvar.Int
var sys_broker_messages_sent *expvar.Int
var sys_broker_messages_stored *expvar.Int
var sys_broker_messages_publish_dropped *expvar.Int
var sys_broker_messages_retained_count *expvar.Int
var sys_broker_messages_inflight *expvar.Int
var sys_broker_clients_total *expvar.Int
var sys_broker_clients_maximum *expvar.Int
var sys_broker_clients_disconnected *expvar.Int
var sys_broker_load_bytes_sent *expvar.Int
var sys_broker_load_bytes_received *expvar.Int
var sys_broker_subscriptions_count *expvar.Int

func init() {
	sys_broker_clients_connected = expvar.NewInt("$SYS/broker/clients/connected")
	sys_broker_broker_uptime = expvar.NewInt("$SYS/broker/broker/uptime")
	sys_broker_broker_uptime = expvar.NewInt("$SYS/broker/broker/time")
	sys_broker_load_bytes_received = expvar.NewInt("$SYS/broker/messages/received")
	sys_broker_load_bytes_sent = expvar.NewInt("$SYS/broker/messages/sent")
	sys_broker_messages_stored = expvar.NewInt("$SYS/broker/messages/stored")
	sys_broker_messages_publish_dropped = expvar.NewInt("$SYS/broker/messages/publish/dropped")
	sys_broker_messages_retained_count = expvar.NewInt("$SYS/broker/messages/retained/count")
	sys_broker_messages_inflight = expvar.NewInt("$SYS/broker/messages/inflight")
	sys_broker_clients_total = expvar.NewInt("$SYS/broker/clients/total")
	sys_broker_clients_maximum = expvar.NewInt("$SYS/broker/clients/maximum")
	sys_broker_clients_disconnected = expvar.NewInt("$SYS/broker/clients/disconnected")
	sys_broker_load_bytes_sent = expvar.NewInt("$SYS/broker/load/bytes/sent")
	sys_broker_load_bytes_received = expvar.NewInt("$SYS/broker/load/bytes/received")
	sys_broker_subscriptions_count = expvar.NewInt("$SYS/broker/subscriptions/count")
}
