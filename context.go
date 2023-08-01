package fasthttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/kind"
	"github.com/vela-ssoc/vela-kit/lua"
	"github.com/vela-ssoc/vela-kit/vela"
	"net"
	"net/http"
	"strings"
	"time"
)

type fsContext struct {
	sayRaw  *lua.LFunction
	sayJson *lua.LFunction
	sayFile *lua.LFunction
	say     *lua.LFunction
	format  *lua.LFunction
	append  *lua.LFunction
	exit    *lua.LFunction
	eof     *lua.LFunction
	rdt     *lua.LFunction //redirect
	rph     *lua.LFunction //request header
	rqh     *lua.LFunction
	try     *lua.LFunction
	bind    *lua.LFunction
	clone   *lua.LFunction

	//meta lua.UserKV
}

func (fsc *fsContext) String() string                         { return fmt.Sprintf("fasthttp.context %p", fsc) }
func (fsc *fsContext) Type() lua.LValueType                   { return lua.LTObject }
func (fsc *fsContext) AssertFloat64() (float64, bool)         { return 0, false }
func (fsc *fsContext) AssertString() (string, bool)           { return "", false }
func (fsc *fsContext) AssertFunction() (*lua.LFunction, bool) { return nil, false }
func (fsc *fsContext) Peek() lua.LValue                       { return fsc }

func newContext() *fsContext {
	return &fsContext{
		sayJson: lua.NewFunction(sayJsonL),
		sayRaw:  lua.NewFunction(sayRawL),
		sayFile: lua.NewFunction(sayFileL),
		append:  lua.NewFunction(appendL),
		say:     lua.NewFunction(fsSay),
		format:  lua.NewFunction(fsFormat),
		exit:    lua.NewFunction(exitL),
		eof:     lua.NewFunction(eofL),
		rdt:     lua.NewFunction(fsRedirect),
		rph:     lua.NewFunction(rphL),
		rqh:     lua.NewFunction(rqhL),
		try:     lua.NewFunction(tryL),
		bind:    lua.NewFunction(luaBodyBind),
		clone:   lua.NewFunction(cloneL),
	}
}

func xPort(addr net.Addr) int {
	x, ok := addr.(*net.TCPAddr)
	if !ok {
		return 0
	}
	return x.Port
}

func addr(ctx *RequestCtx) string {
	uv := ctx.UserValue(usr_addr_key)

	ip, ok := uv.(string)
	if ok {
		return ip
	}

	return ctx.RemoteIP().String()
}

func regionCityId(ctx *RequestCtx) int {
	uv := ctx.UserValue("region_city")
	v, ok := uv.(int)
	if ok {
		return v
	}
	return 0
}

func regionRaw(ctx *RequestCtx) []byte {
	uv := ctx.UserValue("region")
	if uv == nil {
		return byteNull
	}

	v, ok := uv.(*vela.IPv4Info)
	if ok {
		return v.Byte()
	}

	return byteNull
}

func fsSay(co *lua.LState) int {
	n := co.GetTop()
	if n == 0 {
		return 0
	}

	ctx := checkRequestCtx(co)
	var buf bytes.Buffer

	for i := 1; i <= n; i++ {
		buf.WriteString(co.Get(i).String())
	}
	ctx.Response.SetBodyRaw(buf.Bytes())
	return 0
}

func fsFormat(co *lua.LState) int {
	n := co.GetTop()
	if n == 0 {
		return 0
	}

	ctx := checkRequestCtx(co)
	body := auxlib.Format(co, 0)
	ctx.Response.SetBodyRaw(lua.S2B(body))
	return 0
}

type ToJson interface {
	ToJson() ([]byte, error)
}

func sayJsonL(co *lua.LState) int {
	ctx := checkRequestCtx(co)
	lv := co.CheckAny(1)
	chunk, err := json.Marshal(lv)
	if err != nil {
		ctx.Error(err.Error(), 500)
		return 0
	}

	ctx.SetBody(chunk)
	return 0
}

func sayRawL(co *lua.LState) int {
	n := co.GetTop()
	if n == 0 {
		return 0
	}

	ctx := checkRequestCtx(co)
	var buf bytes.Buffer
	for i := 1; i <= n; i++ {
		buf.Write(lua.S2B(co.CheckString(i)))
	}
	ctx.Response.SetBodyRaw(buf.Bytes())
	return 0
}

func sayFileL(co *lua.LState) int {
	ctx := checkRequestCtx(co)
	path := co.CheckString(1)
	ctx.SendFile(path)
	return 0
}

func appendL(co *lua.LState) int {
	n := co.GetTop()
	if n == 0 {
		return 0
	}

	data := make([]string, n)
	ctx := checkRequestCtx(co)
	for i := 1; i <= n; i++ {
		data[i-1] = co.CheckString(i)
	}
	ctx.Response.AppendBody(lua.S2B(strings.Join(data, "")))
	return 0
}

func exitL(co *lua.LState) int {
	code := co.CheckInt(1)
	ctx := checkRequestCtx(co)
	ctx.Response.SetStatusCode(code)
	ctx.SetUserValue(eof_uv_key, true)
	return 0
}

func eofL(co *lua.LState) int {
	ctx := checkRequestCtx(co)
	ctx.SetUserValue(eof_uv_key, true)
	return 0
}

func cloneL(co *lua.LState) int {
	ctx := checkRequestCtx(co)
	url := co.CheckString(1)

	rsp, err := http.Get(url)
	if err != nil {
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.SetBodyString("clone fail")
		return 0
	}

	for key, val := range rsp.Header {
		for _, iv := range val {
			ctx.Response.Header.Set(key, iv)
		}
	}

	size := rsp.ContentLength
	ctx.SetBodyStream(rsp.Body, int(size))
	return 0
}

