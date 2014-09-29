// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"code.google.com/p/go.net/websocket"
	"expvar"
	"fmt"
	"github.com/BurntSushi/toml"
	. "github.com/chobie/momonga/common"
	"github.com/chobie/momonga/util"
	"io"
	"io/ioutil"
	"net/http"
	httpprof "net/http/pprof"
	"net/url"
	"runtime"
	"strconv"
	//log "github.com/chobie/momonga/logger"
)

func init() {
	runtime.SetBlockProfileRate(1)
}

type MyHttpServer struct {
	Engine         *Momonga
	WebSocketMount string
}

func (self *MyHttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	err := self.apiRouter(w, req)
	if err == nil {
		return
	}

	err = self.debugRouter(w, req)
	if err == nil {
		return
	}
}

func (self *MyHttpServer) debugRouter(w http.ResponseWriter, req *http.Request) error {
	switch req.URL.Path {
	case "/debug/vars":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, "{\n")
		first := true
		expvar.Do(func(kv expvar.KeyValue) {
			if kv.Key == "cmdline" || kv.Key == "memstats" {
				return
			}
			if !first {
				fmt.Fprintf(w, ",\n")
			}
			first = false
			fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
		})
		fmt.Fprintf(w, "\n}\n")
	case "/debug/pprof":
		httpprof.Index(w, req)
	case "/debug/pprof/cmdline":
		httpprof.Cmdline(w, req)
	case "/debug/pprof/symbol":
		httpprof.Symbol(w, req)
	case "/debug/pprof/heap":
		httpprof.Handler("heap").ServeHTTP(w, req)
	case "/debug/pprof/goroutine":
		httpprof.Handler("goroutine").ServeHTTP(w, req)
	case "/debug/pprof/profile":
		httpprof.Profile(w, req)
	case "/debug/pprof/block":
		httpprof.Handler("block").ServeHTTP(w, req)
	case "/debug/pprof/threadcreate":
		httpprof.Handler("threadcreate").ServeHTTP(w, req)
	case "/debug/retain":
		itr := self.Engine.DataStore.Iterator()
		for ; itr.Valid(); itr.Next() {
			k := string(itr.Key())
			fmt.Fprintf(w, "<div>key: %s</div>", k)
		}
	case "/debug/retain/clear":
		itr := self.Engine.DataStore.Iterator()
		var targets []string
		for ; itr.Valid(); itr.Next() {
			x := itr.Key()
			targets = append(targets, string(x))
		}
		for _, s := range targets {
			self.Engine.DataStore.Del([]byte(s), []byte(s))
		}
		fmt.Fprintf(w, "<textarea>%#v</textarea>", self.Engine.DataStore)
	case "/debug/connections":
		for _, v := range self.Engine.Connections {
			fmt.Fprintf(w, "<div>%#v</div>", v)
		}
	case "/debug/connections/clear":
		self.Engine.Connections = make(map[string]*MmuxConnection)
		fmt.Fprintf(w, "cleared")
	case "/debug/qlobber/clear":
		self.Engine.TopicMatcher = util.NewQlobber()
		fmt.Fprintf(w, "cleared")
	case "/debug/qlobber/dump":
		fmt.Fprintf(w, "qlobber:\n")
		self.Engine.TopicMatcher.Dump(w)
	case "/debug/config/dump":
		e := toml.NewEncoder(w)
		e.Encode(self.Engine.Config())

	default:
		return fmt.Errorf("404 %s", req.URL.Path)
	}
	return nil
}

func (self *MyHttpServer) apiRouter(w http.ResponseWriter, req *http.Request) error {
	switch req.URL.Path {
	case "/":
		fmt.Fprintf(w, "HELO MOMONGA WORLD")
	case "/pub":
		reqParams, err := url.ParseQuery(req.URL.RawQuery)
		if err != nil {
			return nil
		}

		var topic string
		var qos string
		if topics, ok := reqParams["topic"]; ok {
			topic = topics[0]
		}
		if qoss, ok := reqParams["qos"]; ok {
			qos = qoss[0]
		}

		if qos == "" {
			qos = "0"
		}

		readMax := int64(8192)
		body, _ := ioutil.ReadAll(io.LimitReader(req.Body, readMax))
		if len(body) < 1 {
			return fmt.Errorf("body required")
		}

		rqos, _ := strconv.ParseInt(qos, 10, 32)
		self.Engine.SendMessage(topic, []byte(body), int(rqos))
		w.Write([]byte(fmt.Sprintf("OK")))
		return nil
	case "/stats":
		return nil
	case self.WebSocketMount:
		websocket.Handler(func(ws *websocket.Conn) {
			// need for binary frame
			ws.PayloadType = 0x02

			myconf := GetDefaultMyConfig()
			myconf.MaxMessageSize = self.Engine.Config().Server.MessageSizeLimit
			conn := NewMyConnection(myconf)
			conn.SetMyConnection(ws)
			conn.SetId(ws.RemoteAddr().String())
			self.Engine.HandleConnection(conn)
		}).ServeHTTP(w, req)
	default:
		return fmt.Errorf("404 %s", req.URL.Path)
	}
	return nil
}

func (self *MyHttpServer) getTopicFromQuery(req *http.Request) (url.Values, string, error) {
	reqParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return nil, "", err
	}

	topicNames, _ := reqParams["topic"]
	topicName := topicNames[0]

	return reqParams, topicName, nil
}
