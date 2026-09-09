package config

import "sync/atomic"

var current atomic.Pointer[Config]

func SetCurrent(cfg *Config) {
	if cfg == nil {
		return
	}
	snapshot := *cfg
	current.Store(&snapshot)
}

func Current() Config {
	cfg := current.Load()
	if cfg == nil {
		panic("config not initialized")
	}
	return *cfg
}

func CurrentOrDefault() Config {
	cfg := current.Load()
	if cfg == nil {
		return Config{}
	}
	return *cfg
}
