package pgpool

import (
	"testing"
	"time"
)

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"abc":   "'abc'",
		"it's":  "'it''s'",
		"a'b'c": "'a''b''c'",
		"中文'引号": "'中文''引号'",
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Fatalf("Quote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteAndParseTime(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	q := QuoteTime(now)
	if q == "NULL" || q[0] != '\'' {
		t.Fatalf("QuoteTime 异常: %s", q)
	}
	if QuoteTime(time.Time{}) != "NULL" {
		t.Fatal("零值时间应为 NULL")
	}
	// 解析常见文本格式
	if got := ParseTime("2026-10-05 06:30:00+08"); got.IsZero() {
		t.Fatal("ParseTime 不应解析失败")
	}
	if got := ParseTime("2026-10-05T06:30:00+08:00"); got.IsZero() {
		t.Fatal("ParseTime RFC3339 不应解析失败")
	}
}

func TestParseHelpers(t *testing.T) {
	if !ParseBool("t") || !ParseBool("true") || ParseBool("f") {
		t.Fatal("ParseBool 异常")
	}
	if ParseInt64("42") != 42 || ParseInt64("bad") != 0 {
		t.Fatal("ParseInt64 异常")
	}
	if ParseFloat64("0.5") != 0.5 || ParseFloat64("bad") != 0 {
		t.Fatal("ParseFloat64 异常")
	}
	if Str(nil) != "" || Str("x") != "x" {
		t.Fatal("Str 异常")
	}
}
