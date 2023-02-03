package fasthttp

import (
	"github.com/vela-ssoc/vela-kit/lua"
)

func vhHelper(L *lua.LState) *vhost {
	tab := L.CheckTable(1)
	app := &vhost{}

	tab.Range(func(key string, val lua.LValue) {
		app.NewIndex(L, key, val)
	})

	if e := app.verify(L); e != nil {
		L.RaiseError("vhost %s", e)
		return nil
	}

	app.r = newRouter(L, tab)
	app.code = L.CodeVM()
	app.V(lua.VTInit, vhostTypeof)
	return app

}

func (v *vhost) fssL(L *lua.LState, val lua.LValue) {
	if val.Type() != lua.LTVelaData {
		L.RaiseError(" vhost server must  web server  , got %s", val.Type().String())
		return
	}

	var ok bool
	v.fss, ok = val.(*lua.VelaData).Data.(*server)
	if !ok {
		L.RaiseError(" invalid vhost serve")
		return
	}
}

func (v *vhost) hostL(L *lua.LState, val lua.LValue) {
	v.host = val.String()
}

func (v *vhost) startL(L *lua.LState) int {
	xEnv.Start(L, v).From(L.CodeVM()).Err(func(err error) {
		L.RaiseError("%s %s start fail", v.name, v.host)
	}).Do()
	return 0
}

func (v *vhost) Index(L *lua.LState, key string) lua.LValue {

	if key == "start" {
		return lua.NewFunction(v.startL)
	}

	return v.r.Index(L, key)
}

func (v *vhost) NewIndex(L *lua.LState, key string, val lua.LValue) {

	switch key {
	case "name":
		v.name = val.String()
	case "server":
		v.fssL(L, val)
	case "host":
		v.hostL(L, val)
	}

}
