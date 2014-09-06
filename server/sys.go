package server

type System struct {
	Broker SystemBroker
}

type SystemBroker struct {
	Load SystemBrokerLoad
	Clients SystemBrokerClients
	Messages SystemBrokerMessages
	Subscriptions SystemBrokerSubscriptions
	Broker SystemBrokerBroker
}

type SystemBrokerLoad struct {
	Bytes SystemBrokerLoadBytes
}

type SystemBrokerLoadBytes struct {
	Received int
	Sent int
}

type SystemBrokerClients struct {
	Connected int
	Disconnected int
	Maximum int
	Total int
}

type SystemBrokerMessages struct {
	Inflight int
	Received int
	Sent int
	Stored int
	Publish SystemBrokerMessagesPublish
	Retained SystemBrokerMessagesRetained
}

type SystemBrokerMessagesPublish struct {
	Sent int
	Count int
}

type SystemBrokerMessagesRetained struct {
	Count int
}
type SystemBrokerSubscriptions struct {
	Count int
}

type SystemBrokerBroker struct {
	Time int
	Uptime int
	Version string
}
