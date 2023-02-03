package fasthttp

import (
	"bytes"
	"github.com/vela-ssoc/vela-kit/auxlib"
	"github.com/vela-ssoc/vela-kit/kind"
	"github.com/vela-ssoc/vela-kit/lua"
	"regexp"
)

// "hello ${aa} pp ccc dd ee ff "
var reg = regexp.MustCompile(`\${([a-zA-Z0-9_]{2,})}`)

type conversion struct {
	raw    []byte
	handle []func(*RequestCtx) []byte
}

func (cnn *conversion) len() int {
	return len(cnn.handle)
}

func (cnn *conversion) push(v []byte) {
	cnn.append(func(_ *RequestCtx) []byte {
		return v
	})
}

func (cnn *conversion) append(fn func(*RequestCtx) []byte) {
	if fn == nil {
		return
	}

	cnn.handle = append(cnn.handle, fn)
}

func (cnn *conversion) pretreatment(data string) {
	indexes := reg.FindAllStringIndex(data, -1)
	size := len(indexes)
	if size == 0 {
		cnn.raw = auxlib.S2B(data)
		return
	}

	offset := 0
	for i := 0; i < size; i++ {
		el := indexes[i]
		if offset != el[0] {
			cnn.push([]byte(data[offset:el[0]]))
		}
		offset = el[1]

		//克隆原有字符串防止无法寻址
		var buf []byte
		copy(buf, data[el[0]:el[1]])

		// ${item}
		item := data[el[0]+2 : el[1]-1]
		cnn.append(func(ctx *RequestCtx) []byte {
			lv := k2v(ctx, item)
			if lv == lua.LNil {
				return buf
			}
			return auxlib.S2B(lv.String())
		})
	}

	if offset != len(data) {
		cnn.push([]byte(data[offset:]))
	}

}

func (cnn *conversion) Response(ctx *RequestCtx) {
	n := len(cnn.handle)
	if n == 0 {
		ctx.Response.SetBodyRaw(cnn.raw)
		return
	}

	for i := 0; i < n; i++ {
		fn := cnn.handle[i]
		ctx.Response.AppendBody(fn(ctx))
	}
}

func (cnn *conversion) Map(ctx *RequestCtx) []byte {
	buff := kind.NewJsonEncoder()
	n := len(cnn.handle)
	buff.Tab("")
	if n == 0 {
		buff.Write(cnn.raw)
		buff.End("]")
		return buff.Bytes()
	}

	for i := 0; i < n; i++ {
		fn := cnn.handle[i]
		buff.Insert(fn(ctx))
		buff.Char(',')
	}

	buff.End("]")
	return buff.Bytes()
}

func (cnn *conversion) Json(ctx *RequestCtx) []byte {
	buff := kind.NewJsonEncoder()
	n := len(cnn.handle)
	buff.Arr("")
	if n == 0 {
		buff.Write(cnn.raw)
		buff.End("]")
		return buff.Bytes()
	}

	for i := 0; i < n; i++ {
		fn := cnn.handle[i]
		buff.Insert(fn(ctx))
		buff.Char(',')
	}

	buff.End("]")

	return buff.Bytes()

}

func (cnn *conversion) Line(ctx *RequestCtx) []byte {
	var buff bytes.Buffer

	n := len(cnn.handle)
	if n == 0 {
		buff.Write(cnn.raw)
		return buff.Bytes()
	}

	for i := 0; i < n; i++ {
		fn := cnn.handle[i]
		buff.Write(fn(ctx))
	}

	return buff.Bytes()

}
