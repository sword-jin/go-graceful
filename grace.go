package graceful

import (
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

const (
	EnvWorker       = "ENV_WORKER"
	EnvWorkerVal    = "1"
	EnvOldWorkerPid = "ENV_OLD_WORKER_PID"
)

var (
	defaultWatchInterval             = time.Second
	defaultStopTimeout               = 20 * time.Second
	defaultReloadSignals             = []syscall.Signal{syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2}
	defaultStopSignals               = []syscall.Signal{syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT}
	defaultMaxConnectionNumber int64 = 10000
)

type Grace struct {
	opt      *option
	addr     Address
	services []*service
}

type service struct {
	addr         Address
	startFunc    func(net.Listener) error
	shutdownFunc func() error
}

type Address struct {
	addr    string
	network string //tcp, unix
}

func NewAddress(addr, network string) Address {
	return Address{addr, network}
}

func New(opts ...Option) *Grace {
	option := &option{
		watchInterval:         defaultWatchInterval,
		stopTimeout:           defaultStopTimeout,
		stopSignals:           defaultStopSignals,
		reloadSignals:         defaultReloadSignals,
		enableConnectionLimit: false,
		maxConnectionNumber:   defaultMaxConnectionNumber,
	}
	for _, opt := range opts {
		opt(option)
	}
	return &Grace{
		opt:      option,
		services: []*service{},
	}
}

func (g *Grace) RegisterService(addr Address, startFun func(ln net.Listener) error, shutdownFun func() error) {
	g.services = append(g.services, &service{
		addr:         addr,
		startFunc:    startFun,
		shutdownFunc: shutdownFun,
	})
}

func (g *Grace) Run() error {
	if IsWorker() {
		log.Printf("[info]Worker here, pid=%v", syscall.Getpid())
		worker := &worker{
			opt:       g.opt,
			stopCh:    make(chan struct{}),
			listeners: make([]net.Listener, 0, len(g.services)),
			services:  g.services,
		}
		return worker.run()
	}
	log.Printf("[info]master here, pid=%v", syscall.Getpid())
	master := &master{
		addr:       g.addr,
		opt:        g.opt,
		workerExit: make(chan error),
	}
	return master.run(g.services)
}

func IsWorker() bool {
	return os.Getenv(EnvWorker) == EnvWorkerVal
}

func IsMaster() bool {
	return !IsWorker()
}

type option struct {
	watchInterval         time.Duration
	stopTimeout           time.Duration
	reloadSignals         []syscall.Signal
	stopSignals           []syscall.Signal
	enableConnectionLimit bool
	maxConnectionNumber   int64
}

type Option func(o *option)

func WithStopTimeout(timeout time.Duration) Option {
	return func(o *option) {
		o.stopTimeout = timeout
	}
}

func WithWatchInterval(timeout time.Duration) Option {
	return func(o *option) {
		o.watchInterval = timeout
	}
}

func WithReloadSignals(reloadSignals []syscall.Signal) Option {
	return func(o *option) {
		o.reloadSignals = reloadSignals
	}
}

func WithStopSignals(stopSignals []syscall.Signal) Option {
	return func(o *option) {
		o.stopSignals = stopSignals
	}
}

func WithConnectionLimit(enable bool, limit int64) Option {
	return func(o *option) {
		o.enableConnectionLimit = enable
		o.maxConnectionNumber = limit
	}
}
