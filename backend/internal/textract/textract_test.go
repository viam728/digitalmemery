package textract

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func buildDocx(t *testing.T, documentXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const sampleDoc = `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>第一段内容</w:t></w:r></w:p><w:p><w:r><w:t>任职要求：熟悉 </w:t></w:r><w:r><w:t>Go</w:t></w:r></w:p></w:body></w:document>`

func TestDocxText(t *testing.T) {
	text, ok := DocxText(buildDocx(t, sampleDoc))
	if !ok {
		t.Fatal("合法 docx 应可解析")
	}
	if !strings.Contains(text, "第一段内容") {
		t.Fatalf("缺少第一段：%q", text)
	}
	if !strings.Contains(text, "任职要求：熟悉 Go") {
		t.Fatalf("跨 run 文本应拼接：%q", text)
	}
}

func TestDocxInvalid(t *testing.T) {
	if _, ok := DocxText([]byte("not a zip")); ok {
		t.Fatal("非 zip 输入应返回 false")
	}
	if _, ok := DocxText(buildDocx(t, "<empty/>")); ok {
		t.Fatal("无文本内容应返回 false")
	}
}
