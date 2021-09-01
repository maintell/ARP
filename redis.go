package main

import (
	redigo "github.com/gomodule/redigo/redis"
	"time"
)

// PoolInitRedis redis pool Initialization
func RedisPoolInitialization(server string, password string, dbIndex int) *redigo.Pool {
	return &redigo.Pool{
		MaxIdle:     10, //空闲数
		IdleTimeout: 240 * time.Second,
		MaxActive:   0, //最大数
		Wait:        true,
		Dial: func() (redigo.Conn, error) {
			c, err := redigo.Dial("tcp", server, redigo.DialDatabase(dbIndex), redigo.DialPassword(password))

			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