func tryL(co *lua.LState) int {
	n := co.GetTop()
	if n == 0 {
		co.RaiseError("invalid")
		return 0
	}

	data := make([]interface{}, n)
	format := make([]string, n)
	for i := 1; i <= n; i++ {
		format[i-1] = "%v "
		data[i-1] = co.CheckAny(i)
	}
	co.RaiseError(strings.Join(format, " "), data...)
	return 0
}

func fsHeaderHelper(co *lua.LState, resp bool) int {
	n := co.GetTop()
	if n == 0 {
		return 0
	}

	if n%2 != 0 {
		co.RaiseError("#args % 2 != 0")
		return 0
	}

	ctx := checkRequestCtx(co)

	for i := 0; i < n; {
		key := co.CheckString(i + 1)
		val := co.CheckString(i + 2)
		i += 2
		if resp {
			ctx.Response.Header.Set(key, val)
		} else {
			ctx.Request.Header.Set(key, val)
		}
	}

	return 0
}

func k2v(ctx *RequestCtx, key string) lua.LValue {
	switch key {
	//主机头
	case "host":
		return lua.B2L(ctx.Host())
	case "addr":
		return lua.S2L(addr(ctx))
	case "scheme":
		return lua.B2L(ctx.URI().Scheme())

	case "method":
		return lua.B2L(ctx.Method())

	//浏览器标识
	case "ua":
		return lua.B2L(ctx.UserAgent())

	//客户端信息
	case "remote_addr":
		return lua.S2L(ctx.RemoteIP().String())
	case "remote_port":
		return lua.LInt(xPort(ctx.RemoteAddr()))

	//服务器信息
	case "server_addr":
		return lua.S2L(ctx.LocalIP().String())
	case "server_port":
		return lua.LInt(xPort(ctx.LocalAddr()))

	case "time":
		return lua.S2L(time.Now().Format("2006-01-02 13:04:05.00"))

	//请求信息
	case "uri":
		return lua.S2L(lua.B2S(ctx.URI().Path()))
	case "full_uri":
		return lua.S2L(ctx.URI().String())

	case "query":
		return lua.S2L(lua.B2S(ctx.URI().QueryString()))
	case "referer":
		return lua.S2L(lua.B2S(ctx.Request.Header.Peek("referer")))

	case "content_length":
		size := uint(ctx.Request.Header.ContentLength())
		return lua.LInt(size)

	case "size":
		raw := ctx.Request.Header.RawHeaders()
		full := ctx.URI().FullURI()
		return lua.LInt(len(raw) + len(full))

	case "content_type":
		return lua.S2L(lua.B2S(ctx.Request.Header.ContentType()))

	//返回结果
	case "status":
		return lua.LInt(ctx.Response.StatusCode())
	case "sent":
		return lua.LInt(ctx.Response.Header.ContentLength())
	//返回完整的数据
	case "region_raw":
		return lua.B2L(regionRaw(ctx))
	case "header_raw", "header":
		return lua.B2L(ctx.Request.Header.RawHeaders())
	case "cookie_raw", "cookie":
		return lua.B2L(ctx.Request.Header.Peek("cookie"))
	case "body_raw":
		return lua.B2L(ctx.Request.Body())

	default:
		switch {
		case strings.HasPrefix(key, "arg_"):
			return lua.B2L(ctx.QueryArgs().Peek(key[4:]))

		case strings.HasPrefix(key, "post_"):
			return lua.B2L(ctx.PostArgs().Peek(key[5:]))

		case strings.HasPrefix(key, "http_"):
			item := lua.S2B(key[5:])
			for i := 0; i < len(item); i++ {
				if item[i] == '_' {
					item[i] = '-'
				}
			}
			return lua.B2L(ctx.Request.Header.Peek(lua.B2S(item)))

		case strings.HasPrefix(key, "cookie_"):
			return lua.B2L(ctx.Request.Header.Cookie(key[7:]))

		case strings.HasPrefix(key, "region_"):
			uv := ctx.UserValue("region")
			if uv == nil {
				return lua.LNil
			}

			info, ok := uv.(vela.IPv4Info)
			if !ok {
				return lua.LNil
			}

			switch key[7:] {
			case "city":
				return lua.B2L(info.City())
			case "city_id":
				return lua.LInt(info.CityID())
			case "province":
				return lua.B2L(info.Province())
			case "region":
				return lua.B2L(info.Region())
			case "isp":
				return lua.B2L(info.ISP())
			default:
				return lua.LNil
			}

		case strings.HasPrefix(key, "param_"):
			uv := ctx.UserValue(key[6:])
			switch s := uv.(type) {
			case lua.LValue:
				return s
			case string:
				return lua.S2L(s)
			case int:
				return lua.LNumber(s)
			case interface{ String() string }:
				return lua.S2L(s.String())
			case interface{ Byte() []byte }:
				return lua.B2L(s.Byte())
			default:
				return lua.LNil
			}
		}
	}

	uv := ctx.UserValue(key)
	if uv == nil {
		return lua.LNil
	}

	val, ok := uv.(string)
	if ok {
		return lua.S2L(val)
	}

	return lua.LNil
}

func luaBodyBind(L *lua.LState) int {
	ctx := checkRequestCtx(L)
	tn := L.CheckString(1)
	switch tn {
	case "json":
		fast := &kind.Fast{}
		err := fast.ParseBytes(ctx.Request.Body())
		if err != nil {
			L.RaiseError("invalid json body")
			return 0
		}
		L.Push(fast)
		return 1

	case "file":
		return newLuaFormFile(L)

	default:
		L.RaiseError("invalid bind type")
		return 0
	}
}
