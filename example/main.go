package main

import (
	"context"
	"fmt"
	"github.com/rrylee/go-graceful"
	"log"
	"net"
	"net/http"
	"syscall"
)

type myHandler struct{}
func (*myHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(fmt.Sprintf("Hello world, pid = %d, ppid=%d", syscall.Getpid(), syscall.Getppid())))
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", &myHandler{})

	srv := &http.Server{Handler: mux}

	grace := graceful.New()
	grace.RegisterService(graceful.NewAddress("127.0.0.1:8124", "tcp"), func(ln net.Listener) error {
		return srv.Serve(ln)
	}, func() error {
		return srv.Shutdown(context.Background())
	})

	err := grace.Run()
	if err != nil {
		log.Fatal(err)
	}
}

