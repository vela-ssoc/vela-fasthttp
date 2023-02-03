package fasthttp

import (
	"errors"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"os"
)

var (
	defaultAccessJsonFormat = "[${time}] - [${remote_port}] - ${server_addr}:${server_port} ${remote_addr} " +
		"${method} [${scheme}] [${host}] ${uri} ${query} ${ua} ${referer} ${status} ${size} ${region_city}"
)

type config struct {
	//基础配置
	name      string
	bind      auxlib.URL // tcp://0.0.0.0:9090?read_timeout=100&idle_timeout=100
	router    string
	handler   string
	keepalive string
	reuseport string
	daemon    string
	region    string
	notFound  *HandleChains
	variables map[string]string

	//下面对象配置
	fd     *os.File
	output lua.Writer
	access func(*RequestCtx) []byte

	r     *vRouter
	co    *lua.LState
	debug bool
}

func newConfig(L *lua.LState) *config {
	tab := L.CheckTable(1)
	cnn := &conversion{}
	cnn.pretreatment(defaultAccessJsonFormat)

	cfg := &config{
		router:    xEnv.Prefix() + "/www/vhost",
		handler:   xEnv.Prefix() + "/www/handle",
		access:    cnn.Line,
		r:         newRouter(L, lua.LNil),
		co:        xEnv.Clone(L),
		variables: make(map[string]string),
	}

	tab.Range(func(key string, val lua.LValue) {
		switch key {
		case "name":
			cfg.name = val.String()
		case "daemon":
			cfg.daemon = val.String()
		case "reuseport":
			cfg.reuseport = val.String()
		case "keepalive":
			cfg.keepalive = val.String()
		case "bind":
			cfg.bind = auxlib.CheckURL(val, L)
		case "output":
			cfg.output = checkOutputSdk(L, val)

		default:
			L.RaiseError("invalid web config %s field", key)
			return
		}
	})

	if e := cfg.verify(); e != nil {
		L.RaiseError("%v", e)
		return nil
	}
	return cfg
}

func (cfg *config) verify() error {
	if cfg.name == "" {
		return errors.New("invalid name")
	}

	return nil
}
