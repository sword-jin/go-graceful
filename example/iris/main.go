package main

import (
	"context"
	"github.com/kataras/iris"
	context2 "github.com/kataras/iris/context"
	"github.com/rrylee/go-graceful"
	"log"
	"net"
	"syscall"
)

func main() {
	app := iris.New()

	app.Get("/", func(c context2.Context) {
		_, _ = c.Writef("hello world! pid=%d, ppid=%d", syscall.Getpid(), syscall.Getppid())
	})

	grace := graceful.New()
	grace.RegisterService(graceful.NewAddress("127.0.0.1:8124", "tcp"), func(ln net.Listener) error {
		return app.Run(iris.Listener(ln))
	}, func() error {
		return app.Shutdown(context.Background())
	})
	err := grace.Run()
	if err != nil {
		log.Fatal(err)
	}
}
