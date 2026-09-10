package rag

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"jasperlee/backend/internal/models"
)

// replaceHostIP 把 base URL 的 host 替换为直连 IP（保留 scheme/path）
func replaceHostIP(base, ip string) string {
	scheme := "https://"
	rest := base
	if strings.HasPrefix(rest, "http://") {
		scheme = "http://"
		rest = strings.TrimPrefix(rest, "http://")
	} else {
		rest = strings.TrimPrefix(rest, "https://")
	}
	if i := strings.Index(rest, "/"); i >= 0 {
		return scheme + ip + rest[i:]
	}
	return scheme + ip
}

func insecureTransport(skipVerify bool) *http.Transport {
	if !skipVerify {
		return http.DefaultTransport.(*http.Transport)
	}
	//nolint:gosec
	return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
}

// EmbeddingProvider 向量化提供方抽象，便于在云 API 与本地模型之间切换
//
//   - glm：智谱 embedding-3（复用 GLM Key，免费额度，推荐）
//   - aliyun：阿里云百炼「通义 Text-Embedding-V3」+ DashVector（需 ALIYUN_API_KEY）
//   - local：本地 BGE-M3（FlagEmbedding），零 API 成本、需本地算力
//   - mock：无 Embedding，纯关键词打分，用于开发调试
type EmbeddingProvider interface {
	Name() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// VectorStore 向量库抽象
type VectorStore interface {
	// Search 按查询文本与向量检索 Top-K，返回命中的 chunk（按语义相似度排序）
	Search(ctx context.Context, query string, queryVec []float32, topK int) ([]models.RagHit, error)
	// Index 写入一条向量
	Index(ctx context.Context, hit models.RagHit, vec []float32) error
}

// Service RAG 服务：分块、向量化、检索
type Service struct {
	embed EmbeddingProvider
	store VectorStore
}

// New 构造 RAG 服务
//
// provider 取值："glm" | "aliyun" | "local" | "mock"
// dataDir 索引持久化目录（glm/local/mock 的内存向量库落盘于此，重启后仍可检索）
// embedKey / embedBaseURL 向量化服务密钥与端点（glm 复用模型 Key）
func New(provider, dataDir, embedKey, embedBaseURL string) *Service {
	var e EmbeddingProvider
	var s VectorStore

	switch provider {
	case "glm":
		e = &GLMEmbedding{apiKey: embedKey, baseURL: embedBaseURL}
		s = newMemStore(dataDir)
		log.Println("[rag] provider=glm (embedding-3 语义向量)")
	case "aliyun":
		e = &AliyunEmbedding{}
		s = &AliyunVectorStore{}
		log.Println("[rag] provider=aliyun (DashVector + 通义 Embedding)")
	case "local":
		e = &LocalBGE{}
		s = newMemStore(dataDir)
		log.Println("[rag] provider=local (BGE-M3 本地向量)")
	default:
		e = &MockEmbedding{}
		s = newMemStore(dataDir)
		log.Println("[rag] provider=mock (关键词打分, 开发用)")
	}
	return &Service{embed: e, store: s}
}

// Chunk 把文本切成定长块，块间保留重叠
func Chunk(text string, size, overlap int) []string {
	if size <= 0 {
		size = 500
	}
	if overlap < 0 {
		overlap = 50
	}
	var chunks []string
	runes := []rune(text)
	for i := 0; i < len(runes); i += size - overlap {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}
	return chunks
}

// Ingest 文档入库：分块 + 向量化 + 索引
func (svc *Service) Ingest(ctx context.Context, fileID, fileName, content string) error {
	chunks := Chunk(content, 500, 50)
	vecs, err := svc.embed.Embed(ctx, chunks)
	if err != nil {
		return err
	}
	for i, ch := range chunks {
		var v []float32
		if i < len(vecs) {
			v = vecs[i]
		}
		_ = svc.store.Index(ctx, models.RagHit{
			FileID:   fileID,
			FileName: fileName,
			Chunk:    ch,
		}, v)
	}
	return nil
}

// Query 检索 Top-K
func (svc *Service) Query(ctx context.Context, query string, topK int) []models.RagHit {
	if topK <= 0 {
		topK = 3
	}
	vecs, err := svc.embed.Embed(ctx, []string{query})
	if err != nil {
		log.Printf("[rag] embed failed: %v", err)
		return nil
	}
	var qv []float32
	if len(vecs) > 0 {
		qv = vecs[0]
	}
	hits, err := svc.store.Search(ctx, query, qv, topK)
	if err != nil {
		return nil
	}
	return hits
}

// ---------- 实现 ----------

// GLMEmbedding 智谱 embedding-3（复用 GLM API Key，OpenAI 兼容 /embeddings）
type GLMEmbedding struct {
	apiKey  string
	baseURL string
	hostIP  string
}

func (g *GLMEmbedding) Name() string { return "glm" }

// SetHostIP 设置直连 IP（本机 DNS 被污染时绕过解析，与 llm 共用同一机制）
func (g *GLMEmbedding) SetHostIP(ip string) { g.hostIP = ip }

func (g *GLMEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if g.apiKey == "" {
		return make([][]float32, len(texts)), nil
	}
	base := g.baseURL
	if base == "" {
		base = "https://open.bigmodel.cn/api/paas/v4"
	}
	url := base + "/embeddings"
	if g.hostIP != "" {
		url = replaceHostIP(base, g.hostIP) + "/embeddings"
	}
	body, _ := json.Marshal(map[string]any{
		"model": "embedding-3",
		"input": texts,
	})
	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
		if g.hostIP != "" {
			req.Header.Set("Host", "open.bigmodel.cn")
			req.Host = "open.bigmodel.cn"
		}
		return req, nil
	}
	req, err := newReq()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second, Transport: insecureTransport(g.hostIP != "")}
	resp, err := client.Do(req)
	if err != nil {
		// 一次重试（网络抖动兜底）
		req2, err2 := newReq()
		if err2 != nil {
			return nil, err
		}
		resp, err = client.Do(req2)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &apiErr{status: resp.StatusCode}
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	vecs := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

type apiErr struct{ status int }

func (e *apiErr) Error() string { return "embedding api status " + itoa(e.status) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// MockEmbedding 无向量、基于关键词打分（开发调试用，零成本）
type MockEmbedding struct{}

func (MockEmbedding) Name() string { return "mock" }

func (MockEmbedding) Embed(_ context.Context, texts []string) ([][]float32, error) {
	return make([][]float32, len(texts)), nil
}

// LocalBGE 本地 BGE-M3 占位实现；接入时替换为 FlagEmbedding 推理
type LocalBGE struct{}

func (LocalBGE) Name() string { return "local" }

func (LocalBGE) Embed(_ context.Context, texts []string) ([][]float32, error) {
	// TODO: import "github.com/FlagOpen/FlagEmbedding" 并调用推理
	return make([][]float32, len(texts)), nil
}

// AliyunEmbedding 阿里云百炼通义 Embedding；需配置 ALIYUN_API_KEY/ENDPOINT
type AliyunEmbedding struct{}

func (AliyunEmbedding) Name() string { return "aliyun" }

func (AliyunEmbedding) Embed(_ context.Context, texts []string) ([][]float32, error) {
	log.Println("[rag] aliyun embedding stub: 请配置 ALIYUN_API_KEY / ALIYUN_ENDPOINT")
	return make([][]float32, len(texts)), nil
}

// AliyunVectorStore 阿里云 DashVector 向量库占位实现
type AliyunVectorStore struct{}

func (AliyunVectorStore) Search(_ context.Context, _ string, _ []float32, topK int) ([]models.RagHit, error) {
	return []models.RagHit{}, nil
}

func (AliyunVectorStore) Index(_ context.Context, _ models.RagHit, _ []float32) error { return nil }

// MemVectorStore 内存向量库：优先按余弦相似度排序；无向量时按关键词重叠兜底。
// 索引与向量以 JSON 落盘（dataDir/rag_index.json），重启自动加载。
type MemVectorStore struct {
	mu    sync.Mutex
	path  string
	items []models.RagHit
	vecs  [][]float32
}

func newMemStore(dataDir string) *MemVectorStore {
	m := &MemVectorStore{path: filepath.Join(dataDir, "rag_index.json")}
	m.load()
	return m
}

func (m *MemVectorStore) load() {
	b, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	var data struct {
		Items []models.RagHit `json:"items"`
		Vecs  [][]float32     `json:"vecs"`
	}
	if json.Unmarshal(b, &data) == nil {
		m.items = data.Items
		m.vecs = data.Vecs
	}
}

func (m *MemVectorStore) flush() {
	_ = os.MkdirAll(filepath.Dir(m.path), 0o755)
	if b, err := json.Marshal(map[string]any{"items": m.items, "vecs": m.vecs}); err == nil {
		_ = os.WriteFile(m.path, b, 0o644)
	}
}

// tokenize 简单分词：拆词 + 去重（关键词兜底用）
func tokenize(s string) map[string]bool {
	tokens := make(map[string]bool)
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == '，' || r == '。' || r == '、' || r == '：' || r == '？' || r == '!'
	}) {
		if len(w) >= 2 {
			tokens[w] = true
		}
	}
	return tokens
}

