// +build profile

package server

import (
	"expvar"
	"fmt"
	myexpvar "github.com/chobie/momonga/expvar"
	"github.com/cloudfoundry/gosigar"
	"github.com/influxdb/influxdb/client"
	"github.com/chobie/momonga/util"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var result map[string]interface{}
var curkey string
var ignore []string = []string{"memstats.PauseNs", "memstats.BySize"}

func flatten(xv reflect.Value, key string, r interface{}) {
	for i := 0; i < len(ignore); i++ {
		if strings.Contains(key, ignore[i]) {
			return
		}
	}

	switch xv.Kind() {
	case reflect.Map:
		result[key] = r
	case reflect.Slice, reflect.Array:
		num := xv.Len()
		for i := 0; i < num; i++ {
			c := xv.Index(i)
			if c.Kind() == reflect.Struct {
				flatten(c, fmt.Sprintf("%s.%d", key, i), c.Interface())
			} else {
				result[fmt.Sprintf("%s.%d", key, i)] = c.String()
			}
		}
	case reflect.Struct:
		num := xv.NumField()
		ty := xv.Type()
		for i := 0; i < num; i++ {
			y := xv.Field(i)
			if y.Kind() == reflect.Struct {
				flatten(y, fmt.Sprintf("%s.%s", key, ty.Field(i).Name), y.Interface())
			} else if y.Kind() == reflect.Slice {
				flatten(y, fmt.Sprintf("%s.%s", key, ty.Field(i).Name), y.Interface())
			} else if y.Kind() == reflect.Array {
				flatten(y, fmt.Sprintf("%s.%s", key, ty.Field(i).Name), y.Interface())
			} else {
				result[fmt.Sprintf("%s.%s", key, ty.Field(i).Name)] = y.Interface()
			}
		}
	default:
		panic(fmt.Sprintf("%d not supported\n", xv.Kind()))
	}
}

func cb(kv expvar.KeyValue) {
	m := reflect.ValueOf(kv.Value).MethodByName("Do")
	if m.IsValid() {
		curkey = kv.Key
		m.Call([]reflect.Value{reflect.ValueOf(cb)})
		curkey = ""
	} else {
		var key string

		if curkey != "" {
			key = fmt.Sprintf("%s.%s", curkey, kv.Key)
		} else {
			key = kv.Key
		}
		if f, ok := kv.Value.(expvar.Func); ok {
			r := f()
			xv := reflect.ValueOf(r)
			flatten(xv, key, r)
		} else if f, ok := kv.Value.(*expvar.Int); ok {
			a, _ := strconv.ParseInt(f.String(), 10, 64)
			result[key] = a
		} else if f, ok := kv.Value.(*expvar.Float); ok {
			a, _ := strconv.ParseFloat(f.String(), 64)
			result[key] = a
		} else if f, ok := kv.Value.(*myexpvar.DiffFloat); ok {
			a, _ := strconv.ParseFloat(f.String(), 64)
			result[key] = a
		} else if f, ok := kv.Value.(*myexpvar.DiffInt); ok {
			a, _ := strconv.ParseInt(f.String(), 10, 64)
			result[key] = a
		} else {
			result[key] = kv.Value.String()
		}
	}
}

func init() {
	result = make(map[string]interface{})

	go func() {
		lastCpu := sigar.Cpu{}
		cpu := sigar.Cpu{}
		lastCpu.Get()

		c, _ := client.NewClient(&client.ClientConfig{
			Database: "test",
		})

		for {
			Metrics.NumGoroutine.Set(int64(runtime.NumGoroutine()))
			Metrics.NumCgoCall.Set(int64(runtime.NumGoroutine()))
			Metrics.Uptime.Set(time.Now().Unix())

			Metrics.MessageSentPerSec.Set(util.GetIntValue(Metrics.System.Broker.Messages.Sent))
			if util.GetIntValue(Metrics.System.Broker.Clients.Connected) > 0 {
				Metrics.GoroutinePerConn.Set(float64(util.GetIntValue(Metrics.NumGoroutine) / util.GetIntValue(Metrics.System.Broker.Clients.Connected)))
			}

			mem := sigar.Mem{}
			mem.Get()

			Metrics.MemFree.Set(int64(mem.Free))
			Metrics.MemUsed.Set(int64(mem.Used))
			Metrics.MemActualFree.Set(int64(mem.ActualFree))
			Metrics.MemActualUsed.Set(int64(mem.ActualUsed))
			Metrics.MemTotal.Set(int64(mem.Total))

			load := sigar.LoadAverage{}
			load.Get()
			Metrics.LoadOne.Set(float64(load.One))
			Metrics.LoadFive.Set(float64(load.Five))
			Metrics.LoadFifteen.Set(float64(load.Fifteen))

			cpu.Get()

			Metrics.CpuUser.Set(float64(cpu.User - lastCpu.User))
			Metrics.CpuNice.Set(float64(cpu.Nice - lastCpu.Nice))
			Metrics.CpuSys.Set(float64(cpu.Sys - lastCpu.Sys))
			Metrics.CpuIdle.Set(float64(cpu.Idle - lastCpu.Idle))
			Metrics.CpuWait.Set(float64(cpu.Wait - lastCpu.Wait))
			Metrics.CpuIrq.Set(float64(cpu.Irq - lastCpu.Irq))
			Metrics.CpuSoftIrq.Set(float64(cpu.SoftIrq - lastCpu.SoftIrq))
			Metrics.CpuStolen.Set(float64(cpu.Stolen - lastCpu.Stolen))
			Metrics.CpuTotal.Set(float64(cpu.Total() - lastCpu.Total()))

			expvar.Do(cb)
			w := &client.Series{
				Name: "test",
			}
			p := []interface{}{}
			for k, v := range result {
				w.Columns = append(w.Columns, k)
				p = append(p, v)
			}
			w.Points = [][]interface{}{p}
			e := c.WriteSeries([]*client.Series{w})

			if e != nil {
				fmt.Printf("error: %s", e)
			}

			lastCpu = cpu
			time.Sleep(time.Second)
		}
	}()
}
