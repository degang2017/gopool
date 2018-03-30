package pool

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrorClosed    = errors.New("client is closed")
	ErrPoolTimeout = errors.New("connection pool timeout")
	timers         = sync.Pool{
		New: func() interface{} {
			t := time.NewTimer(time.Hour)
			t.Stop()
			return t
		},
	}
)

type Pooler interface {
	NewConn() (*Conn, error)
	CloseConn(*Conn) error

	Get() (*Conn, error)
	Put(*Conn) error
	Remove(*Conn) error

	ConnNum() uint32
	FreeConnNum() uint32

	Stats() *Stats

	CloseAll() error
}

type Stats struct {
	//总的连接数
	TotalConns uint32
	//空闲连接数
	FreeConns uint32
	//连接池超时的次数
	PoolTimeouts uint32
	//连接错误数
	DialErrorNum uint32
}

type Options struct {
	Addr        string
	Protocol    string
	DialTimeout time.Duration

	PoolSize          int
	PoolTimeout       time.Duration
	FreeConnTimeout   time.Duration
	FreeConnCheckTime time.Duration
}

type Pool struct {
	option *Options

	lastDialError      error
	lastDialErrorMutex sync.RWMutex

	queue chan struct{}

	freeConnsMutex sync.Mutex
	freeConns      []*Conn

	connsMutex sync.Mutex
	conns      []*Conn

	status Stats

	closed uint32
}

var _ Pooler = (*Pool)(nil)

func Init(option *Options) *Pool {
	if option.Addr == "" {
		option.Addr = "localhost:80"
	}

	if option.Protocol == "" {
		option.Protocol = "tcp"
	}

	//默认超时时间为5秒
	if option.DialTimeout == 0 {
		option.DialTimeout = 5 * time.Second
	}

	if option.PoolSize == 0 {
		option.PoolSize = 10
	}

	if option.PoolTimeout == 0 {
		option.PoolTimeout = 3*time.Second + time.Second
	}

	if option.FreeConnTimeout == 0 {
		option.FreeConnTimeout = 5 * time.Minute
	}

	if option.FreeConnCheckTime == 0 {
		option.FreeConnCheckTime = time.Minute
	}

	pool := &Pool{
		option: option,

		queue:     make(chan struct{}, option.PoolSize),
		freeConns: make([]*Conn, 0, option.PoolSize),
		conns:     make([]*Conn, 0, option.PoolSize),
	}

	if option.FreeConnTimeout > 0 && option.FreeConnCheckTime > 0 {
		go pool.freeCheck(option.FreeConnCheckTime)
	}

	return pool
}

func (p *Pool) closeStatus() bool {
	return atomic.LoadUint32(&p.closed) == 1
}

func (p *Pool) NewConn() (*Conn, error) {
	if p.closeStatus() {
		return nil, ErrorClosed
	}

	netConn, err := net.DialTimeout(p.option.Protocol, p.option.Addr, p.option.DialTimeout)
	if err != nil {
		atomic.AddUint32(&p.status.DialErrorNum, 1)
		p.setLastDialError(err)
		return nil, err
	}

	conn := InitConn(netConn)
	p.connsMutex.Lock()
	p.conns = append(p.conns, conn)
	p.connsMutex.Unlock()

	return conn, nil
}

func (p *Pool) setLastDialError(err error) {
	p.lastDialErrorMutex.Lock()
	p.lastDialError = err
	p.lastDialErrorMutex.Unlock()
}

func (p *Pool) getLastDialError() error {
	p.lastDialErrorMutex.RLock()
	err := p.lastDialError
	p.lastDialErrorMutex.RUnlock()
	return err
}

func (p *Pool) CloseConn(conn *Conn) error {
	p.connsMutex.Lock()
	for k, v := range p.conns {
		if v == conn {
			p.conns = append(p.conns[:k], p.conns[k+1:]...)
			break
		}
	}
	p.connsMutex.Unlock()
	return conn.Close()
}

func (p *Pool) Get() (*Conn, error) {
	if p.closeStatus() {
		return nil, ErrorClosed
	}

	select {
	case p.queue <- struct{}{}:
	default:
		timer := timers.Get().(*time.Timer)
		timer.Reset(p.option.PoolTimeout)

		select {
		case p.queue <- struct{}{}:
			if !timer.Stop() {
				<-timer.C
			}

			timers.Put(timer)

		case <-timer.C:
			timers.Put(timer)
			atomic.AddUint32(&p.status.PoolTimeouts, 1)
			return nil, ErrPoolTimeout
		}
	}

	for {
		p.freeConnsMutex.Lock()
		conn := p.popFreeConn()
		p.freeConnsMutex.Unlock()

		if conn == nil {
			break
		}

		if conn.IsFreeTimeout(p.option.FreeConnTimeout) {
			p.CloseConn(conn)
			continue
		}

		return conn, nil
	}

	newConn, err := p.NewConn()
	if err != nil {
		<-p.queue
		return nil, err
	}

	return newConn, nil
}

func (p *Pool) popFreeConn() *Conn {
	freeLen := len(p.freeConns)
	if freeLen == 0 {
		return nil
	}

	idx := freeLen - 1
	conn := p.freeConns[idx]
	p.freeConns = p.freeConns[:idx]
	return conn

}

func (p *Pool) Put(conn *Conn) error {
	p.freeConnsMutex.Lock()
	p.freeConns = append(p.freeConns, conn)
	p.freeConnsMutex.Unlock()
	<-p.queue
	return nil
}

func (p *Pool) Remove(conn *Conn) error {
	p.CloseConn(conn)
	<-p.queue
	return nil
}

func (p *Pool) ConnNum() uint32 {
	p.connsMutex.Lock()
	len := len(p.conns)
	p.connsMutex.Unlock()
	return uint32(len)
}

func (p *Pool) FreeConnNum() uint32 {
	p.freeConnsMutex.Lock()
	len := len(p.freeConns)
	p.freeConnsMutex.Unlock()
	return uint32(len)
}

func (p *Pool) Stats() *Stats {
	return &Stats{
		TotalConns:   p.ConnNum(),
		FreeConns:    p.FreeConnNum(),
		PoolTimeouts: atomic.LoadUint32(&p.status.PoolTimeouts),
		DialErrorNum: atomic.LoadUint32(&p.status.DialErrorNum),
	}
}

func (p *Pool) CloseAll() error {
	if !atomic.CompareAndSwapUint32(&p.closed, 0, 1) {
		return ErrorClosed
	}

	var lastErr error
	for _, conn := range p.conns {
		if err := p.CloseConn(conn); err != nil {
			lastErr = err
		}
	}

	p.connsMutex.Lock()
	p.conns = nil
	p.connsMutex.Unlock()

	p.freeConnsMutex.Lock()
	p.freeConns = nil
	p.freeConnsMutex.Unlock()

	return lastErr
}

func (p *Pool) freeCheck(timeOut time.Duration) {
	ticker := time.NewTicker(timeOut)

	defer ticker.Stop()

	for range ticker.C {
		if p.closeStatus() {
			break
		}

		for {
			if p.FreeConnNum() == 0 {
				break
			}
			p.queue <- struct{}{}
			p.freeConnsMutex.Lock()

			conn := p.freeConns[0]
			reaper := conn.IsFreeTimeout(p.option.FreeConnTimeout)

			if !reaper {
				break
			} else {
				p.CloseConn(conn)
				p.freeConns = append(p.freeConns[:0], p.freeConns[1:]...)
			}
			p.freeConnsMutex.Unlock()
			<-p.queue

		}
	}
}
