# Hertz-WebSocket(This is a community driven project)

**Notice**: This project is now archived. For users and those seeking similar functionality, please consider migrate to the official `gorilla/websocket` toolchain using the [Hertz HTTP Adaptor](https://www.cloudwego.io/docs/hertz/tutorials/basic-feature/http-adaptor/). Check [migration-guide](https://www.cloudwego.io/docs/hertz/tutorials/third-party/protocol/websocket/#migration-guide).

This repo is forked from [Gorilla WebSocket](https://github.com/gorilla/websocket/) and adapted to Hertz.

### How to use
```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/hertz-contrib/websocket"
)

var upgrader = websocket.HertzUpgrader{} // use default options

func echo(_ context.Context, c *app.RequestContext) {
	err := upgrader.Upgrade(c, func(conn *websocket.Conn) {
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", message)
			err = conn.WriteMessage(mt, message)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	})
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
}


func main() {
	h := server.Default(server.WithHostPorts(addr))
	// https://github.com/cloudwego/hertz/issues/121
	h.NoHijackConnPool = true
	h.GET("/echo", echo)
	h.Spin()
}

```

### More info

See [examples](examples/)

