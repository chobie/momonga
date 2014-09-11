// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"github.com/chobie/momonga/configuration"
)

const KILOBYTE = 1024
const MEGABYTE = 1024 * KILOBYTE
const MAX_REQUEST_SIZE = MEGABYTE * 2

func NewMomongaServer(conf *configuration.Config) (*MomongaServer, error) {
	server := &MomongaServer{
		forceSSLUsers: map[string]bool{},
		Connections:   map[string]Connection{},
		Engine: NewMomonga(),
	}

	server.listenAddress = conf.GetListenAddress()
	server.SSLlistenAddress = conf.GetSSLListenAddress()
	server.listenSocket = conf.GetSocketAddress()
	server.WebSocketMount = conf.Server.WebSocketMount
	server.WebSocketPort = conf.Server.WebSocketPort

	if !conf.Server.EnableSys {
		server.Engine.DisableSys()
	}

	server.Engine.SetupCallback()

	// TODO: SSL
	//	if config.TcpInputUseSSL {
	//		cert, err := tls.LoadX509KeyPair(config.TcpInputSSLCert(), config.TcpInputSSLKey())
	//		if err != nil {
	//			log.Error("tcp server: loadkeys failed. disable ssl feature: %s", err)
	//		} else {
	//			tslConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	//			tslConfig.Rand = rand.Reader
	//
	//			server.tlsConfig = tslConfig
	//			for _, name := range config.TcpInputForceSSL() {
	//				server.forceSSLUsers[name] = true
	//			}
	//			log.Debug("SSL Config loaded")
	//		}
	//	}

	server.shutdown = make(chan bool, 1)
	server.TcpShutdown = make(chan bool, 1)
	server.SSLShutdown = make(chan bool, 1)
	server.UnixShutdown = make(chan bool, 1)
	server.Wakeup = make(chan bool, 1)

	return server, nil
}
