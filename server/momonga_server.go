package server

import (
	"code.google.com/p/go.net/websocket"
	"crypto/tls"
	log "github.com/chobie/momonga/logger"
	"io"
	"net"
	"net/http"
	"os"
	"time"
	//	"github.com/chobie/momonga/encoding/mqtt"
)

type MomongaServer struct {
	listenAddress    string
	listenSocket     string
	SSLlistenAddress string
	database         string
	tlsConfig        *tls.Config
	forceSSLUsers    map[string]bool

	// remote addr based map (ようはてんぽらり)
	Connections     map[string]Connection
	ConnectionCount int
	Engine          *Momonga
	shutdown         chan bool
	TcpShutdown chan bool
	UnixShutdown chan bool
	SSLShutdown chan bool
	Wakeup chan bool
}

func (self *MomongaServer) Terminate() {
	self.shutdown <- true
}

func (self *MomongaServer) SSLAvailable() bool {
	return true
}

func (self *MomongaServer) HandleConnection(Identifier string) {
	var conn Connection
	var ok bool

	if conn, ok = self.Connections[Identifier]; !ok {
		log.Error("Connection not find: %s", Identifier)
		return
	}

	mux, err := self.Engine.Handshake(conn)
	if err != nil {
		log.Debug("Handshake Error: %s", err)
		return
	}

	for {
		err := self.Engine.HandleRequest(mux)
		if err != nil {
			// Closeとかは必ずここでやる
			if _, ok := err.(*DisconnectError); !ok {
				if nerr, ok := err.(net.Error); ok {
					if nerr.Timeout() {
						// Closing connection.
					} else if nerr.Temporary() {
						log.Info("Temporary Error: %s", err)
					}
				} else if err == io.EOF {
					// Expected error. Closing connection.
				} else {
					log.Error("Handle Connection Error: %s", err)
				}
			}

			// どうしよっかなー
			mux.Detach(conn)
			conn.Close()

			if mux.ShouldClearSession() {
				delete(self.Engine.Connections, mux.GetId())
				delete(self.Connections, mux.GetId())
			}

			// ここはテンポラリ接続の所
			delete(self.Connections, Identifier)
			self.ConnectionCount--
			return
		}
	}
}

func (self *MomongaServer) tcpListenAndServe() {
	var err error
	var server net.Listener

	log.Debug("MomongaServer: Listen at: ", self.listenAddress)
	addr, err := net.ResolveTCPAddr("tcp4", self.listenAddress)
	if err != nil {
		log.Error("MomongaServer: ResolveTCPAddr: ", err)
		return
	}

	if self.listenAddress != "" {
		server, err = net.ListenTCP("tcp", addr)
		if err != nil {
			log.Error("MomongaServer: Listen: ", err)
			return
		}
	}
	self.acceptLoop(server, nil)
}

func (self *MomongaServer) tcpSSLListenAndServe() {
	var err error
	var server net.Listener

	log.Debug("TcpSSLServer: Listen at: ", self.SSLlistenAddress)
	if err != nil {
		log.Error("MomongaServer: ResolveTCPAddr: ", err)
		return
	}

	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err == nil {
		// TODO: あとで
		config := tls.Config{Certificates: []tls.Certificate{cert}}
		if self.SSLlistenAddress != "" {
			server, err = tls.Listen("tcp", self.SSLlistenAddress, &config)
			if err != nil {
				log.Error("TCPSSLServer: Listen: ", err)
				return
			}
		}

		self.acceptLoop(server, nil)
	}
}

func (self *MomongaServer) acceptLoop(listener net.Listener, yield func()) {
	defer func() {
		listener.Close()
		// これなんに使ってたんだっけ?
		if yield != nil {
			yield()
		}
	}()

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Error("Accept Failed: ", err)
			continue
		}
		if self.ConnectionCount > 100 {
			// TODO: Send Error message and close connection immediately as we don't won't accept new connection.
		}

		conn := NewTcpConnection(client, self.Engine.ErrorChannel, func(c Connection, time time.Time) {
			log.Debug("Closing Connection")
			//conn.Close()
		})
		self.Connections[conn.GetId()] = conn
		self.ConnectionCount++

		go self.HandleConnection(conn.GetId())
	}
}

func (self *MomongaServer) unixListenAndServe() {
	var err error
	var server net.Listener

	if self.listenSocket != "" {
		server, err = net.Listen("unix", self.listenSocket)
		if err != nil {
			log.Error("UnixServer: Listen: ", err)
			return
		}
	}

	self.acceptLoop(server, func() {
		os.Remove(self.listenSocket)
	})
}

func (self *MomongaServer) RemoveConnection(conn Connection) {
	delete(self.Connections, conn.GetId())
	self.ConnectionCount--
}

func (self *MomongaServer) ListenAndServe() {
	if self.listenAddress != "" {
		go self.tcpListenAndServe()
	}
	if self.listenSocket != "" {
		go self.unixListenAndServe()
	}
	if self.SSLlistenAddress != "" {
		go self.tcpSSLListenAndServe()
	}

	http.Handle("/mqtt", websocket.Handler(func(ws *websocket.Conn) {
		log.Debug("Accept websocket")
		conn := NewTcpConnection(ws, self.Engine.ErrorChannel, func(c Connection, time time.Time) {
			log.Debug("Closing Connection")
			//conn.Close()
		})
		self.Connections[ws.RemoteAddr().String()] = conn
		self.ConnectionCount++
		self.HandleConnection(ws.RemoteAddr().String())
	}))
	go http.ListenAndServe(":9999", nil)

	go self.Engine.Run()
	self.Wakeup <- true

	select {
	case <- self.shutdown:
		self.SSLShutdown <- true
		self.TcpShutdown <- true
		self.UnixShutdown <- true
		self.Engine.Terminate()

		log.Debug("CLOSED")
		return
	}
}
