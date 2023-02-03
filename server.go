package fasthttp

import (
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"net"
	"os"
	"time"
)

type server struct {
	lua.SuperVelaData

	//基础配置
	cfg *config

	//监听
	ln net.Listener

	//中间对象
	fs *fasthttp.Server

	vhost *pool
}

func newServer(cfg *config) *server {
	cnn := &conversion{}
	cnn.pretreatment(defaultAccessJsonFormat)
	srv := &server{cfg: cfg, vhost: newPool()}
	srv.V(lua.VTInit, typeof)
	return srv
}

func (fss *server) Name() string {
	return fss.cfg.name
}

func (fss *server) Close() error {
	if fss.IsClose() {
		return nil
	}

	if fss.cfg.fd != nil {
		_ = fss.cfg.fd.Close()
		fss.cfg.fd = nil
	}

	xEnv.Errorf("%s web vhost clear", fss.Name())

	routerPool.clear(fss.cfg.router)
	handlePool.clear(fss.cfg.handler)
	if fss.fs == nil {
		goto done
	}

	if e := fss.fs.Shutdown(); e != nil {
		xEnv.Errorf("%s web close error %v", fss.Name(), e)
		fss.V(lua.VTPanic)
		return e
	}

done:
	fss.V(lua.VTClose)
	return nil
}

func (fss *server) Listen() (net.Listener, error) {
	var network, address string

	network = fss.cfg.bind.Scheme()
	switch network {
	case "unix", "pipe":
		address = fss.cfg.bind.Path()
	default:
		address = fss.cfg.bind.Host()

	}

	if fss.cfg.reuseport == "on" {
		return reuseport.Listen(network, address)
	}

	return net.Listen(network, address)
}

func (fss *server) keepalive() bool {
	if fss.cfg.keepalive == "on" {
		return true
	}
	return false
}

func (fss *server) notFoundBody(ctx *RequestCtx) {
	ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
	ctx.Response.SetBodyString("not found")
}

func (fss *server) notFound(ctx *RequestCtx) {
	if fss.cfg.r == nil {
		fss.notFoundBody(ctx)
		return
	}

	fss.cfg.r.do(ctx)
}

func (fss *server) invalid(ctx *RequestCtx, err error) {
	ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
	ctx.Response.SetBodyString(err.Error())
}

func (fss *server) Region(r *vRouter, ctx *RequestCtx) {
	region := fss.cfg.region
	if r == nil {
		goto done
	}

	if r.region != "" {
		region = r.region
	}

done:
	if region == "" {
		return
	}

	ip := k2v(ctx, region).String()
	if auxlib.Ipv6(ip) || auxlib.Ipv4(ip) {
		ctx.SetUserValue(usr_addr_key, ip)
	}

	if !auxlib.Ipv4(ip) {
		return
	}

	info, err := xEnv.Region(ip)
	if err != nil {
		xEnv.Errorf("%v", err)
		return
	}

	ctx.SetUserValue("region", info)
	return

}

func (fss *server) Log(r *vRouter, ctx *RequestCtx) {
	fn := fss.cfg.access
	sdk := fss.cfg.output

	if r == nil {
		goto done
	}

	//关闭
	if r.AccessLogOff() {
		return
	}

	if r.access != nil {
		fn = r.access
	}

	//获取每个域名的请求
	if r.output != nil {
		sdk = r.output
	}

done:
	if fn == nil {
		return
	}

	if sdk != nil {
		sdk.Write(fn(ctx))
		return
	}

	if fss.cfg.fd != nil {
		fss.cfg.fd.Write(fn(ctx))
		fss.cfg.fd.Write([]byte("\n"))
	}
}

//编译
//func (fss *server) compile() {
//	fss.accessFn = compileAccessFormat(fss.cfg.accessFormat, fss.cfg.accessEncode)
//}

func (fss *server) require(ctx *RequestCtx) (*vRouter, error) {
	host := lua.B2S(ctx.Request.Header.Host())

	item := fss.vhost.Get(host)
	if item != nil {
		return item.val.(*vRouter), nil
	}

	return requireRouter(fss.cfg.router, fss.cfg.handler, host)
}

func (fss *server) setUserValue(r *vRouter, ctx *RequestCtx) {
	if r != nil {
		setUserValueByMap(r.variables, ctx)
	}

	setUserValueByMap(fss.cfg.variables, ctx)
}

func (fss *server) Handler(ctx *RequestCtx) {
	ctx.SetUserValue(web_conf_key, fss.cfg)

	r, err := fss.require(ctx)
	//是否获取IP地址位置信息
	fss.Region(r, ctx)

	fss.setUserValue(r, ctx)

	if err != nil {
		if os.IsNotExist(err) {
			fss.notFound(ctx)
			goto done
		}

		fss.invalid(ctx, err)
		goto done
	}

	r.do(ctx)

done:
	fss.Log(r, ctx)

	//释放co
	freeLuaThread(ctx)
}

func (fss *server) Start() error {
	xEnv.Errorf("%s fasthttp start ...", fss.Name())

	ln, err := fss.Listen()
	if err != nil {
		return err
	}

	fss.fs = &fasthttp.Server{
		Handler:         fss.Handler,
		TCPKeepalive:    fss.keepalive(),
		ReadTimeout:     time.Duration(fss.cfg.bind.Int("read_timeout")) * time.Second,
		IdleTimeout:     time.Duration(fss.cfg.bind.Int("idle_timeout")) * time.Second,
		CloseOnShutdown: true,
	}
	fss.ln = ln
	go func() {
		err = fss.fs.Serve(ln)
	}()

	time.Sleep(500 * time.Millisecond)
	return err
}
