package server

import (
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"net"
	"strings"
	"time"
)

type TcpServer struct {
	ListenAddress string
	Engine        *Momonga
	config        *configuration.Config
	stop          chan bool
}

func NewTcpServer(engine *Momonga, config *configuration.Config) *TcpServer {
	t := &TcpServer{
		Engine:        engine,
		ListenAddress: config.GetListenAddress(),
		config:        config,
		stop:          make(chan bool, 1),
	}

	return t
}

func (self *TcpServer) ListenAndServe() error {
	addr, err := net.ResolveTCPAddr("tcp4", self.ListenAddress)
	listener, err := net.ListenTCP("tcp", addr)

	if err != nil {
		return err
	}

	for i := 0; i < self.config.GetAcceptorCount(); i++ {
		go self.Serve(listener)
	}
	return nil
}

func (self *TcpServer) Serve(l net.Listener) error {
	log.Info("momonga_tcp: started tcp server")
	defer func() {
		l.Close()
	}()

	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		select {
		case <-self.stop:
			return nil
		default:
			client, err := l.Accept()
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
				if !strings.Contains(err.Error(), "use of closed network connection") {
					log.Error("Accept Failed: %s", err)
				}

				return err
			}
			tempDelay = 0

			conn := NewMyConnection()
			conn.SetMyConnection(client)
			conn.SetId(client.RemoteAddr().String())

			log.Debug("Accepted: %s", conn.GetId())
			go self.Engine.HandleConnection(conn)
		}
	}

	return nil
}

func (self *TcpServer) Stop() {
	self.stop <- true
}
