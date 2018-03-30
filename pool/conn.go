package pool

import (
	"net"
	"sync/atomic"
	"time"
)

type Conn struct {
	netConn net.Conn
	useTime atomic.Value
}

func InitConn(netConn net.Conn) *Conn {
	conn := &Conn{
		netConn: netConn,
	}

	conn.SetUseTime(time.Now())
	return conn
}

func (conn *Conn) Close() error {
	return conn.netConn.Close()
}

func (conn *Conn) UseTime() time.Time {
	return conn.useTime.Load().(time.Time)
}

func (conn *Conn) SetUseTime(tm time.Time) {
	conn.useTime.Store(tm)
}

func (conn *Conn) IsFreeTimeout(timeout time.Duration) bool {
	return timeout > 0 && time.Since(conn.UseTime()) > timeout
}

func (conn *Conn) Write(b []byte) (n int, err error) {
	return conn.netConn.Write(b)
}

func (conn *Conn) Read(b []byte) (n int, err error) {
	return conn.netConn.Read(b)
}

func (conn *Conn) RemoteAddr() net.Addr {
	return conn.netConn.RemoteAddr()
}
