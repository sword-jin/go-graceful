package graceful

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
)

type master struct {
	addr        Address
	opt         *option
	socketFiles []*os.File
	workerPid   int
	workerExit  chan error
}

func (m *master) run(services []*service) error {
	err := m.createFDs(services)
	if err != nil {
		return err
	}

	pid, err := m.forkWorker()
	if err != nil {
		return err
	}

	m.workerPid = pid
	m.waitSignal()
	return nil
}

func (m *master) waitSignal() {
	ch := make(chan os.Signal, 1)
	sigs := make([]os.Signal, 0, len(m.opt.reloadSignals)+len(m.opt.stopSignals))
	for _, s := range m.opt.reloadSignals {
		sigs = append(sigs, s)
	}
	for _, s := range m.opt.stopSignals {
		sigs = append(sigs, s)
	}
	signal.Notify(ch, sigs...)

	for {
		var sig os.Signal
		select {
		case err := <-m.workerExit:
			if _, ok := err.(*exec.ExitError); ok {
				log.Printf("[warning] worker exit with error: %+v, master is going to shutdown.", err)
				m.stop()
				return
			}
		case sig = <-ch:
			log.Printf("[info] master got signal: %v\n", sig)
		}

		for _, s := range m.opt.stopSignals {
			if s == sig {
				m.stop()
				return
			}
		}

		for _, s := range m.opt.reloadSignals {
			if s == sig {
				m.reload()
				break
			}
		}
	}
}

func (m *master) reload() {
	pid, err := m.forkWorker()
	if err != nil {
		log.Printf("[warning] fork error: %v\n", err)
	}

	m.workerPid = pid
}

func (m *master) stop() {
	os.Exit(1)
}

func (m *master) forkWorker() (int, error) {
	// 获取 args
	var args []string
	path := os.Args[0]
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	workerFlag := fmt.Sprintf("%s=%s", EnvWorker, EnvWorkerVal)
	oldWorkerPid := fmt.Sprintf("%s=%d", EnvOldWorkerPid, m.workerPid)
	env := append(os.Environ(), workerFlag, oldWorkerPid)

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = m.socketFiles
	cmd.Env = env
	err := cmd.Start()
	if err != nil {
		return 0, err
	}

	go func() {
		m.workerExit <- cmd.Wait()
	}()

	forkId := cmd.Process.Pid
	log.Printf("[info] start new process success, pid %d\n", forkId)

	return forkId, nil
}

func (m *master) createFDs(services []*service) error {
	m.socketFiles = make([]*os.File, 0, len(services))
	for _, service := range services {
		f, err := m.listen(service.addr)
		if err != nil {
			return fmt.Errorf("failed to listen on addr: %s, err: %v", m.addr, err)
		}
		m.socketFiles = append(m.socketFiles, f)
	}
	return nil
}

func (m *master) listen(addr Address) (*os.File, error) {
	if addr.network == "tcp" {
		a, err := net.ResolveTCPAddr("tcp", addr.addr)
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", a)
		if err != nil {
			return nil, err
		}
		f, err := l.File()
		if err != nil {
			return nil, err
		}
		if err := l.Close(); err != nil {
			return nil, err
		}
		return f, nil
	}

	return nil, fmt.Errorf("unknown network: %v", addr.network)
}
