package storage

import "jasperlee/backend/internal/models"

// seedSocialLinks 平台看板的初始条目（一批常见平台，按分类）。
//
// 约定：账号 / 链接默认留空，由所有者在界面「双击卡片」就地填写；
// 已知信息（GitHub 用户名）预填。
func seedSocialLinks() []models.SocialLink {
	return []models.SocialLink{
		// —— 代码 & 开源 ——
		{ID: "github", Platform: "GitHub", Category: "code", Account: "viam728", URL: "https://github.com/viam728", Note: "开源项目与代码仓库"},
		{ID: "gitee", Platform: "Gitee 码云", Category: "code", Note: "国内代码托管"},
		{ID: "stackoverflow", Platform: "Stack Overflow", Category: "code", Note: "技术问答与声誉"},
		{ID: "dockerhub", Platform: "Docker Hub", Category: "code", Note: "镜像仓库"},

		// —— 技术社区 ——
		{ID: "juejin", Platform: "掘金", Category: "community", Note: "技术文章与分享"},
		{ID: "csdn", Platform: "CSDN", Category: "community", Note: "技术博客"},
		{ID: "segmentfault", Platform: "思否 SegmentFault", Category: "community", Note: "开发者问答社区"},
		{ID: "v2ex", Platform: "V2EX", Category: "community", Note: "创意工作者社区"},
		{ID: "cnblogs", Platform: "博客园", Category: "community", Note: "老牌技术博客"},
		{ID: "zhihu", Platform: "知乎", Category: "community", Note: "技术问答与专栏"},

		// —— 博客 & 内容 ——
		{ID: "blog", Platform: "个人博客", Category: "blog", Note: "自建技术博客"},
		{ID: "wechat-mp", Platform: "微信公众号", Category: "blog", Note: "技术文章与项目记录"},
		{ID: "yuque", Platform: "语雀", Category: "blog", Note: "知识库与文档"},
		{ID: "medium", Platform: "Medium", Category: "blog", Note: "英文技术写作"},

		// —— 练习 & 竞赛 ——
		{ID: "leetcode", Platform: "LeetCode 力扣", Category: "practice", Note: "算法题库"},
		{ID: "nowcoder", Platform: "牛客网", Category: "practice", Note: "笔试面试题库"},
		{ID: "codeforces", Platform: "Codeforces", Category: "practice", Note: "算法竞赛"},
		{ID: "luogu", Platform: "洛谷", Category: "practice", Note: "算法练习与竞赛"},
		{ID: "kaggle", Platform: "Kaggle", Category: "practice", Note: "数据科学竞赛"},

		// —— 社交 & 职业 ——
		{ID: "linkedin", Platform: "LinkedIn 领英", Category: "social", Note: "职业社交"},
		{ID: "x", Platform: "X (Twitter)", Category: "social", Note: "技术动态"},
		{ID: "bilibili", Platform: "哔哩哔哩", Category: "social", Note: "视频分享"},
		{ID: "xiaohongshu", Platform: "小红书", Category: "social", Note: "生活与技术分享"},
	}
}
