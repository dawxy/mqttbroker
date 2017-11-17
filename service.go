package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vizee/echo"
)

func Run() {
	l, err := net.Listen("tcp", optListen)
	if err != nil {
		panic(err)
	}
	signalChan := make(chan os.Signal, 1)
	go func() {
		signal.Notify(signalChan,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)
		sign := <-signalChan
		echo.Info("get", echo.String("sign", sign.String()))
		l.Close()
		// TODO: graceful
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			echo.Error("accept", echo.Errval(err))
			break
		}
		c := newConn(conn)
		go c.serve()
	}
}
