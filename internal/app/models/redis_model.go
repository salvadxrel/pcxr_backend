package models

import "time"

type Redis_Config_Model struct {
	Addr         string
	Password     string
	User         string
	DB           int
	MaxRetries   int
	DialTimeout  time.Duration
	Timeout      time.Duration
	PoolSize     int
	MinIdleConns int
}
