package graceful

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	workerStopSignal = syscall.SIGTERM
)

type worker struct {
	opt       *option
	stopCh    chan struct{}
	listeners []net.Listener
	services  []*service
}

func (w *worker) run() error {
	err := w.initListeners()
	if err != nil {
		return err
	}

	go w.startServers()

	oldWorkerPid, err := strconv.Atoi(os.Getenv(EnvOldWorkerPid))
	if err != nil {
		return err
	}
	if oldWorkerPid > 1 {
		err = syscall.Kill(oldWorkerPid, workerStopSignal)
		if err != nil {
			log.Printf("[warning]kill old worker error: %v", err)
		}
	}

	go w.watchMaster()
	w.waitSignal()

	return nil
}

func (w *worker) initListeners() error {
	for i := 0; i < len(w.services); i++ {
		f := os.NewFile(uintptr(3+i), "")
		l, err := net.FileListener(f)
		if err != nil {
			return err
		}
		w.listeners = append(w.listeners, newListener(l.(*net.TCPListener), w.opt.enableConnectionLimit, w.opt.maxConnectionNumber))
	}
	return nil
}

func (w *worker) startServers() {
	for i, l := range w.listeners {
		err := w.services[i].startFunc(l)
		if err != nil {
			log.Printf("[warning]worker start service error: %v, service is %v", err, w.services)
		}
	}
}

func (w *worker) watchMaster() {
	for {
		// if parent id change to 1, it means parent is dead
		if os.Getppid() == 1 {
			log.Printf("[warning] master dead, stop worker <%d>\n", syscall.Getpid())
			w.stop()
			break
		}
		time.Sleep(w.opt.watchInterval)
	}
	w.stopCh <- struct{}{}
}

func (w *worker) stop() {
	for _, s := range w.services {
		err := s.shutdownFunc()
		if err != nil {
			fmt.Printf("[warning]shutdown server error: %s", err)
		}
	}
}

func (w *worker) waitSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, workerStopSignal)
	select {
	case sig := <-ch:
		log.Printf("[info]worker <%d> got signal: %v\n", syscall.Getpid(), sig)
	case <-w.stopCh:
		log.Printf("[info]stop worker: %d", syscall.Getpid())
	}

	w.stop()
}
