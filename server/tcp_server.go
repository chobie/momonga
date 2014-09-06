package server

import (
	"net"
	"os"
	"io"
	"time"
	"crypto/tls"
	log "github.com/chobie/momonga/logger"
	"net/http"
	"code.google.com/p/go.net/websocket"
//	"github.com/chobie/momonga/encoding/mqtt"
)

type TcpServer struct {
	listenAddress  string
	listenSocket   string
	SSLlistenAddress  string
	database       string
	shutdown       chan bool
	tlsConfig      *tls.Config
	forceSSLUsers map[string]bool

	// remote addr based map (ようはてんぽらり)
	Connections map[string]Connection
	ConnectionCount int
	Engine *Pidgey
}

func (self *TcpServer) SSLAvailable() bool {
	return true
}

func (self *TcpServer) HandleConnection(Identifier string) {
	var conn Connection
	var ok bool

	if conn, ok = self.Connections[Identifier]; !ok {
		log.Error("Connection not find: %s", Identifier)
		return
	}

	mux, err := self.Engine.Handshake(conn)
	if err != nil{
		log.Debug("Handshake Error: %s", err)
		return
	}

	for {
		err := self.Engine.HandleRequest(mux)
		if err != nil {
			// Closeとかは必ずここでやる
			if _, ok := err.(*DisconnectError); !ok {
				if err != io.EOF {
					log.Error("Handle Connection Error: [%s] %s", conn.GetId(), err)
				}
			}

			// どうしよっかなー
			mux.Detach(conn)
			conn.Close()

			if mux.ShouldClearSession() {
				delete(self.Connections, mux.GetId())
			}
			// ここはテンポラリ接続の所
			delete(self.Connections, Identifier)
			self.ConnectionCount--
			return
		}
	}
}

func (self *TcpServer) tcpListenAndServe() {
	var err error
	var server net.Listener

	log.Debug("TcpServer: Listen at: ", self.listenAddress)
	addr, err := net.ResolveTCPAddr("tcp4", self.listenAddress)
	if err != nil {
		log.Error("TCPServer: ResolveTCPAddr: ", err)
		return
	}

	if self.listenAddress != "" {
		server, err = net.ListenTCP("tcp", addr)
		if err != nil {
			log.Error("TCPServer: Listen: ", err)
			return
		}
	}
	self.acceptLoop(server, nil)
}

func (self *TcpServer) tcpSSLListenAndServe() {
	var err error
	var server net.Listener

	log.Debug("TcpSSLServer: Listen at: ", self.SSLlistenAddress)
	if err != nil {
		log.Error("TCPServer: ResolveTCPAddr: ", err)
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


func (self *TcpServer) acceptLoop(listener net.Listener, yield func()) {
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

		conn := NewTcpConnection(client, self, self.Engine.ErrorChannel, func(c Connection, time time.Time) {
				log.Debug("Closing Connection")
				//conn.Close()
			})
		self.Connections[conn.GetId()] = conn
		self.ConnectionCount++

		go self.HandleConnection(conn.GetId())
	}
}

func (self *TcpServer) unixListenAndServe() {
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

func (self *TcpServer) RemoveConnection(conn Connection) {
	delete(self.Connections, conn.GetId())
	self.ConnectionCount--
}

func (self *TcpServer) ListenAndServe() {
	defer func() {
		self.shutdown <- true
	}()

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
			conn := NewTcpConnection(ws, self, self.Engine.ErrorChannel, func(c Connection, time time.Time) {
					log.Debug("Closing Connection")
					//conn.Close()
				})
			self.Connections[ws.RemoteAddr().String()] = conn
			self.ConnectionCount++
			self.HandleConnection(ws.RemoteAddr().String())
		}))
	go http.ListenAndServe(":9999", nil)
	go self.Engine.Run()
	select {}
}
