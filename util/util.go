// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package util

import (
	"crypto/rand"
	log "github.com/chobie/momonga/logger"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"
)

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
	var ret uintptr
	var err syscall.Errno

	ret, _, err = syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if err != 0 {
		log.Info("failed to fall fork")
		return -1
	}
	switch ret {
	case 0:
		break
	default:
		os.Exit(0)
	}
	pid, err2 := syscall.Setsid()
	if err2 != nil {
		log.Info("failed to call setsid %+v", err2)
	}
	if pid == -1 {
		return -1
	}

	if nochdir == 0 {
		os.Chdir("/")
	}

	syscall.Umask(0)
	if noclose == 0 {
		f, e := os.OpenFile("/dev/null", os.O_RDWR, 0)
		if e == nil {
			fd := int(f.Fd())
			syscall.Dup2(fd, int(os.Stdin.Fd()))
			syscall.Dup2(fd, int(os.Stdout.Fd()))
			syscall.Dup2(fd, int(os.Stderr.Fd()))
		}
	}
	return 0
}
