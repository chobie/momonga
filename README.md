Momonga MQTT
============

![momonga gopher](http://i.imgur.com/Jbo9Gl8.png)

# About

Momonga is a Golang MQTT library to make MQTT client and Server easily.

This project has been started as my hobby project, to learn MQTT3.1.1 behaviour.
This might contains some unreliable codes. Yes, contributions are very welcome.

# Features

* MQTT 3.1.1 compliant
* QoS 0 supported.
* also QoS 1, 2 are available, although really suck implementation. don't rely it.

Misc

* WebSocket Support (might contain bugs. requires go.net/websocket)
* SSL Support
* UnixSocket Support

# Requirements

Go 1.3 higher

# Version

dev

# Quick Start

```
#momonga (MQTT Server)
go get -u github.com/chobie/momonga/momonga
momonga -config=config.toml

#momonga_cli
go get -u github.com/chobie/momonga/momonga_cli
momonga_cli
```

# Development

NOTE: momonga is still under development. API may change.

```
mkdir momonga && cd momonga
export GOPATH=`pwd`
go get -u github.com/chobie/momonga/momonga

# server
go run src/github.com/chobie/momonga/momonga/momonga.go -config=src/github.com/chobie/momonga/config.toml

# cli
go build -o momonga_cli src/github.com/chobie/momonga/momonga_cli/momonga_cli.go
```

# Author

Shuhei Tanuma

# Links

* MQTT 3.1.1 Specification
http://docs.oasis-open.org/mqtt/mqtt/v3.1.1/csprd01/mqtt-v3.1.1-csprd01.html

* Conformance Test
http://www.eclipse.org/paho/clients/testing/

# Conformance Test

```
momonga% python3 interoperability/client_test.py
hostname localhost port 1883
clean up starting
clean up finished
Basic test starting
Basic test succeeded
Retained message test starting
Retained message test succeeded
This server is queueing QoS 0 messages for offline clients
Offline message queueing test succeeded
Will message test succeeded
Overlapping subscriptions test starting
This server is publishing one message for all matching overlapping subscriptions, not one for each.
Overlapping subscriptions test succeeded
Keepalive test starting
Keepalive test succeeded
Redelivery on reconnect test starting
Redelivery on reconnect test succeeded
test suite succeeded
```

# Prebuilt Binaries

for linux, osx

https://drone.io/github.com/chobie/momonga/files

```
wget https://drone.io/github.com/chobie/momonga/files/artifacts/bin/linux_arm/momonga
chmod +x momonga
./momonga
```

# License - MIT License -

Copyright (c) 2014 Shuhei Tanuma,

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.