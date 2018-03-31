# gopool
gopool连接池服务

# 案例

``` go
    //初始化一个连接池
    poolConn := pool.Init(&pool.Options{
        Addr:        "127.0.0.1:8001", //地址
        Protocol:    "tcp",            //协议
        DialTimeout: 3 * time.Second,  //连接超时时间

        PoolSize:          10,              //连接池大小
        PoolTimeout:       3 * time.Second, //连接池超时时间
        FreeConnTimeout:   5 * time.Minute, //连接池空闲连接超时时间
        FreeConnCheckTime: 5 * time.Minute, //连接池空闲连接检查时间
    }) 

    //获取或创建一个连接
    conn, err = poolConn.Get()
    if err != nil {
        fmt.Println("error", err)
    }  
 
    //放入池中
    poolConn.Put(conn)
```
