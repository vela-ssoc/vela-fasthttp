package fasthttp

import (
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/kind"
	"github.com/vela-ssoc/vela-kit/lua"
	"os"
)

func (fss *server) vHost(L *lua.LState) int {
	n := L.GetTop()
	hostname := L.CheckString(1)
	var r *vRouter

	switch n {
	case 1:
		r = newRouter(L, lua.LNil)

	case 2:
		r = newRouter(L, L.CheckTable(2))

	default:
		L.RaiseError("invalid router options")
		return 0
	}

	fss.vhost.insert(hostname, r)
	xEnv.Errorf("add %s router succeed", hostname)
	L.Push(r)
	return 1
}

func (fss *server) formatL(L *lua.LState) int {
	codec := L.CheckString(1)
	if codec == "off" {
		return 0
	}

	if codec == "dict" {
		fss.cfg.access = PrepareDictJson(L)
		return 0
	}

	data := L.CheckString(2)
	fss.cfg.access = compileAccessFormat(codec, data)
	return 0
}

func (fss *server) addrL(L *lua.LState) int {
	fss.cfg.region = L.CheckString(1)
	return 0
}

func (fss *server) notFoundL(L *lua.LState) int {
	fss.cfg.notFound = checkHandleChains(L, 1)
	//fss.cfg.notFound = L.CheckString(1)
	return 0
}

func (fss *server) openFileL(L *lua.LState, filename string) int {
	fd, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModeAppend|os.ModePerm)
	if err != nil {
		L.RaiseError("open output file error %v", err)
		return 0
	}

	fss.cfg.fd = fd
	return 0
}

func (fss *server) outputL(L *lua.LState) int {
	val := L.Get(1)
	switch val.Type() {
	case lua.LTString:
		return fss.openFileL(L, val.String())
	case lua.LTVelaData:
		fss.cfg.output = lua.CheckWriter(val.(*lua.VelaData))

	default:
		L.RaiseError("invalid output sdk , got %s", val.Type().String())
	}
	return 0
}

func (fss *server) startL(L *lua.LState) int {
	xEnv.Start(L, fss).From(L.CodeVM()).Do()
	return 0
}

func (fss *server) dictL(L *lua.LState) int {
	n := L.GetTop()
	if n == 0 {
		return 0
	}

	var fileds []string

	for i := 2; i <= n; i++ {
		fileds = append(fileds, L.CheckString(i))
	}

	fss.cfg.access = func(ctx *RequestCtx) []byte {
		enc := kind.NewJsonEncoder()
		enc.Tab("")
		for _, key := range fileds {
			enc.KV(key, k2v(ctx, key).String())
		}
		enc.End("}")
		return enc.Bytes()
	}

	return 0
}

func (fss *server) varL(L *lua.LState) int {
	L.Callback(func(val lua.LValue) (stop bool) {
		k, v := auxlib.ParamLValue(val.String())
		if v == nil {
			return true
		}
		fss.cfg.variables[k] = v.String()
		return
	})
	return 0
}

func (fss *server) Index(L *lua.LState, key string) lua.LValue {
	switch key {
	case "vhost":
		return L.NewFunction(fss.vHost)
	case "start":
		return L.NewFunction(fss.startL)
	case "format":
		return L.NewFunction(fss.formatL)
	case "dict":
		return L.NewFunction(fss.dictL)
	case "format_map":
		return L.NewFunction(fss.formatL)
	case "addr":
		return L.NewFunction(fss.addrL)
	case "to":
		return L.NewFunction(fss.outputL)
	case "var":
		return lua.NewFunction(fss.varL)

	case "r":
		return fss.cfg.r

	case "default":
		return L.NewFunction(fss.notFoundL)

	}

	return lua.LNil
}
