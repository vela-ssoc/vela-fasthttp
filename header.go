package fasthttp

import (
	"errors"
	"fmt"
	"github.com/vela-ssoc/vela-kit/lua"
	"strings"
)

type headerKV struct {
	key string
	val string
}

type header []headerKV

func (h *header) String() string                         { return fmt.Sprintf("fasthttp.header %p", h) }
func (h *header) Type() lua.LValueType                   { return lua.LTObject }
func (h *header) AssertFloat64() (float64, bool)         { return 0, false }
func (h *header) AssertString() (string, bool)           { return "", false }
func (h *header) AssertFunction() (*lua.LFunction, bool) { return nil, false }
func (h *header) Peek() lua.LValue                       { return h }

func (h *header) Len() int {
	return len(*h)
}

func (h *header) Set(key string, val string) {
	hd := *h
	n := h.Len()
	for i := 0; i < n; i++ {
		item := &hd[i]
		if strings.EqualFold(item.key, key) {
			item.key = key
			item.val = val
			return
		}
	}

	hd = append(hd, headerKV{key, val})
	*h = hd
}

func (h *header) ForEach(fn func(string, string)) {
	hd := *h
	n := h.Len()
	for i := 0; i < n; i++ {
		item := &hd[i]
		fn(item.key, item.val)
	}
}

func newHeader() *header {
	return &header{}
}

func toHeader(L *lua.LState, val lua.LValue) *header {
	if val.Type() != lua.LTTable {
		L.RaiseError("header must be table")
		return nil
	}
	tab := val.(*lua.LTable)
	h := newHeader()

	tab.Range(func(key string, val lua.LValue) {

		switch val.Type() {

		case lua.LTString:
			h.Set(key, val.String())

		case lua.LTNumber:
			h.Set(key, val.String())

		default:
			L.RaiseError("invalid header , must be string , got %s", val.Type().String())
		}
	})

	return h
}

func newLuaHeader(L *lua.LState) int {
	tab := L.CheckTable(1)
	h := newHeader()
	tab.Range(func(key string, val lua.LValue) {
		switch val.Type() {

		case lua.LTString:
			h.Set(key, val.String())

		case lua.LTNumber:
			h.Set(key, val.String())

		default:
			L.RaiseError("invalid header , must be string , got %s", val.Type().String())
		}
	})

	L.Push(h)
	return 1
}

var (
	invalidHeaderType  = errors.New("invalid header type , must be userdata")
	invalidHeaderValue = errors.New("invalid header value")
)
