package websocket

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol"
)

const (
	testaddr = "localhost:10012"
	testpath = "/echo"
)

func ExampleClient() {
	runServer(testaddr)
	time.Sleep(50 * time.Millisecond) // await server running

	c, err := client.NewClient(client.WithDialer(standard.NewDialer()))
	if err != nil {
		panic(err)
	}

	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	req.SetRequestURI("http://" + testaddr + testpath)
	req.SetMethod("GET")

	u := &ClientUpgrader{}
	u.PrepareRequest(req)
	err = c.Do(context.Background(), req, resp)
	if err != nil {
		panic(err)
	}
	conn, err := u.UpgradeResponse(req, resp)
	if err != nil {
		panic(err)
	}

	conn.WriteMessage(TextMessage, []byte("hello"))
	m, b, err := conn.ReadMessage()
	if err != nil {
		panic(err)
	}
	fmt.Println(m, string(b))
	// Output: 1 hello
}

func runServer(addr string) {
	upgrader := HertzUpgrader{} // use default options
	h := server.Default(server.WithHostPorts(addr))
	// https://github.com/cloudwego/hertz/issues/121
	h.NoHijackConnPool = true
	h.GET(testpath, func(_ context.Context, c *app.RequestContext) {
		err := upgrader.Upgrade(c, func(conn *Conn) {
			for {
				mt, message, err := conn.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					break
				}
				log.Printf("[server] recv: %v %s", mt, message)
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
	})
	go func() {
		if err := h.Run(); err != nil {
			log.Fatal(err)
		}
	}()
}
