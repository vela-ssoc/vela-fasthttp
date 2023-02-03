package fasthttp

import (
	"github.com/vela-ssoc/vela-kit/lua"
)

func (fss *server) Header(out lua.Console) {
	out.Printf("type: %s", fss.Type())
	out.Printf("uptime: %s", fss.Uptime.Format("2006-01-02 15:04:06"))
	out.Printf("version: v1.0.5")
	out.Println("")
}

func (fss *server) Show(out lua.Console) {
	fss.Header(out)
	out.Printf("name  = %s", fss.Name())
	out.Printf("bind = %s", fss.cfg.bind.String())
	out.Printf("routers = %s", fss.cfg.router)
	out.Printf("handler = %s", fss.cfg.handler)
	out.Printf("not_found = %s", fss.cfg.notFound)
	out.Printf("output = %s", fss.cfg.output.Name())
}

func (fss *server) Help(out lua.Console) {
	fss.Header(out)
}
