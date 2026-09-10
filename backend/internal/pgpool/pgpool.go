// Package pgpool 是一个仅用 Go 标准库实现的最小 PostgreSQL 客户端。
//
// 背景：本机 Go module 代理不可达，无法引入 lib/pq / pgx 等第三方驱动；
// 因此基于 PostgreSQL 官方 v3 前端/后端协议手写一个足够可用的客户端。
//
// 支持能力（覆盖本项目所需）：
//   - 启动握手（StartupMessage, 协议 3.0）
//   - 登录认证：AuthenticationOk / Cleartext / MD5（PostgreSQL 常用 md5 认证）
//   - 简单查询（Simple Query 'Q'）与结果扫描（RowDescription / DataRow）
//   - 文本协议结果：每个字段以其文本形式返回（NULL 返回 nil）
//   - 语句字段注入：用单引号转义（本项目所有值均由应用自身控制，无外部输入拼接）
//
// 不实现：SSL/TLS、扩展查询协议、COPY、事务游标等高级特性（本项目用不到）。
package pgpool

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Conn 一个 Postgres 连接。方法在内部串行化（单连接 + 互斥锁），
// 适合数据量小、单机低并发的数字分身应用。
type Conn struct {
	mu   sync.Mutex
	conn net.Conn
}

// Config Postgres 连接配置（可用 DATABASE_URL 或 PGHOST 等环境变量解析）。
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// Open 建立连接并完成认证。cfg 为空时回退到默认本地参数。
func Open(cfg Config) (*Conn, error) {
	applyDefaults(&cfg)
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	nc, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("pgpool: dial %s: %w", addr, err)
	}
	c := &Conn{conn: nc}
	if err := c.startup(&cfg); err != nil {
		_ = nc.Close()
		return nil, err
	}
	return c, nil
}

func applyDefaults(c *Config) {
	if c.Host == "" {
		c.Host = "localhost"
	}
	if c.Port == 0 {
		c.Port = 5432
	}
	if c.User == "" {
		c.User = "jasper"
	}
	if c.Database == "" {
		c.Database = c.User
	}
}

// Close 关闭底层连接。
func (c *Conn) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// ---- 启动与认证 ----

func (c *Conn) startup(cfg *Config) error {
	var b []byte
	b = appendInt32(b, 196608) // 协议版本 3.0
	b = appendParam(b, "user", cfg.User)
	b = appendParam(b, "database", cfg.Database)
	b = appendParam(b, "client_encoding", "UTF8")
	b = appendParam(b, "application_name", "jasperlee")
	b = append(b, 0) // 参数终止符
	// 外包长度字段（含自身 4 字节）
	pkt := appendInt32(nil, int32(len(b)+4))
	pkt = append(pkt, b...)
	if err := c.writeRaw(pkt); err != nil {
		return err
	}

	for {
		mtype, payload, err := c.readMsg()
		if err != nil {
			return err
		}
		switch mtype {
		case 'R':
			if len(payload) < 4 {
				return fmt.Errorf("pgpool: short AuthenticationRequest")
			}
			code := binary.BigEndian.Uint32(payload[:4])
			switch code {
			case 0: // AuthenticationOk
				// 之后会收到 ReadyForQuery 'Z'，继续循环直到 'Z'
			case 3: // CleartextPassword
				if err := c.sendPassword(cfg.Password); err != nil {
					return err
				}
			case 5: // MD5Password
				if len(payload) < 8 {
					return fmt.Errorf("pgpool: short MD5 salt")
				}
				salt := payload[4:8]
				if err := c.sendPassword(md5Response(cfg.User, cfg.Password, salt)); err != nil {
					return err
				}
			default:
				return fmt.Errorf("pgpool: unsupported auth method %d", code)
			}
		case 'E':
			return fmt.Errorf("pgpool: server error: %s", parseError(payload))
		case 'Z': // ReadyForQuery
			return nil
		case 'S', 'K', 'N': // ParameterStatus / BackendKeyData / NoticeResponse
			// 忽略
		default:
			// 忽略未知消息
		}
	}
}

func md5Response(user, pass string, salt []byte) string {
	inner := md5.Sum([]byte(pass + user)) // 密码+用户名 的二进制 MD5
	innerHex := hex.EncodeToString(inner[:])
	// 外层：MD5( innerHex字节 + salt )，再包 "md5" 前缀
	h := md5.Sum(append([]byte(innerHex), salt...))
	return "md5" + hex.EncodeToString(h[:])
}

func (c *Conn) sendPassword(p string) error {
	payload := append([]byte(p), 0)
	return c.writeMsg('p', payload)
}

// ---- 简单查询 ----

// Exec 执行不返回结果集的语句（DDL / INSERT / UPDATE / DELETE）。
func (c *Conn) Exec(sql string) error {
	_, err := c.query(sql)
	return err
}

// ExecCount 执行 DML 并返回受影响行数（用于判断删除/更新是否命中）。
func (c *Conn) ExecCount(sql string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.writeMsg('Q', append([]byte(sql), 0)); err != nil {
		return 0, err
	}
	var n int64
	for {
		mtype, payload, err := c.readMsg()
		if err != nil {
			return 0, err
		}
		switch mtype {
		case 'C': // CommandComplete，payload 形如 "UPDATE 1" / "DELETE 2" / "INSERT 0 1"
			sp := strings.Fields(string(payload))
			if len(sp) > 0 {
				n, _ = strconv.ParseInt(sp[len(sp)-1], 10, 64)
			}
		case 'Z':
			return n, nil
		case 'E':
			return 0, fmt.Errorf("pgpool: query error: %s", parseError(payload))
		case 'N', 'S', 'K', 'T', 'D', 'I':
		}
	}
}

