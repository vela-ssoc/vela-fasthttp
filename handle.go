package fasthttp

import (
	"errors"
	"github.com/valyala/fasthttp"
	cond "github.com/vela-ssoc/vela-cond"
	"github.com/vela-ssoc/vela-kit/lua"
	"sync/atomic"
)

const (
	VHANDLER handleType = iota + 1 //表示当前数据类型
	VHSTRING
	VHFUNC
)

var (
	emptyHandle      = errors.New("empty handle object")
	velaServerHeader = "vela-fasthttp v2.0"
)

type handleType int

type handle struct {
	//必须字段
	name  string
	mtime int64

	//业务字段
	count  uint32
	cnd    *cond.Cond
	method string

	//返回包处理
	code   int
	header *header
	hook   *lua.LFunction
	close  *lua.LFunction
	//返回结果
	body func(*RequestCtx) error

	//结束匹配
	eof bool
}

func newHandle(name string) *handle {
	return &handle{name: name, eof: false}
}

func (hd *handle) Close() error {
	if hd.close == nil {
		return nil
	}
	co := xEnv.Coroutine()
	defer xEnv.Free(co)
	return co.CallByParam(xEnv.P(hd.close))
}

func (hd *handle) MTime() int64 {
	return hd.mtime
}

func (hd *handle) Option() interface{} {
	return nil
}

func (hd *handle) Match(v string) bool {
	return hd.name == v
}

func (hd *handle) filter(ctx *RequestCtx) bool {
	if hd.cnd == nil {
		return true
	}

	return hd.cnd.Match(ctx)
}

func (hd *handle) do(co *lua.LState, ctx *RequestCtx, eof *bool) error {
	atomic.AddUint32(&hd.count, 1)

	if hd.filter(ctx) {
		goto set
	}

	//如果没有命中 eof 掉

	*eof = false
	return nil

set:
	//设置header
	ctx.Response.Header.Set("server", velaServerHeader)
	if hd.header != nil {
		hd.header.ForEach(func(key string, val string) {
			ctx.Response.Header.Set(key, val)
		})
	}

	if hd.code == 0 && hd.body == nil {
		return emptyHandle
	}

	//设置状态
	if hd.code != 0 {
		//设置状态码
		ctx.SetStatusCode(hd.code)
	}

	//设置响应体
	if hd.body != nil {
		return hd.body(ctx)
	}
	*eof = true

	return nil
}

type HandleChains struct {
	data []interface{}
	mask []handleType
	cap  int
}

func newHandleChains(cap int) *HandleChains {
	hc := &HandleChains{cap: 0}
	if cap == 0 {
		return hc
	}
	hc.cap = cap
	hc.data = make([]interface{}, cap)
	hc.mask = make([]handleType, cap)
	return hc
}

func (hc *HandleChains) Store(v interface{}, mask handleType, offset int) {
	if offset > hc.cap {
		xEnv.Errorf("vHandle overflower , cap:%d , got: %d", hc.cap, offset)
		return
	}

	hc.data[offset] = v
	hc.mask[offset] = mask
}

// 没有匹配的Handle代码
var notFoundBody = []byte("not found handle")

func (hc *HandleChains) notFound(ctx *RequestCtx) {
	ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
	ctx.Response.SetBody(notFoundBody)
}

func (hc *HandleChains) invalid(ctx *RequestCtx, body string) {
	ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
	ctx.Response.SetBodyString(body)
}

func (hc *HandleChains) do(ctx *RequestCtx, path string) { //path handle 查找路径
	if hc.cap == 0 {
		hc.notFound(ctx)
		return
	}

	var item *handle
	var err error
	var eof bool

	co := newLuaThread(ctx)
	for i := 0; i < hc.cap; i++ {
		switch hc.mask[i] {

		//字符串
		case VHSTRING:
			item, err = requireHandle(path, hc.data[i].(string))
			if err != nil {
				hc.invalid(ctx, err.Error())
				return
			}

			err = item.do(co, ctx, &eof)
			if err != nil {
				hc.invalid(ctx, err.Error())
				return
			}

		//处理对象
		case VHANDLER:
			item = hc.data[i].(*handle)
			err = item.do(co, ctx, &eof)
			if err != nil {
				hc.invalid(ctx, err.Error())
				return
			}

		case VHFUNC:
			cp := xEnv.P(hc.data[i].(*lua.LFunction))
			if e := co.CallByParam(cp); e != nil {
				hc.invalid(ctx, e.Error())
				return
			}

		//异常
		default:
			hc.invalid(ctx, "invalid handle type")
			return
		}

		if eof || checkLuaEof(ctx) {
			return
		}

	}
}
