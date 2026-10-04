package agent

import (
	"strings"
	"testing"
)

func TestPlanInterview(t *testing.T) {
	cases := []struct{ q, skill string }{
		{"介绍一下你自己", "自我介绍"},
		{"讲讲你最有意思的项目", "项目深挖"},
		{"你的技术栈有哪些", "面试应答"},
		{"这个岗位的要求我匹配吗", "岗位匹配度评估"},
	}
	for _, c := range cases {
		p := PlanInterview(c.q)
		if p.Skill != c.skill {
			t.Fatalf("问题 %q 命中 SKILL = %s，want %s", c.q, p.Skill, c.skill)
		}
		if p.Focus == "" {
			t.Fatalf("问题 %q 应给出考察点", c.q)
		}
	}
}

func TestSoulSystemPrompt(t *testing.T) {
	p := SoulSystemPrompt("知识片段A", "引用文件B", "当前任务C")
	for _, want := range []string{"JasperLee", "SKILL", "知识片段A", "引用文件B", "当前任务C"} {
		if !strings.Contains(p, want) {
			t.Fatalf("系统提示词缺少 %q", want)
		}
	}
}
