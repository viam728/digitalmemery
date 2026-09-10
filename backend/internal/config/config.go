package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv 读取 .env（若存在），把 KEY=VALUE 注入环境变量（不覆盖已有环境变量）。
// 依次尝试：当前工作目录、可执行文件所在目录（与 cwd 解耦，便于任意路径启动）。
func loadDotEnv() {
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}
	for _, p := range candidates {
		applyDotEnv(p)
	}
}

func applyDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// Config 后端运行时配置
type Config struct {
	Port string
	// DataDir 数据目录（文件存储、SQLite、向量索引等）
	DataDir string
	// RAGProvider 向量化/检索提供方：aliyun | local | mock
	RAGProvider string
	// AliyunAPIKey 阿里云 DashVector / 百炼 API Key（免费额度）
	AliyunAPIKey string
	// AliyunEndpoint DashVector 端点
	AliyunEndpoint string
	// ModelProvider 模型提供方：mock | deepseek | 其它 OpenAI 兼容
	ModelProvider string
	// ModelAPIKey 模型密钥（服务端配置）
	ModelAPIKey string
	// ModelBaseURL OpenAI 兼容端点
	ModelBaseURL string
	// ModelName 模型名，如 deepseek-chat
	ModelName string
	// ModelList 可选模型列表（逗号分隔），为空则用 ModelName；
	// 启动时若能连通 GLM /models 接口会自动刷新（见 /api/models）
	ModelList string
	// ModelOverride 用户直连 GLM 主机 IP（本机 DNS 被污染时用，见 README）
	ModelHostIP string
	// AdminPassword 管理员入口密码（暂定 feng，可通过 .env 覆盖）
	AdminPassword string
	// PGHost PostgreSQL 主机（DATABASE_URL 优先时由其填充）
	PGHost string
	// PGPort PostgreSQL 端口（string 简化）
	PGPort string
	// PGUser PostgreSQL 用户
	PGUser string
	// PGPassword PostgreSQL 密码
	PGPassword string
	// PGDatabase PostgreSQL 数据库名
	PGDatabase string
}

// Load 从环境变量 + backend/.env 读取配置，提供默认值
func Load() *Config {
	loadDotEnv()
	c := &Config{
		Port:           env("PORT", "8080"),
		DataDir:        env("DATA_DIR", "../data"),
		RAGProvider:    env("RAG_PROVIDER", "mock"),
		AliyunAPIKey:   env("ALIYUN_API_KEY", ""),
		AliyunEndpoint: env("ALIYUN_ENDPOINT", ""),
		ModelProvider:  env("MODEL_PROVIDER", "mock"),
		ModelAPIKey:    env("MODEL_API_KEY", ""),
		ModelBaseURL:   env("MODEL_BASE_URL", "https://api.deepseek.com"),
		ModelName:      env("MODEL_NAME", "deepseek-chat"),
		ModelList:      env("MODEL_LIST", ""),
		ModelHostIP:    env("MODEL_HOST_IP", ""),
		AdminPassword:  env("ADMIN_PASSWORD", "feng"),
		PGHost:         env("PGHOST", "localhost"),
		PGPort:         env("PGPORT", "5432"),
		PGUser:         env("PGUSER", "jasper"),
		PGPassword:     env("PGPASSWORD", "jasper"),
		PGDatabase:     env("PGDATABASE", "jasper"),
	}
	// DATABASE_URL（postgres://user:pass@host:port/db）优先解析填充 PG 字段
	if du := os.Getenv("DATABASE_URL"); du != "" {
		parseDatabaseURL(du, c)
	}
	return c
}

// parseDatabaseURL 解析 postgres://user:pass@host:port/db 形式的连接串，填充 PG 相关字段。
func parseDatabaseURL(u string, c *Config) {
	rest := u
	switch {
	case strings.HasPrefix(rest, "postgres://"):
		rest = strings.TrimPrefix(rest, "postgres://")
	case strings.HasPrefix(rest, "postgresql://"):
		rest = strings.TrimPrefix(rest, "postgresql://")
	default:
		return
	}
	// userinfo@host:port/db
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return
	}
	userinfo := rest[:at]
	hostport := rest[at+1:]
	if i := strings.Index(userinfo, ":"); i >= 0 {
		c.PGUser = userinfo[:i]
		c.PGPassword = userinfo[i+1:]
	} else {
		c.PGUser = userinfo
	}
	// host:port/db
	if i := strings.Index(hostport, "/"); i >= 0 {
		c.PGDatabase = strings.TrimSpace(hostport[i+1:])
		hostport = hostport[:i]
	}
	// host:port
	if i := strings.LastIndex(hostport, ":"); i >= 0 {
		c.PGHost = strings.Trim(hostport[:i], "[]")
		c.PGPort = hostport[i+1:]
	} else {
		c.PGHost = strings.Trim(hostport, "[]")
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
