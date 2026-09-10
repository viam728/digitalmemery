package llm

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ModelInfo 可选模型
type ModelInfo struct {
	ID      string `json:"id"`
	OwnedBy string `json:"ownedBy,omitempty"`
}

// modelCache GLM /models 列表缓存（key 包含的模型全配进去，供切换）
var modelCache = struct {
	sync.Mutex
	models  []ModelInfo
	updated time.Time
}{}

// ListModels 拉取 OpenAI 兼容 /models 列表。hostIP 非空时直连该 IP（绕过被污染的 DNS）。
// 失败返回 err，调用方回退到配置的默认模型。
func ListModels(baseURL, apiKey, hostIP string) ([]ModelInfo, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://open.bigmodel.cn/api/paas/v4"
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		// GLM 证书是配给域名的；直连 IP 时跳过主机名校验（仅用于本机 DNS 污染场景）
		TLSClientConfig: &tls.Config{InsecureSkipVerify: hostIP != ""}, //nolint:gosec
	}
	if hostIP != "" {
		if h, _, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")); err == nil && h != "" {
			_ = h
		}
		// 把 base 中的 host 替换为 IP，保留路径；同时发 Host 头保证网关路由
		base = replaceHost(base, hostIP)
	}
	client := &http.Client{Timeout: 20 * time.Second, Transport: transport}
	req, err := http.NewRequest(http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if hostIP != "" {
		req.Header.Set("Host", "open.bigmodel.cn")
		req.Host = "open.bigmodel.cn"
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models status %d", resp.StatusCode)
	}
	var out struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(out.Data))
	for _, m := range out.Data {
		models = append(models, ModelInfo{ID: m.ID, OwnedBy: m.OwnedBy})
	}
	modelCache.Lock()
	modelCache.models = models
	modelCache.updated = time.Now()
	modelCache.Unlock()
	log.Printf("[llm] fetched %d models from %s", len(models), baseURL)
	return models, nil
}

// CachedModels 返回缓存的模型列表（可能为空）
func CachedModels() []ModelInfo {
	modelCache.Lock()
	defer modelCache.Unlock()
	return append([]ModelInfo(nil), modelCache.models...)
}

// replaceHost 把 URL 中的 host 替换为 IP（保留 scheme/port/path）
func replaceHost(rawURL, ip string) string {
	// rawURL 形如 https://open.bigmodel.cn/api/paas/v4
	scheme := "https://"
	rest := rawURL
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

// RefreshModels 后台刷新一次模型列表（失败仅打日志，不阻塞启动）
func RefreshModels(baseURL, apiKey, hostIP string) {
	if apiKey == "" {
		return
	}
	if _, err := ListModels(baseURL, apiKey, hostIP); err != nil {
		log.Printf("[llm] refresh models failed: %v", err)
	}
}
