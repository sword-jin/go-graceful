package graceful

import (
	"net"
	"sync"
	"time"
)

type Listener struct {
	*net.TCPListener

	connLimit connLimit
	sem       chan struct{}
	closeOnce sync.Once
	done      chan struct{}
}

func newListener(tl *net.TCPListener, enableLimit bool, limitNumber int64) net.Listener {
	if limitNumber < 0 {
		limitNumber = defaultMaxConnectionNumber
	}
	l := &Listener{
		TCPListener: tl,
		done:        make(chan struct{}),
		connLimit: connLimit{
			enable: enableLimit,
			number: limitNumber,
		},
	}
	if enableLimit {
		l.sem = make(chan struct{}, limitNumber)
	}
	return l
}

type connLimit struct {
	enable bool
	number int64
}

func (l *Listener) Fd() (uintptr, error) {
	file, err := l.TCPListener.File()
	if err != nil {
		return 0, err
	}
	return file.Fd(), nil
}

// override
func (l *Listener) Accept() (net.Conn, error) {
	canAcquired := true
	if l.connLimit.enable {
		canAcquired = l.acquire()
	}
	tc, err := l.AcceptTCP()
	if err != nil {
		if l.connLimit.enable && canAcquired {
			l.release()
		}
		return nil, err
	}
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAlivePeriod(time.Minute)

	conn := &ListenerConn{Conn: tc, enableLimit: l.connLimit.enable}
	if l.connLimit.enable {
		conn.release = l.release
	}
	return conn, nil
}

// override
func (l *Listener) Close() error {
	err := l.TCPListener.Close()
	l.closeOnce.Do(func() { close(l.done) })
	return err
}

func (l *Listener) acquire() bool {
	select {
	case <-l.done:
		return false
	case l.sem <- struct{}{}:
		return true
	}
}

func (l *Listener) release() { <-l.sem }

type ListenerConn struct {
	net.Conn
	enableLimit bool
	releaseOnce sync.Once
	release     func()
}

func (l *ListenerConn) Close() error {
	err := l.Conn.Close()
	if l.enableLimit {
		l.releaseOnce.Do(l.release)
	}
	return err
}
