// Package textract 提供常见办公文档的纯文本抽取（零第三方依赖）。
//
// 目前支持：docx（Word OOXML：解压 word/document.xml，按段落拼接文本）。
// 不支持：pdf（二进制格式复杂，保留原文下载，全文由对应 md 承载）。
//
// 抽取结果用于：资料库向量化入库、@引用上下文、收件箱自动应答阅读正文。
package textract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// DocxText 从 docx 字节流中抽取纯文本（按段落换行）。ok=false 表示解析失败或正文为空。
func DocxText(b []byte) (string, bool) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", false
	}
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", false
	}
	rc, err := doc.Open()
	if err != nil {
		return "", false
	}
	defer rc.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(rc)
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" { // w:t：文本运行
				inText = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p": // 段落结束 → 换行
				sb.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				sb.WriteString(string(t))
			}
		}
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", false
	}
	return out, true
}
