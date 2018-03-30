package pool_test

import (
	"github.com/gopool/pool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("ConnPool", func() {
	var connPool *pool.Pool

	BeforeEach(func() {
		connPool = pool.Init(&pool.Options{
			Addr:        "127.0.0.1",
			Protocol:    "tcp",
			DialTimeout: 3 * time.Second,

			PoolSize: 10,
		})
	})
	AfterEach(func() {
		connPool.CloseAll()
	})

	It("Get conn", func() {
		conn, err := connPool.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(connPool.ConnNum()).To(Equal(uint32(1)))
		connPool.Put(conn)
		Expect(connPool.FreeConnNum()).To(Equal(uint32(1)))
	})

	It("Close all", func() {
		err := connPool.CloseAll()
		Expect(err).NotTo(HaveOccurred())
		Expect(connPool.FreeConnNum()).To(Equal(uint32(0)))
	})
})

var _ = Describe("Free conn timeout check", func() {
	var connPool *pool.Pool

	BeforeEach(func() {
		connPool = pool.Init(&pool.Options{
			Addr:              "127.0.0.1",
			Protocol:          "tcp",
			DialTimeout:       3 * time.Second,
			FreeConnCheckTime: 5 * time.Second,
			FreeConnTimeout:   5 * time.Second,

			PoolSize: 10,
		})

		conn, err := connPool.Get()
		Expect(err).NotTo(HaveOccurred())
		connPool.Put(conn)

	})

	It("Conn number", func() {
		Expect(connPool.ConnNum()).To(Equal(uint32(1)))
		Expect(connPool.FreeConnNum()).To(Equal(uint32(1)))
	})

	It("Free conn number", func() {
		time.Sleep(6 * time.Second)
		Expect(connPool.FreeConnNum()).To(Equal(uint32(0)))
		Expect(connPool.ConnNum()).To(Equal(uint32(0)))
	})

	AfterEach(func() {
		connPool.CloseAll()
	})
})
