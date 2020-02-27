package main

import (
	"fmt"
	"github.com/rrylee/go-graceful"
	"github.com/valyala/fasthttp"
	"log"
	"net"
	"syscall"
)

func main() {
	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			_, _ = fmt.Fprintf(ctx, "Hello, world! pid = %d, ppid = %d", syscall.Getpid(), syscall.Getppid())
		},
	}

	grace := graceful.New()
	grace.RegisterService(graceful.NewAddress("127.0.0.1:8124", "tcp"), func(ln net.Listener) error {
		return server.Serve(ln)
	}, func() error {
		return server.Shutdown()
	})

	err := grace.Run()
	if err != nil {
		log.Fatal(err)
	}
}
