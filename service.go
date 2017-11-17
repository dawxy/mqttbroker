package main

import (
	"net"
	"time"

	"github.com/vizee/echo"
)

func Run() {
	l, err := net.Listen("tcp", optListen)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			echo.Error("accept", echo.Errval(err))
			time.Sleep(time.Second)
		}
		c := newConn(conn)
		go c.serve()
	}
}
