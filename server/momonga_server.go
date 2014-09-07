package server

import (
	"crypto/tls"
	log "github.com/chobie/momonga/logger"
	"io"
	"net"
	"os"
	"code.google.com/p/go.net/websocket"
	"net/http"
	"fmt"
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
	WebSocketPort int
	WebSocketMount string
}

func (self *MomongaServer) Terminate() {
	self.shutdown <- true
}

func (self *MomongaServer) SSLAvailable() bool {
	return true
}

func (self *MomongaServer) HandleConnection(conn Connection) {
	for {
		_, err := conn.ReadMessage()
		if conn.GetState() == STATE_CLOSED {
			err = &DisconnectError{}
		}

		if err != nil {
			self.Engine.CleanSubscription(conn)
			if _, ok := err.(*DisconnectError); !ok {
				if conn.HasWillMessage() {
					self.Engine.SendWillMessage(conn)
				}
			} else if err == io.EOF {
				// nothing to do
			} else {
				log.Error("Handle Connection Error: %s", err)
			}

			if mux, ok := self.Engine.Connections[conn.GetId()]; ok {
				mux.Detach(conn)

				if mux.ShouldClearSession() {
					delete(self.Engine.Connections, mux.GetId())
					delete(self.Connections, mux.GetId())
					log.Debug("remove mux connection")
				}
			}

			conn.Close()
			// ここはテンポラリ接続の所?
			delete(self.Connections, conn.GetId())
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

		//conn := NewTcpConnection(client, self.Engine.ErrorChannel)

		conn := NewMyConnection()
		hndr := NewHandler(conn, self.Engine)

		conn.SetOpaque(hndr)
		conn.SetMyConnection(client)
		conn.SetId(client.RemoteAddr().String())

		log.Debug("Accepted: %s", conn.GetId())
		// これサーバー側でも確保する必要あるんかなぁ。
		self.Connections[conn.GetId()] = conn
		self.ConnectionCount++

		go self.HandleConnection(conn)
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

	// TODO: 場所変える
	if self.WebSocketPort > 0 {
		http.Handle(self.WebSocketMount, websocket.Handler(func(ws *websocket.Conn) {
				conn := NewMyConnection()
				conn.SetMyConnection(ws)
				conn.SetId(ws.RemoteAddr().String())

				self.Connections[ws.RemoteAddr().String()] = conn
				self.ConnectionCount++
				self.HandleConnection(conn)
			}))

		go http.ListenAndServe(fmt.Sprintf(":%d", self.WebSocketPort), nil)
	}

	// TODO: pass concurrency parameter
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