// Query 执行查询并返回所有行。每行为一个 []any，非 NULL 的字段为 string 文本，
// NULL 字段为 nil。（简单查询协议返回的是文本格式结果。）
func (c *Conn) Query(sql string) ([][]any, error) {
	return c.query(sql)
}

func (c *Conn) query(sql string) ([][]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.writeMsg('Q', append([]byte(sql), 0)); err != nil {
		return nil, err
	}
	return c.readQueryResult()
}

func (c *Conn) readQueryResult() ([][]any, error) {
	var rows [][]any
	for {
		mtype, payload, err := c.readMsg()
		if err != nil {
			return nil, err
		}
		switch mtype {
		case 'T': // RowDescription
			// 无需解析列信息；DataRow 已自含字段长度。
		case 'D': // DataRow
			row, err := parseDataRow(payload)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		case 'C': // CommandComplete
		case 'I': // EmptyQueryResponse
		case 'Z': // ReadyForQuery -> 结束
			return rows, nil
		case 'E':
			return nil, fmt.Errorf("pgpool: query error: %s", parseError(payload))
		case 'N', 'S', 'K': // Notice / ParameterStatus / BackendKeyData
		default:
			// 忽略未知消息
		}
	}
}

func parseDataRow(payload []byte) ([]any, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("pgpool: short DataRow")
	}
	n := int(binary.BigEndian.Uint16(payload[:2]))
	pos := 2
	row := make([]any, n)
	for i := 0; i < n; i++ {
		if pos+4 > len(payload) {
			return nil, fmt.Errorf("pgpool: short DataRow field")
		}
		flen := int32(binary.BigEndian.Uint32(payload[pos : pos+4]))
		pos += 4
		if flen == -1 { // NULL
			row[i] = nil
			continue
		}
		if pos+int(flen) > len(payload) {
			return nil, fmt.Errorf("pgpool: short DataRow field data")
		}
		row[i] = string(payload[pos : pos+int(flen)])
		pos += int(flen)
	}
	return row, nil
}

func parseError(payload []byte) string {
	// payload 为一系列 \0 分隔的 {标签, 值}，最后以 '\0' 结尾。
	// 提取 'M'（主信息）与 'C'（SQLSTATE）。
	var msg, code string
	for len(payload) > 0 {
		i := 0
		for i < len(payload) && payload[i] != 0 {
			i++
		}
		if i == 0 {
			break
		}
		label := payload[0]
		val := string(payload[1:i])
		switch label {
		case 'M':
			msg = val
		case 'C':
			code = val
		}
		if i < len(payload) {
			payload = payload[i+1:]
		} else {
			break
		}
	}
	if code != "" {
		return code + ": " + msg
	}
	return msg
}

// ---- 底层消息读写 ----

func (c *Conn) writeMsg(mtype byte, payload []byte) error {
	pkt := appendInt32(nil, int32(len(payload)+4))
	pkt = append(pkt, byte(mtype))
	pkt = append(pkt, payload...)
	return c.writeRaw(pkt)
}

func (c *Conn) writeRaw(b []byte) error {
	_, err := c.conn.Write(b)
	return err
}

func (c *Conn) readMsg() (byte, []byte, error) {
	head := make([]byte, 5)
	if _, err := io.ReadFull(c.conn, head); err != nil {
		return 0, nil, err
	}
	mtype := head[0]
	length := int(binary.BigEndian.Uint32(head[1:5]))
	if length < 4 {
		return 0, nil, fmt.Errorf("pgpool: bad message length %d", length)
	}
	payload := make([]byte, length-4)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return 0, nil, err
	}
	return mtype, payload, nil
}

func appendInt32(b []byte, v int32) []byte {
	var t [4]byte
	binary.BigEndian.PutUint32(t[:], uint32(v))
	return append(b, t[:]...)
}

func appendParam(b []byte, k, v string) []byte {
	b = append(b, k...)
	b = append(b, 0)
	b = append(b, v...)
	b = append(b, 0)
	return b
}

// ---- SQL 字面量注入（安全：值全部来自应用自身，无用户原始 SQL） ----

// Quote 转义 SQL 单引号字符串字面量。
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// QuoteTime 把 time.Time 转成 PostgreSQL 可输入的 timestamptz 字面量。
func QuoteTime(t time.Time) string {
	if t.IsZero() {
		return "NULL"
	}
	return Quote(t.Format("2006-01-02 15:04:05.999999999-07:00"))
}

// ParseTime 解析 PostgreSQL 文本协议返回的时间字符串。
func ParseTime(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ParseBool 解析 PostgreSQL 文本布尔（"t"/"f"/"true"/"false"/"1"/"0"）。
func ParseBool(s string) bool {
	switch s {
	case "t", "true", "1", "on", "yes":
		return true
	}
	return false
}

// ParseInt64 解析整数字符串；失败返回 0。
func ParseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// ParseFloat64 解析浮点字符串；失败返回 0。
func ParseFloat64(s string) float64 {
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

// Str 把查询结果字段可靠转换为字符串（nil 返回 ""）。
func Str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}