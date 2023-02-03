package fasthttp

import (
	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"github.com/vela-ssoc/vela-kit/lua"
)

type RequestCtx = fasthttp.RequestCtx

type vRouter struct {

	//获取名称
	name string

	//上次修改时间
	mtime int64 //时间

	//匹配
	match  func(string) bool
	method string

	accessOff string
	access    func(ctx *RequestCtx) []byte
	region    string
	output    lua.Writer
	variables map[string]string

	//handler处理脚本路径
	handler string

	close       *lua.LFunction
	interceptor *lua.LFunction

	//缓存路由
	r *router.Router
}

func (r *vRouter) AccessLogOff() bool {
	return r.accessOff == "off"
}

func (r *vRouter) Close() error {
	if r.close == nil {
		return nil
	}
	co := xEnv.Coroutine()
	defer xEnv.Free(co)
	cp := xEnv.P(r.close)
	return co.CallByParam(cp)
}

func (r *vRouter) MTime() int64 {
	return r.mtime
}

func (r *vRouter) Option() interface{} {
	return r.handler
}

func (r *vRouter) Name() string {
	return r.name
}

func (r *vRouter) Match(v string) bool {
	return r.name == v
}

func (r *vRouter) do(ctx *RequestCtx) {
	r.r.Handler(ctx)

	if r.interceptor == nil {
		return
	}

	co := newLuaThread(ctx)
	cp := xEnv.P(r.interceptor)

	err := co.CallByParam(cp)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString(err.Error())
		return
	}
}

func newRouter(co *lua.LState, tab lua.LValue) *vRouter {
	r := router.New()
	r.PanicHandler = panicHandler

	v := &vRouter{r: r, variables: map[string]string{}}

	if tab.Type() != lua.LTTable {
		return v
	}

	tab.(*lua.LTable).Range(func(key string, val lua.LValue) {
		v.NewIndex(co, key, val)
	})
	return v
}

func newLuaRouter(co *lua.LState) int {
	r := newRouter(co, co.CheckAny(1))
	ctx := co.WithValue(router_context_key, r)
	co.SetContext(ctx)
	co.Push(r)
	return 1
}
