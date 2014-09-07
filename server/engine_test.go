package server

import (
	"github.com/chobie/momonga/configuration"
	. "gopkg.in/check.v1"
	"testing"
	"github.com/chobie/momonga/client"
	"net"
	"io"
	"os"
	"time"
)

func Test(t *testing.T) { TestingT(t) }

type EngineSuite struct{}

var _ = Suite(&EngineSuite{})

func (s *EngineSuite) SetUpSuite(c *C) {
	os.Remove("/Users/chobie/src/momonga/socket")
}

func (s *EngineSuite) TearDownSuite(c *C) {
}

func (s *EngineSuite) TestServerShouldCreateWithDefaultConfiguration(c *C) {
	conf, _ := configuration.LoadConfiguration("")
	_, err := NewMomongaServer(conf)
	c.Assert(err, Equals, nil)
}

func (s *EngineSuite) TestBasic(c *C) {
	conf, _ := configuration.LoadConfiguration("")
	conf.Server.Port = -1
	conf.Server.Socket = "/Users/chobie/src/momonga/socket"

	svr, _ := NewMomongaServer(conf)
	go svr.ListenAndServe()
	<- svr.Wakeup

	cli := client.NewClient(client.Option{
		TransporterCallback: func() (io.ReadWriteCloser, error) {
			conn, err := net.Dial("unix", "/Users/chobie/src/momonga/socket")
			return conn, err
		},
	})

	go cli.Loop()
	cli.Connect()
	cli.Publish("/debug", []byte("hoge"), 0)
	time.Sleep(time.Second)
	svr.Terminate()
}
