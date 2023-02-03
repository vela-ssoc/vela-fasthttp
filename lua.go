package fasthttp

import (
	"github.com/vela-ssoc/vela-kit/lua"
	"github.com/vela-ssoc/vela-kit/vela"
)

func newLuaServer(L *lua.LState) int {
	cfg := newConfig(L)
	proc := L.NewVelaData(cfg.name, typeof)
	if proc.IsNil() {
		proc.Set(newServer(cfg))
	} else {
		proc.Data.(*server).cfg = cfg
	}

	L.Push(proc)
	return 1
}

func WithEnv(env vela.Environment) {
	xEnv = env
	kv := lua.NewUserKV()
	ctx := newContext()
	kv.Set("context", ctx)
	kv.Set("ctx", ctx)
	kv.Set("h", lua.NewFunction(newLuaHandle))
	kv.Set("handle", lua.NewFunction(newLuaHandle))
	kv.Set("router", lua.NewFunction(newLuaRouter))
	kv.Set("header", lua.NewFunction(newLuaHeader))
	kv.Set("clone", lua.NewFunction(newLuaCloneL))
	kv.Set("redirect", lua.NewFunction(newLuaRedirectL))
	kv.Set("H", lua.NewFunction(newLuaHeader))
	kv.Set("vhost", lua.NewFunction(newLuaHost))

	env.Global("web",
		lua.NewExport("vela.web.export",
			lua.WithTable(kv),
			lua.WithFunc(newLuaServer)))
}
