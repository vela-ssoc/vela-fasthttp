package fasthttp

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
)

func (r *vRouter) String() string                         { return fmt.Sprintf("fasthttp.router %p", r) }
func (r *vRouter) Type() lua.LValueType                   { return lua.LTObject }
func (r *vRouter) AssertFloat64() (float64, bool)         { return 0, false }
func (r *vRouter) AssertString() (string, bool)           { return "", false }
func (r *vRouter) AssertFunction() (*lua.LFunction, bool) { return nil, false }
func (r *vRouter) Peek() lua.LValue                       { return r }

func (r *vRouter) handleIndexFn(L *lua.LState, method string) *lua.LFunction {
	fn := func(co *lua.LState) int {
		path := co.CheckString(1)
		chains := checkHandleChains(co, 1)
		r.r.Handle(method, path, func(ctx *RequestCtx) { chains.do(ctx, r.handler) })
		return 0
	}
	return L.NewFunction(fn)
}

func (r *vRouter) anyIndexFn(L *lua.LState) *lua.LFunction {
	fn := func(co *lua.LState) int {
		path := co.CheckString(1)
		chains := checkHandleChains(co, 1)
		r.r.ANY(path, func(ctx *fasthttp.RequestCtx) { chains.do(ctx, r.handler) })
		return 0
	}

	return L.NewFunction(fn)
}

func (r *vRouter) notFoundIndexFn(L *lua.LState) *lua.LFunction {
	fn := func(co *lua.LState) int {
		chains := checkHandleChains(co, 1)
		r.r.NotFound = func(ctx *fasthttp.RequestCtx) { chains.do(ctx, r.handler) }
		return 0
	}
	return L.NewFunction(fn)
}

func (r *vRouter) fileIndexFn(L *lua.LState) *lua.LFunction {
	fn := func(vm *lua.LState) (ret int) {
		n := vm.GetTop()
		path := vm.CheckString(1)
		root := vm.CheckString(2)
		fs := &fasthttp.FS{
			Root:               root,
			IndexNames:         []string{"index.html"},
			GenerateIndexPages: true,
			AcceptByteRange:    true,
		}

		if n == 3 {
			cp := xEnv.P(vm.CheckFunction(3))

			fs.PathRewrite = func(ctx *fasthttp.RequestCtx) []byte {
				co := newLuaThread(ctx)
				err := co.CallByParam(cp)
				if err != nil {
					xEnv.Errorf("%v", err)
					return ctx.Path()
				}

				if lv := co.Get(-1); lv.Type() == lua.LTString {
					return lua.S2B(lv.String())
				}

				return ctx.Path()
			}
		}

		r.r.ServeFilesCustom(path, fs)
		return
	}

	return L.NewFunction(fn)
}

func (r *vRouter) call(co *lua.LState, hook *lua.LFunction) {
	if hook == nil {
		return
	}
	err := co.CallByParam(xEnv.P(hook))
	if err != nil {
		xEnv.Errorf("http hook call error: %v", err)
	}
}

func (r *vRouter) varL(L *lua.LState) int {
	L.Callback(func(val lua.LValue) (stop bool) {
		k, v := auxlib.ParamLValue(val.String())
		if v == nil {
			return true
		}
		r.variables[k] = v.String()
		return
	})
	return 0
}

func (r *vRouter) Index(L *lua.LState, key string) lua.LValue {
	switch key {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE":
		return r.handleIndexFn(L, key)

	case "FILE", "file":
		return r.fileIndexFn(L)

	case "ANY", "any":
		return r.anyIndexFn(L)

	case "not_found", "default":
		return r.notFoundIndexFn(L)

	case "var":
		return lua.NewFunction(r.varL)
	case "addr":
		return lua.NewFunction(r.addrL)

	case "format":
		return lua.NewFunction(r.formatL)

	case "to":
		return lua.NewFunction(r.outputL)

	case "on_exit":
		return lua.NewFunction(r.onExitL)

	case "interceptor":
		return lua.NewFunction(r.interceptorL)
	}

	return lua.LNil
}

func (r *vRouter) interceptorL(L *lua.LState) int {
	r.interceptor = L.IsFunc(1)
	return 0
}

func (r *vRouter) onExitL(L *lua.LState) int {
	r.close = L.IsFunc(1)
	return 0
}

func (r *vRouter) outputL(L *lua.LState) int {
	r.output = lua.CheckWriter(L.CheckVelaData(1))
	return 0
}

func (r *vRouter) addrL(L *lua.LState) int {
	r.region = L.CheckString(1)
	return 0
}

func (r *vRouter) formatL(L *lua.LState) int {
	codec := L.CheckString(1)
	if codec == "dict" {
		r.access = PrepareDictJson(L)
		return 0
	}

	data := L.CheckString(2)
	r.access = compileAccessFormat(codec, data)
	return 1
}

func (r *vRouter) NewIndex(L *lua.LState, key string, val lua.LValue) {

	switch key {
	case "method":
		r.method = val.String()

	case "interceptor":
		r.interceptor = lua.IsFunc(val)
	}

}
