// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package util

import (
	"crypto/rand"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"expvar"
)

func GetIntValue(i *expvar.Int) int64 {
	v := i.String()
	vv, e := strconv.ParseInt(v, 10, 64)
	if e != nil {
		return 0
	}

	return vv
}

func GetFloatValue(f *expvar.Float) float64 {
	a, e := strconv.ParseFloat(f.String(), 64)
	if e != nil {
		return 0
	}

	return a
}


func GenerateId(n int) string {
	const alphanum = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, n)

	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	return string(bytes)
}

func WritePid(pidFile string) {
	if pidFile != "" {
		pid := strconv.Itoa(os.Getpid())
		if err := ioutil.WriteFile(pidFile, []byte(pid), 0644); err != nil {
			panic(err)
		}
	}
}

func Daemonize(nochdir, noclose int) int {
	if os.Getenv("DAEMONIZE") != "TRUE" {
		var pwd string

		path, _ := exec.LookPath(os.Args[0])
		if nochdir == 1 {
			pwd, _ = os.Getwd()
		} else {
			pwd = "/"
		}
		descriptors := []*os.File{
			os.Stdin,
			os.Stdout,
			os.Stderr,
		}
		var env []string
		for _, v := range os.Environ() {
			env = append(env, v)
		}
		env = append(env, "DAEMONIZE=TRUE")
		p, _ := os.StartProcess(path, os.Args, &os.ProcAttr{
			Dir:   pwd,
			Env:   env,
			Files: descriptors,
		})

		if noclose == 0 {
			p.Release()
		}
		os.Exit(0)
	}

	return 0
}
