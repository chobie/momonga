package server

import(
	"net"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/configuration"
	"net/http"
	"fmt"
)

type HttpServer struct {
	http.Server
	Engine *Momonga
	Address string
}

func NewHttpServer(engine *Momonga, config *configuration.Config) *HttpServer{
	t := &HttpServer{
		Server: http.Server{
			Handler: &MyHttpServer{
				Engine: engine,
				WebSocketMount: config.Server.WebSocketMount,
			},
		},
		Engine: engine,
		Address: fmt.Sprintf(":%d", config.Server.HttpPort),
	}

	return t
}

func (self *HttpServer) ListenAndServe() error{
	addr, err := net.ResolveTCPAddr("tcp4", ":9999")
	base, err := net.ListenTCP("tcp", addr)

	listener := NewHttpListener(base)
	if err != nil {
		return err
	}

	return self.Serve(listener)
}

func (self *HttpServer) Serve(l net.Listener) error{
	log.Info("momonga_http: started tpc server")

	self.Server.Serve(l)
	return nil
}

