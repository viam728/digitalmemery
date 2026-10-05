package pgpool

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// TestPBKDF2SHA256 用公开测试向量校验 PBKDF2-HMAC-SHA256 实现。
func TestPBKDF2SHA256(t *testing.T) {
	cases := []struct {
		iter int
		want string
	}{
		{1, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{2, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{4096, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
	}
	for _, c := range cases {
		got := hex.EncodeToString(pbkdf2SHA256([]byte("password"), []byte("salt"), c.iter, 32))
		if got != c.want {
			t.Fatalf("pbkdf2 iter=%d = %s, want %s", c.iter, got, c.want)
		}
	}
}

// TestScramFlow 与服务端侧独立实现交叉校验 SCRAM 全流程（proof 与 server signature）。
func TestScramFlow(t *testing.T) {
	password := "pencil"
	salt := []byte("0123456789abcdef")
	iter := 4096

	c := &scramClient{password: password}
	first := c.clientFirst()
	attrs := parseAttrs(first[3:]) // 去掉 "n,,"
	nonce := attrs["r"] + "SERVERPART"
	serverFirst := "r=" + nonce + ",s=" + base64.StdEncoding.EncodeToString(salt) + ",i=4096"
	final, err := c.handleServerFirst(serverFirst)
	if err != nil {
		t.Fatal(err)
	}

	// 服务端校验 client-final 的 proof（独立计算）
	base := "c=" + base64.StdEncoding.EncodeToString([]byte("n,,")) + ",r=" + nonce
	authMessage := c.clientFirstBare + "," + serverFirst + "," + base
	salted := pbkdf2SHA256([]byte(password), salt, iter, 32)
	clientKey := hmacSHA256(salted, []byte("Client Key"))
	storedKey := sha256.Sum256(clientKey)
	clientSig := hmacSHA256(storedKey[:], []byte(authMessage))

	finalAttrs := parseAttrs(final)
	if finalAttrs["r"] != nonce {
		t.Fatalf("client-final nonce 异常: %s", finalAttrs["r"])
	}
	proof, err := base64.StdEncoding.DecodeString(finalAttrs["p"])
	if err != nil {
		t.Fatal(err)
	}
	recovered := make([]byte, len(proof))
	for i := range proof {
		recovered[i] = proof[i] ^ clientSig[i]
	}
	if !hmac.Equal(recovered, clientKey) {
		t.Fatal("服务端校验 proof 失败（客户端 SCRAM 实现有误）")
	}

	// server-final 校验
	serverKey := hmacSHA256(salted, []byte("Server Key"))
	serverSig := hmacSHA256(serverKey, []byte(authMessage))
	if err := c.verifyServerFinal("v=" + base64.StdEncoding.EncodeToString(serverSig)); err != nil {
		t.Fatal(err)
	}
	// 篡改的签名应被拒绝
	if err := c.verifyServerFinal("v=" + base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("错误签名应校验失败")
	}
}

func TestParseMechanisms(t *testing.T) {
	raw := []byte("SCRAM-SHA-256-PLUS\x00SCRAM-SHA-256\x00\x00")
	got := parseMechanisms(raw)
	if len(got) != 2 || got[0] != "SCRAM-SHA-256-PLUS" || got[1] != "SCRAM-SHA-256" {
		t.Fatalf("parseMechanisms = %v", got)
	}
	if !containsStr(got, "SCRAM-SHA-256") || containsStr(got, "PLAIN") {
		t.Fatal("containsStr 异常")
	}
}
