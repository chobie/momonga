package main

import (
	"fmt"
	"github.com/chobie/momonga/client"
	"github.com/codegangsta/cli"
	"io"
	"net"
	"os"
	"bufio"
	"code.google.com/p/go.net/websocket"
)

func publish(ctx *cli.Context) {
	opt := client.Option{
		TransporterCallback: func() (io.ReadWriteCloser, error) {
			var conn io.ReadWriteCloser
			var err error

			if ctx.Bool("websocket") {
				origin := ctx.String("origin")
				conn, err = websocket.Dial(ctx.String("url"), "", origin)
			} else {
				conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", ctx.String("host"), ctx.Int("port")))
			}
			return conn, err
		},
		Keepalive: 10,
		Magic:   []byte("MQIsdp"),
		Version: 3,
	}

	opt.UserName = ctx.String("u,user")
	opt.Password = ctx.String("P,password")

	c := client.NewClient(opt)

	qos := ctx.Int("q")
	topic := ctx.String("t")
	if topic == "" {
		fmt.Printf("Topic required\n")
		os.Exit(1)
		return
	}

	c.Connect()
	//retain := c.Bool("r")
	go c.Loop()

	if ctx.Bool("s") {
		// Read from Stdin
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			c.Publish(topic, []byte(scanner.Text()), qos)
		}
	} else {
		payload := ctx.String("m")
		c.PublishWait(topic, []byte(payload), qos)
	}
}

func subscribe(ctx *cli.Context) {
	opt := client.Option{
		TransporterCallback: func() (io.ReadWriteCloser, error) {
			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ctx.String("host"), ctx.Int("port")))
			return conn, err
		},
		Keepalive: 10,
		Magic:   []byte("MQIsdp"),
		Version: 3,
	}

	opt.UserName = ctx.String("u,user")
	opt.Password = ctx.String("P,password")

	c := client.NewClient(opt)

	qos := ctx.Int("q")
	topic := ctx.String("t")
	if topic == "" {
		fmt.Printf("Topic required\n")
		return
	}

	c.Connect()
	c.SetPublishCallback(func(TopicFilter string, Payload []byte) {
		fmt.Printf("%s\n", Payload)
	})
	c.Subscribe(topic, qos)
	c.Loop()
}

func main() {
	app := cli.NewApp()
	app.Name = "momonga_cli"
	app.Usage = `Usage momonga_cli -h host -p port
    subscribe path
`

	commonFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "host",
			Value:  "localhost",
			Usage:  "mqtt host to connect to. Defaults to localhost",
			EnvVar: "MQTT_HOST",
		},
		cli.IntFlag{
			Name:   "p, port",
			Value:  1883,
			Usage:  "network port to connect to. Defaults to 1883",
			EnvVar: "MQTT_PORT",
		},
		cli.StringFlag{
			Name:   "u,user",
			Value:  "",
			Usage:  "provide a username",
			EnvVar: "MQTT_USERNAME",
		},
		cli.StringFlag{
			Name:   "P,password",
			Value:  "",
			Usage:  "provide a password",
			EnvVar: "MQTT_PASSWORD",
		},
		cli.StringFlag{"t", "", "mqtt topic to publish to.", ""},
		cli.IntFlag{"q", 0, "QoS", ""},
		cli.StringFlag{"cafile", "", "CA file", ""},
		cli.StringFlag{"i", "", "ClientiId. Defaults random.", ""},
		cli.StringFlag{"m", "test message", "Message body", ""},
		cli.BoolFlag{"r", "message should be retained.", ""},
		cli.BoolFlag{"d", "enable debug messages", ""},
		cli.BoolFlag{"insecure", "do not check that the server certificate", ""},
//		cli.BoolFlag{"websocket", "use websocket", ""},
		cli.StringFlag{"origin", "", "websocket origin", ""},
		cli.StringFlag{"url", "", "websocket url (ws://localhost:8888/mqtt)", ""},
	}

	subFlags := commonFlags
	pubFlags := append(commonFlags,
		cli.BoolFlag{"s", "read message from stdin, sending line by line as a message", ""},
	)
	app.Action = func(c *cli.Context) {
		println(app.Usage)
	}
	app.Commands = []cli.Command{
		{
			Name:   "pub",
			Usage:  "publish",
			Flags:  pubFlags,
			Action: publish,
		},
		{
			Name:   "sub",
			Usage:  "subscribe",
			Flags:  subFlags,
			Action: subscribe,
		},
	}

	app.Run(os.Args)
}
