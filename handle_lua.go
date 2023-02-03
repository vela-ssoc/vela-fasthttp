package fasthttp

import (
	"fmt"
	cond "github.com/vela-ssoc/vela-cond"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/lua"
	"io"
	"net/http"
)

func (hd *handle) String() string                         { return fmt.Sprintf("fasthttp.handle %p", hd) }
func (hd *handle) Type() lua.LValueType                   { return lua.LTObject }
func (hd *handle) AssertFloat64() (float64, bool)         { return 0, false }
func (hd *handle) AssertString() (string, bool)           { return "", false }
func (hd *handle) AssertFunction() (*lua.LFunction, bool) { return nil, false }
func (hd *handle) Peek() lua.LValue                       { return hd }

func (hd *handle) filterL(lv lua.LValue) {
	switch lv.Type() {
	case lua.LTString:
		hd.cnd = cond.New(lv.String())

	case lua.LTTable:
		hd.cnd = cond.New(auxlib.LTab2SS(lv.(*lua.LTable))...)
	}
}

func (hd *handle) NewIndex(L *lua.LState, key string, val lua.LValue) {
	switch key {
	case "method":
		hd.method = val.String()
	case "filter":
		hd.filterL(val)

	case "code":
		hd.code = lua.IsInt(val)

	case "header":
		hd.header = toHeader(L, val)

	case "close":
		hd.close = lua.IsFunc(val)

	case "eof":
		hd.eof = lua.IsTrue(val)

	case "body":
		switch val.Type() {
		case lua.LTString:
			hd.body = compileHandleBody(val.String())

		case lua.LTFunction:
			cp := xEnv.P(val.(*lua.LFunction))
			cp.NRet = 0

			hd.body = func(ctx *RequestCtx) error {
				co := newLuaThread(ctx)
				return co.CallByParam(cp)
			}

		default:
			hd.body = func(ctx *RequestCtx) error {
				ctx.SetBodyString(val.String())
				return nil
			}
		}

	}
}

func newLuaRedirectL(L *lua.LState) int {
	location := L.CheckString(1)
	code := L.IsInt(1)
	if code == 0 {
		code = 302
	}

	hd := newHandle("")
	h := newHeader()

	h.Set("location", location)

	hd.code = code
	hd.header = h
	hd.eof = true
	L.Push(hd)
	return 1
}

func newLuaCloneL(L *lua.LState) int {
	url := L.CheckString(1)
	hd := newHandle("")
	h := newHeader()
	var body []byte
	var err error

	rsp, err := http.Get(url)
	if err != nil {
		hd.code = http.StatusInternalServerError
		hd.body = func(ctx *RequestCtx) error {
			ctx.SetBodyString("clone fail")
			return nil
		}
		goto done
	}

	for key, val := range rsp.Header {
		for _, iv := range val {
			h.Set(key, iv)
		}
	}

	body, err = io.ReadAll(rsp.Body)
	if err != nil {
		hd.code = http.StatusInternalServerError
		hd.body = func(ctx *RequestCtx) error {
			ctx.SetBodyString("clone fail")
			return nil
		}
		goto done
	}

	hd.header = h
	hd.code = rsp.StatusCode
	hd.body = func(ctx *RequestCtx) error {
		ctx.SetBody(body)
		return nil
	}

done:
	L.Push(hd)
	return 1

}

func newLuaHandle(L *lua.LState) int {
	val := L.Get(1)

	hd := newHandle("")

	switch val.Type() {
	case lua.LTNil:
		hd.code = 404
		hd.body = func(ctx *RequestCtx) error {
			ctx.SetBodyString("nobody")
			return nil
		}

	case lua.LTString:
		hd.code = 200
		hd.eof = true
		hd.body = compileHandleBody(val.String())

	case lua.LTTable:
		val.(*lua.LTable).Range(func(key string, val lua.LValue) {
			hd.NewIndex(L, key, val)
		})

	default:
		hd.code = 200
		hd.eof = true
		hd.body = compileHandleBody(val.String())

	}

	L.Push(hd)
	return 1
}
