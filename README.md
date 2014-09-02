Momonga MQTT
============

# About

Momonga is a Golang MQTT library

This project has been started as my hobby project, to learn MQTT3.1.1 behaviour.
This might contains some unreliable codes. Yes, contributions are very welcome.

# Features

* MQTT 3.1.1 compliant
  (Also allows 3.1 connections, but don't interested previous implementation)
* QoS 0, 1, 2 (I don't say momonga guarantees QoS policy. Especially, error handling is poor)

Misc

* WebSocket Support (might contain bugs. requires go.net/websocket)
* SSL Support
* UnixSocket Support

# Version

dev

# Quick Start

momonga_cli

```
go get -u github.com/chobie/momonga/momonga_cli
#subscribe /debug topic
momonga_cli sub -t /debug

```

# Dependencies

```
required:
code.google.com/p/log4go

optional:
github.com/chobie/gopsutil
github.com/BurntSushi/toml
code.google.com/p/go.net/websocket
```

# Development

```
mkdir -p momonga/src
cd momonga
export GOPATH=`pwd`

go get -u code.google.com/p/log4go
go get -u code.google.com/p/go.net/websocket
go get -u github.com/BurntSushi/toml
go get -u github.com/chobie/momonga

# cli
go build -o momonga_cli src/github.com/chobie/momonga/momonga_cli/momonga_cli.go

# server
go build -o moomngad src/github.com/chobie/momonga/momongad/momongad.go

```

# Author

Shuhei Tanuma

# Links

* MQTT 3.1.1 Specification
http://docs.oasis-open.org/mqtt/mqtt/v3.1.1/csprd01/mqtt-v3.1.1-csprd01.html

* Conformance Test
http://www.eclipse.org/paho/clients/testing/

# License - MIT License -

Copyright (c) 2014 Shuhei Tanuma,

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.