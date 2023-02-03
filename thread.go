package fasthttp

import (
	"github.com/vela-ssoc/vela-kit/lua"
)

func newLuaThread(ctx *RequestCtx) *lua.LState {
	var co *lua.LState

	uv := ctx.UserValue(thread_uv_key)
	if uv != nil {
		return uv.(*lua.LState)
	}

	//clone online security coroutine
	cv := ctx.UserValue(web_conf_key)
	if cv != nil {
		if cfg, ok := cv.(*config); ok {
			co = xEnv.Clone(cfg.co)
			co.SetValue(web_context_key, ctx)
			goto done
		}
	}

	co = xEnv.Coroutine()
	co.SetValue(web_context_key, ctx)
	goto done

done:
	ctx.SetUserValue(thread_uv_key, co)
	return co
}

func freeLuaThread(ctx *RequestCtx) {
	co := ctx.UserValue(thread_uv_key)
	if co == nil {
		return
	}

	xEnv.Free(co.(*lua.LState))
}
