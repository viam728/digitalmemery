package agent

import (
	"context"
	"fmt"
	"strings"

	"jasperlee/backend/internal/llm"
)

// 面试场景的专业工作流：按「考察点识别 → 简历定位 → STAR 组织 → 岗位关联」推进。
//
// 成熟 Agent 范式做法：把工作流拆成可观测的步骤（plan → act → observe），
// 每一步都有明确输入输出；本实现把步骤落成可展示的 step 事件文本，随产物一起返回。

// InterviewStep 面试工作流的一个步骤（可观测）
type InterviewStep struct {
	Name    string `json:"name"`
	Detail  string `json:"detail"`
	Elapsed int64  `json:"elapsedMs,omitempty"`
}

// InterviewPlan 面试应答计划
type InterviewPlan struct {
	Focus    string   `json:"focus"`    // 考察点
	Projects []string `json:"projects"` // 选用的真实项目
	Skill    string   `json:"skill"`    // 命中的 SKILL
}

// PlanInterview 为面试问题生成应答计划（纯规则，可解释、无需调模型）
func PlanInterview(question string) InterviewPlan {
	q := strings.ToLower(question)
	plan := InterviewPlan{Skill: "面试应答"}
	switch {
	case containsAny(q, []string{"introduc", "介绍", "自我"}):
		plan.Focus = "自我介绍与主线经历"
		plan.Skill = "自我介绍"
		plan.Projects = []string{"InkBloom", "BeYoung"}
	case containsAny(q, []string{"project", "项目", "做过", "代表作", "最有意思"}):
		plan.Focus = "项目经验与技术深度"
		plan.Skill = "项目深挖"
		plan.Projects = pickProjects(q)
	case containsAny(q, []string{"skill", "技能", "擅长", "stack", "技术栈"}):
		plan.Focus = "技能全景与擅长方向"
		plan.Projects = []string{"InkBloom", "LLM 检索增强问答系统"}
	case containsAny(q, []string{"jd", "匹配", "岗位", "合适", "要求"}):
		plan.Focus = "岗位匹配度"
		plan.Skill = "岗位匹配度评估"
		plan.Projects = pickProjects(q)
	case containsAny(q, []string{"agent", "harness", "rag", "llm", "模型"}):
		plan.Focus = "Agent 工程能力"
		plan.Skill = "项目深挖"
		plan.Projects = []string{"InkBloom", "LLM 检索增强问答系统"}
	case containsAny(q, []string{"go", "后端", "并发", "微服务", "架构"}):
		plan.Focus = "后端工程能力"
		plan.Skill = "项目深挖"
		plan.Projects = []string{"BeYoung", "InkBloom"}
	case containsAny(q, []string{"contact", "联系", "电话", "邮箱", "微信"}):
		plan.Focus = "联系方式"
		plan.Projects = nil
	default:
		plan.Focus = "综合问答"
		plan.Projects = []string{"InkBloom"}
	}
	return plan
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func pickProjects(q string) []string {
	var out []string
	if containsAny(q, []string{"inkbloom", "aigc", "创作", "harness", "sse"}) {
		out = append(out, "InkBloom")
	}
	if containsAny(q, []string{"beyoung", "电商", "独立站", "b2b", "价格", "询价"}) {
		out = append(out, "BeYoung")
	}
	if containsAny(q, []string{"rag", "检索", "问答", "知识库", "langchain"}) {
		out = append(out, "LLM 检索增强问答系统")
	}
	if containsAny(q, []string{"订货", "d-s", "证据", "数学"}) {
		out = append(out, "D-S 证据理论订货决策工具")
	}
	if len(out) == 0 {
		out = []string{"InkBloom", "BeYoung"}
	}
	return out
}

// RunInterview 执行面试应答工作流：计划 → 组装灵魂提示词 → 流式生成 → 产出评估文件。
// steps 返回可观测的步骤记录；reply 为完整回复；usage 为用量。
func RunInterview(ctx context.Context, client *llm.Client, model, question, knowledge, refs string, emit func(string)) (steps []InterviewStep, reply string, usage llm.Usage, err error) {
	plan := PlanInterview(question)
	steps = append(steps, InterviewStep{
		Name:   "识别考察点",
		Detail: fmt.Sprintf("考察点：%s；命中 SKILL：%s", plan.Focus, plan.Skill),
	})
	if len(plan.Projects) > 0 {
		steps = append(steps, InterviewStep{
			Name:   "定位简历项目",
			Detail: "选用真实项目：" + strings.Join(plan.Projects, "、"),
		})
	}
	task := fmt.Sprintf("面试问题：%s\n应答计划：考察点[%s]，选用项目[%s]，使用 SKILL[%s]，按 STAR 结构第一人称作答。",
		question, plan.Focus, strings.Join(plan.Projects, "、"), plan.Skill)
	sys := SoulSystemPrompt(knowledge, refs, task)
	steps = append(steps, InterviewStep{Name: "组装上下文", Detail: "已注入灵魂 + SKILL + 资料片段 + 引用文件"})

	var sb strings.Builder
	usage, err = client.Stream(ctx, llm.StreamRequest{
		Model:    model,
		Messages: messagesWithSystem(sys, question),
		Hits:     nil,
	}, func(delta string) {
		sb.WriteString(delta)
		emit(delta)
	})
	if err != nil {
		return steps, "", usage, err
	}
	steps = append(steps, InterviewStep{Name: "生成应答", Detail: "流式输出完成"})
	return steps, sb.String(), usage, nil
}

// messagesWithSystem 兼容旧 StreamRequest 形状的辅助：返回完整消息列表。
// 注意：StreamRequest.Messages 为 []ChatMessage，此处直接构造 system+user 两条。
func messagesWithSystem(sys, question string) []llm.ChatMessage {
	return []llm.ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: question},
	}
}

// InterviewReport 生成面试问答的评估产物（markdown，可暴露到 Jasper 的空间）
func InterviewReport(question, reply string, plan InterviewPlan) string {
	var sb strings.Builder
	sb.WriteString("# 面试问答记录\n\n")
	sb.WriteString("## 问题\n\n" + question + "\n\n")
	sb.WriteString("## 应答\n\n" + reply + "\n\n")
	sb.WriteString("## 工作流\n\n")
	sb.WriteString(fmt.Sprintf("- 考察点：%s\n- 命中 SKILL：%s\n- 选用项目：%s\n",
		plan.Focus, plan.Skill, strings.Join(plan.Projects, "、")))
	return sb.String()
}
