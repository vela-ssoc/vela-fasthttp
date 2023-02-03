package fasthttp

import (
	"github.com/vela-ssoc/vela-kit/vela"
	"reflect"
	"sync"
	"time"
)

var (
	once       sync.Once
	handlePool *pool
	routerPool *pool
	xEnv       vela.Environment
	typeof     = reflect.TypeOf((*server)(nil)).String()
)

const (
	web_conf_key       = "__web_cfg__"
	usr_addr_key       = "__usr_addr__"
	web_context_key    = "__web_context__"
	router_context_key = "__web_router__"
	thread_uv_key      = "__thread_co__"
	eof_uv_key         = "__handle_eof__"
	debug_uv_key       = "__debug__"
)

func init() {
	once.Do(func() {
		handlePool = newPool()
		routerPool = newPool()
		go func() {
			tk := time.NewTicker(time.Second)
			defer tk.Stop()

			for range tk.C {
				routerPool.sync(compileRouter)
				handlePool.sync(compileHandle)
			}
		}()
	})
}
