package server

import (
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"net"
	"strings"
	"time"
)

type UnixServer struct {
	Engine  *Momonga
	Address string
}

func NewUnixServer(engine *Momonga, config *configuration.Config) *UnixServer {
	t := &UnixServer{
		Engine:  engine,
		Address: config.GetSocketAddress(),
	}

	return t
}

func (self *UnixServer) ListenAndServe() error {
	listener, err := net.Listen("unix", self.Address)

	if err != nil {
		return err
	}

	return self.Serve(listener)
}

func (self *UnixServer) Serve(l net.Listener) error {
	log.Info("momonga_unix: started server")
	defer func() {
		l.Close()
	}()

	var tempDelay time.Duration // how long to sleep on accept failure
	for {
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

	return nil
}
