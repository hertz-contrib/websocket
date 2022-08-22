# Hertz-WebSocket(This is a community driven project)


This repo is forked from [Gorilla WebSocket](https://github.com/gorilla/websocket/) and adapted to Hertz.

### Example
```
	h := server.Default(server.WithHostPorts(addr))
	h.GET("/ws", func(c context.Context, ctx *app.RequestContext) {

		err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) error {
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
			return nil
		})
		if err != nil {
			log.Println(err)
			return
		}
		
	})

	h.Spin()
```

