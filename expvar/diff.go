package expvar

import (
	origin "expvar"
	"strconv"
	"sync"
)

type DiffInt struct {
	mu sync.RWMutex
	i  int64
	l  int64
	c  int
	b  bool
}

func (v *DiffInt) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.c > 2 {
		return strconv.FormatInt(v.i-v.l, 10)
	} else {
		return "0"
	}
}

func (v *DiffInt) Set(delta int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.l = v.i
	v.i = delta
	v.c += 1
	if !v.b && v.c > 1 {
		v.b = true
	}
}

func NewDiffInt(name string) *DiffInt {
	v := new(DiffInt)
	origin.Publish(name, v)
	return v
}

type DiffFloat struct {
	mu sync.RWMutex
	f  float64
	l  float64
	c  int
	b  bool
}

func (v *DiffFloat) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.b {
		return strconv.FormatFloat(v.f-v.l, 'g', -1, 64)
	} else {
		return "0.0"
	}
}

func (v *DiffFloat) Set(delta float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.l = v.f
	v.f = delta
	v.c += 1
	if !v.b && v.c > 1 {
		v.b = true
	}
}

func NewDiffFloat(name string) *DiffFloat {
	v := new(DiffFloat)
	origin.Publish(name, v)
	return v
}
