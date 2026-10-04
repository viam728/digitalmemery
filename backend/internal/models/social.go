package models

// SocialLink 数字分身的平台看板条目：媒体平台 / 博客 / 技术练习平台的账号与链接。
//
// Category 取值（前端按此分组渲染）：
//
//	code      代码 & 开源
//	community 技术社区
//	blog      博客 & 内容
//	practice  练习 & 竞赛
//	social    社交 & 职业
type SocialLink struct {
	ID       string `json:"id"`       // 稳定标识（如 github / juejin）
	Platform string `json:"platform"` // 平台名
	Category string `json:"category"` // 分类（见上）
	Account  string `json:"account"`  // 账号 / 昵称（展示）
	URL      string `json:"url"`      // 主页链接（可空）
	Note     string `json:"note"`     // 一句话说明（可空）
}
