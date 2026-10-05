// SCRAM-SHA-256（RFC 5802 / 7677）客户端实现：PostgreSQL 13+ 的默认认证方式。
// 仅实现客户端一侧（SCRAM-SHA-256，无 channel binding），覆盖 PG 默认配置。
package pgpool

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// scramClient 一次 SCRAM-SHA-256 协商的客户端状态。
type scramClient struct {
	user     string
	password string

	clientNonce     string
	clientFirstBare string
	serverFirst     string
	serverSignature []byte
}

// parseMechanisms 解析 AuthenticationSASL 的机制列表（null 分隔、空串结尾）。
func parseMechanisms(b []byte) []string {
	var out []string
	for len(b) > 0 {
		i := bytes.IndexByte(b, 0)
		if i <= 0 {
			break
		}
		out = append(out, string(b[:i]))
		b = b[i+1:]
	}
	return out
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// randomScramNonce 生成不含逗号的客户端随机串（base64）。
func randomScramNonce() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "noncefallback0000"
	}
	return base64.StdEncoding.EncodeToString(b)
}

// clientFirst 生成 client-first-message（"n,,n=,r=<nonce>"；用户名已在 Startup 中发送）。
func (c *scramClient) clientFirst() string {
	c.clientNonce = randomScramNonce()
	c.clientFirstBare = "n=,r=" + c.clientNonce
	return "n,," + c.clientFirstBare
}

// parseAttrs 解析 SCRAM 消息中的 k=v 属性（逗号分隔；值内可含 '='）。
func parseAttrs(msg string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(msg, ",") {
		i := strings.IndexByte(part, '=')
		if i <= 0 {
			continue
		}
		out[part[:i]] = part[i+1:]
	}
	return out
}

// handleServerFirst 处理 server-first-message，返回 client-final-message。
func (c *scramClient) handleServerFirst(msg string) (string, error) {
	attrs := parseAttrs(msg)
	nonce := attrs["r"]
	saltB64 := attrs["s"]
	iterS := attrs["i"]
	if nonce == "" || saltB64 == "" || iterS == "" {
		return "", fmt.Errorf("pgpool: SCRAM server-first 缺少必要字段")
	}
	if !strings.HasPrefix(nonce, c.clientNonce) {
		return "", fmt.Errorf("pgpool: SCRAM nonce 不匹配")
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return "", fmt.Errorf("pgpool: SCRAM salt 解码失败: %w", err)
	}
	iter, err := strconv.Atoi(iterS)
	if err != nil || iter <= 0 {
		return "", fmt.Errorf("pgpool: SCRAM 迭代次数非法: %q", iterS)
	}

	c.serverFirst = msg
	salted := pbkdf2SHA256([]byte(c.password), salt, iter, sha256.Size)
	clientKey := hmacSHA256(salted, []byte("Client Key"))
	storedKey := sha256.Sum256(clientKey)
	clientFinalWithoutProof := "c=" + base64.StdEncoding.EncodeToString([]byte("n,,")) + ",r=" + nonce
	authMessage := c.clientFirstBare + "," + c.serverFirst + "," + clientFinalWithoutProof
	clientSignature := hmacSHA256(storedKey[:], []byte(authMessage))
	proof := make([]byte, len(clientKey))
	for i := range clientKey {
		proof[i] = clientKey[i] ^ clientSignature[i]
	}
	serverKey := hmacSHA256(salted, []byte("Server Key"))
	c.serverSignature = hmacSHA256(serverKey, []byte(authMessage))

	return clientFinalWithoutProof + ",p=" + base64.StdEncoding.EncodeToString(proof), nil
}

// verifyServerFinal 校验 server-final-message 的服务器签名。
func (c *scramClient) verifyServerFinal(msg string) error {
	attrs := parseAttrs(msg)
	if e := attrs["e"]; e != "" {
		return fmt.Errorf("pgpool: SCRAM 服务端错误: %s", e)
	}
	v := attrs["v"]
	if v == "" {
		return fmt.Errorf("pgpool: SCRAM server-final 缺少签名")
	}
	expected := base64.StdEncoding.EncodeToString(c.serverSignature)
	if v != expected {
		return fmt.Errorf("pgpool: SCRAM 服务器签名校验失败")
	}
	return nil
}

// ---- 加密原语 ----

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

// pbkdf2SHA256 标准 PBKDF2-HMAC-SHA256（RFC 8018）。
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	dk := make([]byte, 0, numBlocks*hashLen)
	u := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		prf.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u = prf.Sum(u[:0])
		t := make([]byte, hashLen)
		copy(t, u)
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for x := range t {
				t[x] ^= u[x]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

// ---- 发送（'p' 消息） ----

// sendSASLInitial 发送 SASLInitialResponse（机制名 + 首包）。
func (c *Conn) sendSASLInitial(mechanism string, initial []byte) error {
	var b []byte
	b = append(b, mechanism...)
	b = append(b, 0)
	b = appendInt32(b, int32(len(initial)))
	b = append(b, initial...)
	return c.writeMsg('p', b)
}

// sendSASLResponse 发送 SASLResponse（client-final）。
func (c *Conn) sendSASLResponse(data []byte) error {
	return c.writeMsg('p', data)
}
