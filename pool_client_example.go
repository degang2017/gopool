package main

import (
	"fmt"
	"github.com/degang2017/gopool/pool"
	"time"
)

func main() {

	connChan := make(chan struct{}, 30)

	poolConn := pool.Init(&pool.Options{
		Addr:        "127.0.0.1:8001",
		Protocol:    "tcp",
		DialTimeout: 3 * time.Second,

		PoolSize: 10,
	})

	for i := 1; i < 1000; i++ {
		go func(i int) {
			connChan <- struct{}{}
			var conn *pool.Conn
			var err error
			conn, err = poolConn.Get()
			if err != nil {
				fmt.Println("error", err)
			}
			fmt.Printf("conn %-v ---\n", conn)
			//conn.Write([]byte("hello" + strconv.Itoa(i) + "\n"))
			poolConn.Put(conn)
			fmt.Println(poolConn.Stats())
			fmt.Println("addr", conn.RemoteAddr())
		}(i)
	}

	for {
		select {
		case <-connChan:
			fmt.Println("done")
		}
	}

}
