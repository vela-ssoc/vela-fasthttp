package fasthttp

import (
	"errors"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"reflect"
	"time"
)

var vhostTypeof = reflect.TypeOf((*vhost)(nil)).String()

type vhost struct {
	lua.SuperVelaData

	code string
	name string
	host string

	r   *vRouter
	fss *server
}

func (v *vhost) Name() string {
	return v.name
}

func (v *vhost) Start() error {
	v.fss.vhost.insert(v.host, v.r)
	return nil
}

func (v *vhost) Close() error {
	v.fss.vhost.clear(v.name)
	v.V(lua.VTClose, time.Now())
	return nil
}

func (v *vhost) verify(L *lua.LState) error {
	if L.CodeVM() == "" {
		return errors.New("vhost not allow thread")
	}

	if v.fss == nil {
		return errors.New("vhost not found web server")
	}

	if v.host == "" {
		return errors.New("vhost invalid hostname")
	}

	if e := auxlib.Name(v.name); e != nil {
		return e
	}

	return nil
}

func newLuaHost(L *lua.LState) int {
	app := vhHelper(L)
	proc := L.NewVelaData(app.name, vhostTypeof)
	if proc.IsNil() {
		proc.Set(app)
		L.Push(proc)
		return 1
	}

	old := proc.Data.(*vhost)

	//如果切换web服务中心
	if old.fss.Name() != app.fss.Name() {
		old.fss.vhost.clear(app.Name())
		xEnv.Errorf("%s web %s vhost clear from %s", old.fss.Name(), old.Name())
	}

	old.fss = app.fss
	old.r = app.r
	L.Push(proc)
	return 1
}
