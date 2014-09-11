package server

import (
	"crypto/tls"
	"github.com/chobie/momonga/util"
	log "github.com/chobie/momonga/logger"
	"io"
	"net"
	_ "os"
	"code.google.com/p/go.net/websocket"
	"net/http"
	"fmt"
	"time"
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
	hndr := NewHandler(conn, self.Engine)

	for {
		_, err := conn.ReadMessage()
		if conn.GetState() == STATE_CLOSED {
			err = &DisconnectError{}
		}

		if err != nil {
			log.Debug("DISCONNECT: %s", conn.GetId())
			// ここでmyconnがかえる場合はhandshake前に死んでる
			//self.Engine.CleanSubscription(conn)
			var ok bool
			var mux *MmuxConnection

			if mux, ok = self.Engine.Connections[conn.GetId()]; ok {
			}

			if _, ok := err.(*DisconnectError); !ok {
				if conn.HasWillMessage() {
					self.Engine.SendWillMessage(conn)
				}

				if err == io.EOF {
					// nothing to do
				} else {
					log.Error("Handle Connection Error: %s", err)
				}
			}

			if mux != nil {
				mux.Detach(conn)

				if mux.ShouldClearSession() {
					self.Engine.CleanSubscription(mux)
					delete(self.Engine.Connections, mux.GetId())
				}
			}

			conn.Close()
			hndr.Close()
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

	// NOTE: for durability
	for i := 0; i < 4; i++ {
		go self.acceptLoop(server)
	}
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

		self.acceptLoop(server)
	}
}

func (self *MomongaServer) acceptLoop(listener net.Listener) {
	defer func() {
		listener.Close()
	}()

	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		client, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				log.Info("momonga: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			log.Error("Accept Failed: %s", err)
			return
		}
		tempDelay = 0

		if self.ConnectionCount > 100 {
			// TODO: Send Error message and close connection immediately as we don't won't accept new connection.
		}

		conn := NewMyConnection()
		conn.SetMyConnection(client)
		conn.SetId(client.RemoteAddr().String())
		log.Debug("Accepted: %s", conn.GetId())
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
	//os.Remove(self.listenSocket)
	self.acceptLoop(server)
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
	//if self.WebSocketPort > 0 {
		http.Handle(self.WebSocketMount, websocket.Handler(func(ws *websocket.Conn) {
				conn := NewMyConnection()
				conn.SetMyConnection(ws)
				conn.SetId(ws.RemoteAddr().String())

				self.Connections[ws.RemoteAddr().String()] = conn
				self.ConnectionCount++
				self.HandleConnection(conn)
			}))

		// あるとべんりなのよねー
		http.HandleFunc("/debug/retain", func (w http.ResponseWriter, r *http.Request) {
				itr := self.Engine.DataStore.Iterator()
				for ; itr.Valid(); itr.Next() {
					k := string(itr.Key())
					fmt.Fprintf(w, "<div>key: %s</div>", k)
				}
		})
		http.HandleFunc("/debug/retain/clear", func (w http.ResponseWriter, r *http.Request) {
				itr := self.Engine.DataStore.Iterator()
				var targets []string
				for ; itr.Valid();itr.Next() {
					x := itr.Key()
					targets = append(targets, string(x))
				}

				for _, s := range targets {
					self.Engine.DataStore.Del([]byte(s), []byte(s))
				}
				fmt.Fprintf(w, "<textarea>%#v</textarea>", self.Engine.DataStore)
		})

		http.HandleFunc("/debug/connection", func (w http.ResponseWriter, r *http.Request) {
			for _, v := range self.Engine.Qlobber.Match("TopicA/C") {
				fmt.Fprintf(w, "<div>%s</div>", v)
			}
		})
		http.HandleFunc("/debug/connections", func (w http.ResponseWriter, r *http.Request) {
				for _, v := range self.Engine.Connections {
					fmt.Fprintf(w, "<div>%#v</div>", v)
				}
		})

		http.HandleFunc("/debug/connections/clear", func (w http.ResponseWriter, r *http.Request) {
			self.Engine.Connections = make(map[string]*MmuxConnection)
			fmt.Fprintf(w, "cleared")
		})

		http.HandleFunc("/debug/qlobber/clear", func (w http.ResponseWriter, r *http.Request) {
			self.Engine.Qlobber = util.NewQlobber()
			fmt.Fprintf(w, "cleared")
		})

		http.HandleFunc("/debug/qlobber/dump", func (w http.ResponseWriter, r *http.Request) {
			self.Engine.Qlobber.Dump()
			fmt.Fprintf(w, "dumped")
		})

	go http.ListenAndServe(fmt.Sprintf(":%d", 9000), nil)
	//}

	// TODO: pass concurrency parameter. configでいいんじゃない
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
