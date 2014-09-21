// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"expvar"
	myexpvar "github.com/chobie/momonga/expvar"
)

type MyBroker struct {
	Clients            MyClients
	Messages           MyMessages
	Load               MyLoad
	SubscriptionsCount *expvar.Int
	Uptime             *expvar.Int
}

type MyMessages struct {
	Received       *expvar.Int
	Sent           *expvar.Int
	Stored         *expvar.Int
	PublishDropped *expvar.Int
	RetainedCount  *expvar.Int
}

type MyClients struct {
	Connected    *expvar.Int
	Total        *expvar.Int
	Maximum      *expvar.Int
	Disconnected *expvar.Int
}

type MyLoad struct {
	BytesSend     *expvar.Int
	BytesReceived *expvar.Int
}

type MySystem struct {
	Broker MyBroker
}

type MyMetrics struct {
	System MySystem

	NumGoroutine      *expvar.Int
	NumCgoCall        *expvar.Int
	Uptime            *expvar.Int
	MemFree           *expvar.Int
	MemUsed           *expvar.Int
	MemActualFree     *expvar.Int
	MemActualUsed     *expvar.Int
	MemTotal          *expvar.Int
	LoadOne           *expvar.Float
	LoadFive          *expvar.Float
	LoadFifteen       *expvar.Float
	CpuUser           *expvar.Float
	CpuNice           *expvar.Float
	CpuSys            *expvar.Float
	CpuIdle           *expvar.Float
	CpuWait           *expvar.Float
	CpuIrq            *expvar.Float
	CpuSoftIrq        *expvar.Float
	CpuStolen         *expvar.Float
	CpuTotal          *expvar.Float
	MessageSentPerSec *myexpvar.DiffInt
	ConnectPerSec     *myexpvar.DiffInt
	GoroutinePerConn  *expvar.Float
}

// TODO: should not use expvar as we can't hold multiple MyMetrics metrics.
var Metrics *MyMetrics = &MyMetrics{
	System: MySystem{
		Broker: MyBroker{
			Clients: MyClients{
				Connected:    expvar.NewInt("sys.broker.clients.connected"),
				Total:        expvar.NewInt("sys.broker.clients.total"),
				Maximum:      expvar.NewInt("sys.broker.clients.maximum"),
				Disconnected: expvar.NewInt("sys.broker.clients.disconnected"),
			},
			Uptime: expvar.NewInt("sys.broker.uptime"),
			Messages: MyMessages{
				Received:       expvar.NewInt("sys.broker.messages.received"),
				Sent:           expvar.NewInt("sys.broker.messages.sent"),
				Stored:         expvar.NewInt("sys.broker.messages.stored"),
				PublishDropped: expvar.NewInt("sys.broker.messages.publish.dropped"),
				RetainedCount:  expvar.NewInt("sys.broker.messages.retained.count"),
			},
			Load: MyLoad{
				BytesSend:     expvar.NewInt("sys.broker.load.bytes_send"),
				BytesReceived: expvar.NewInt("sys.broker.load.bytes_received"),
			},
			SubscriptionsCount: expvar.NewInt("sys.broker.subscriptions.count"),
		},
	},

	// for debug
	NumGoroutine:  expvar.NewInt("numgoroutine"),
	NumCgoCall:    expvar.NewInt("numcgocall"),
	Uptime:        expvar.NewInt("uptime"),
	MemFree:       expvar.NewInt("memfree"),
	MemUsed:       expvar.NewInt("memused"),
	MemActualFree: expvar.NewInt("memactualfree"),
	MemActualUsed: expvar.NewInt("memactualused"),
	MemTotal:      expvar.NewInt("memtotal"),
	LoadOne:       expvar.NewFloat("loadone"),
	LoadFive:      expvar.NewFloat("loadfive"),
	LoadFifteen:   expvar.NewFloat("loadfifteen"),
	CpuUser:       expvar.NewFloat("cpuuser"),
	CpuNice:       expvar.NewFloat("cpunice"),
	CpuSys:        expvar.NewFloat("cpusys"),
	CpuIdle:       expvar.NewFloat("cpuidle"),
	CpuWait:       expvar.NewFloat("cpuwait"),
	CpuIrq:        expvar.NewFloat("cpuirq"),
	CpuSoftIrq:    expvar.NewFloat("cpusoftirq"),
	CpuStolen:     expvar.NewFloat("cpustolen"),
	CpuTotal:      expvar.NewFloat("cputotal"),

	MessageSentPerSec: myexpvar.NewDiffInt("msg_sent_per_sec"),
	ConnectPerSec:     myexpvar.NewDiffInt("connect_per_sec"),
	GoroutinePerConn:  expvar.NewFloat("goroutine_per_conn"),
}
