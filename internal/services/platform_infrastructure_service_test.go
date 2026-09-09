package services

import "testing"

func TestParseRedisInfoExtractsStorageMetrics(t *testing.T) {
	values := parseRedisInfo("# Memory\r\nused_memory:4096\r\nused_memory_dataset:3072\r\nmaxmemory:8192\r\n# Clients\r\nconnected_clients:7\r\n")
	if got := redisInfoInt64(values, "used_memory_dataset"); got != 3072 {
		t.Fatalf("used_memory_dataset = %d, want 3072", got)
	}
	if got := redisInfoInt64(values, "maxmemory"); got != 8192 {
		t.Fatalf("maxmemory = %d, want 8192", got)
	}
	if got := redisInfoInt64(values, "connected_clients"); got != 7 {
		t.Fatalf("connected_clients = %d, want 7", got)
	}
}

func TestClampPercentBoundsAndRounds(t *testing.T) {
	if got := clampPercent(12.345); got != 12.3 {
		t.Fatalf("clampPercent(12.345) = %v, want 12.3", got)
	}
	if got := clampPercent(140); got != 100 {
		t.Fatalf("clampPercent(140) = %v, want 100", got)
	}
	if got := clampPercent(-1); got != 0 {
		t.Fatalf("clampPercent(-1) = %v, want 0", got)
	}
}
