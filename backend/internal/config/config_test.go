package config

import "testing"

func TestParseDatabaseURL(t *testing.T) {
	c := &Config{}
	parseDatabaseURL("postgres://user:pass@db-host:5433/mydb", c)
	if c.PGHost != "db-host" || c.PGPort != "5433" || c.PGUser != "user" || c.PGPassword != "pass" || c.PGDatabase != "mydb" {
		t.Fatalf("解析结果异常：%+v", c)
	}
}

func TestParseDatabaseURLIPv6(t *testing.T) {
	c := &Config{}
	parseDatabaseURL("postgresql://u@[::1]:5432/x", c)
	if c.PGHost != "::1" || c.PGPort != "5432" || c.PGUser != "u" || c.PGDatabase != "x" {
		t.Fatalf("IPv6 解析异常：%+v", c)
	}
}

func TestParseDatabaseURLNonPostgres(t *testing.T) {
	c := &Config{PGHost: "keep"}
	parseDatabaseURL("mysql://user@host/db", c)
	if c.PGHost != "keep" {
		t.Fatal("非 postgres 协议不应改动配置")
	}
}