// Search 语义优先：有真实向量则余弦相似度排序；否则关键词重叠兜底
func (m *MemVectorStore) Search(_ context.Context, query string, qv []float32, topK int) ([]models.RagHit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) == 0 {
		return nil, nil
	}
	if topK > len(m.items) {
		topK = len(m.items)
	}

	type scored struct {
		hit models.RagHit
		sc  float64
	}
	scoredList := make([]scored, 0, len(m.items))
	useVec := len(qv) > 0 && len(m.vecs) == len(m.items)

	for i, it := range m.items {
		sc := 0.0
		if useVec {
			sc = cosine(qv, m.vecs[i])
		} else {
			qt := tokenize(query)
			overlap := 0
			for w := range qt {
				if tokenize(it.Chunk)[w] {
					overlap++
				}
			}
			sc = float64(overlap)
		}
		scoredList = append(scoredList, scored{hit: it, sc: sc})
	}
	sort.SliceStable(scoredList, func(i, j int) bool { return scoredList[i].sc > scoredList[j].sc })

	out := make([]models.RagHit, 0, topK)
	for _, s := range scoredList {
		if len(out) >= topK {
			break
		}
		s.hit.Score = s.sc
		out = append(out, s.hit)
	}
	return out, nil
}

// Index 写入一条命中片段与向量并落盘
func (m *MemVectorStore) Index(_ context.Context, hit models.RagHit, vec []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = append(m.items, hit)
	m.vecs = append(m.vecs, vec)
	m.flush()
	return nil
}

// cosine 余弦相似度（归一化点积）
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
