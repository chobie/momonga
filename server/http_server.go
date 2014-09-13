package server

import (
	"fmt"
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"net"
	"net/http"
)

type HttpServer struct {
	http.Server
	Engine  *Momonga
	Address string
	stop    chan bool
}

func NewHttpServer(engine *Momonga, config *configuration.Config) *HttpServer {
	t := &HttpServer{
		Server: http.Server{
			Handler: &MyHttpServer{
				Engine:         engine,
				WebSocketMount: config.Server.WebSocketMount,
			},
		},
		Engine:  engine,
		Address: fmt.Sprintf(":%d", config.Server.HttpPort),
		stop:    make(chan bool, 1),
	}

	return t
}

func (self *HttpServer) ListenAndServe() error {
	addr, err := net.ResolveTCPAddr("tcp4", ":9999")
	base, err := net.ListenTCP("tcp", addr)

	listener := NewHttpListener(base)
	if err != nil {
		return err
	}

	return self.Serve(listener)
}

func (self *HttpServer) Serve(l net.Listener) error {
	log.Info("momonga_http: started http server")

	// アッー. Stop出来ねぇ
	go self.Server.Serve(l)
	return nil
}

func (self *HttpServer) Stop() {
	self.stop <- true
}
