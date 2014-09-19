// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"expvar"
)

type MyBroker struct {
	Clients MyClients
	Messages MyMessages
	Load MyLoad
	SubscriptionsCount *expvar.Int
	Uptime *expvar.Int
}

type MyMessages struct {
	Received *expvar.Int
	Sent *expvar.Int
	Stored *expvar.Int
	PublishDropped *expvar.Int
	RetainedCount *expvar.Int
}

type MyClients struct {
	Connected *expvar.Int
	Total *expvar.Int
	Maximum *expvar.Int
	Disconnected *expvar.Int
}

type MyLoad struct {
	BytesSend *expvar.Int
	BytesReceived *expvar.Int
}

type MySystem struct{
	Broker MyBroker
}

type MyMetrics struct{
	System MySystem
}

// TODO: should not use expvar as we can't hold multiple MyMetrics metrics.
var Metrics *MyMetrics = &MyMetrics{
	System: MySystem{
		Broker: MyBroker{
			Clients: MyClients{
				Connected: expvar.NewInt("$SYS/broker/clients/connected"),
				Total: expvar.NewInt("$SYS/broker/clients/total"),
				Maximum: expvar.NewInt("$SYS/broker/clients/maximum"),
				Disconnected: expvar.NewInt("$SYS/broker/clients/disconnected"),
			},
			Uptime: expvar.NewInt("$SYS/broker/uptime"),
			Messages: MyMessages{
				Received: expvar.NewInt("$SYS/broker/messages/received"),
				Sent: expvar.NewInt("$SYS/broker/messages/sent"),
				Stored: expvar.NewInt("$SYS/broker/messages/stored"),
				PublishDropped: expvar.NewInt("$SYS/broker/messages/publish/dropped"),
				RetainedCount: expvar.NewInt("$SYS/broker/messages/retained/count"),
			},
			Load: MyLoad{
				BytesSend: expvar.NewInt("$SYS/broker/load/bytes_send"),
				BytesReceived: expvar.NewInt("$SYS/broker/load/bytes_received"),
			},
			SubscriptionsCount: expvar.NewInt("$SYS/broker/subscriptions/count"),
		},
	},
}
