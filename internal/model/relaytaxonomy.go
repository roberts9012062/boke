// internal/model/relaytaxonomy.go
// 中继站「分类 → 规范标签」注册表：发帖选择中继站分类/标签、大世界筛选共用。
// 说明：中继站分类由远端 handshake 动态下发，本地无法预知全集；本表为本地规范
//       （12 个常见内容分类、约百个规范标签），远端自定义分类不在表内时仅做
//       数量/长度校验，不做白名单校验（见 service.ValidatePublishChoice）。
package model

// relayTaxonomy 分类 → 规范标签（有序；顺序即前端展示顺序）。
// 标签命名规范：2~6 个汉字为主、不带 # 前缀、避免同义重复（如「前端」与「前端开发」只留前者）。
var relayTaxonomy = map[string][]string{
	"技术": {
		"前端", "后端", "全栈", "人工智能", "编程语言", "数据库",
		"DevOps", "网络安全", "移动开发", "开源项目",
	},
	"生活": {
		"日常记录", "情感心事", "家居布置", "职场见闻", "健康养生",
		"宠物萌照", "消费避坑", "生活技巧",
	},
	"随笔": {
		"所思所想", "日记", "散文", "诗歌", "短篇故事",
		"人生感悟", "碎碎念", "书信体",
	},
	"摄影": {
		"风光摄影", "人像摄影", "街拍纪实", "胶片", "手机摄影",
		"后期调色", "器材分享", "星空拍摄",
	},
	"音乐": {
		"乐评", "原创音乐", "翻唱演奏", "歌单分享", "现场演出",
		"乐器学习", "音乐制作",
	},
	"影视": {
		"电影影评", "剧集讨论", "动漫", "纪录片", "观影清单", "幕后花絮",
	},
	"读书": {
		"书评", "读书笔记", "摘抄金句", "书单推荐", "古典文学", "社科人文",
	},
	"游戏": {
		"游戏体验", "通关攻略", "独立游戏", "主机游戏", "手游", "桌游", "游戏杂谈",
	},
	"美食": {
		"探店记录", "家常菜谱", "烘焙甜点", "地方美食", "下厨房", "食材知识",
	},
	"旅行": {
		"游记", "旅行攻略", "徒步露营", "自驾", "CityWalk", "风景在路上", "省钱旅行",
	},
	"创作": {
		"绘画", "设计", "写作", "视频创作", "手作DIY", "书法", "播客",
	},
	"科学": {
		"科普", "物理", "天文", "生物", "历史", "心理学", "数学",
	},
}

// relayTaxonomyOrder 分类展示顺序（map 无序，前端列表需要稳定顺序）。
var relayTaxonomyOrder = []string{
	"技术", "生活", "随笔", "创作", "读书", "影视",
	"音乐", "游戏", "摄影", "美食", "旅行", "科学",
}

// RelayTaxonomy 返回规范表副本（纯函数：调用方修改不影响注册表）。
func RelayTaxonomy() map[string][]string {
	out := make(map[string][]string, len(relayTaxonomy))
	for category, tags := range relayTaxonomy {
		cp := make([]string, len(tags))
		copy(cp, tags)
		out[category] = cp
	}
	return out
}

// RelayTaxonomyCategories 返回规范表分类（有序副本）。
func RelayTaxonomyCategories() []string {
	out := make([]string, len(relayTaxonomyOrder))
	copy(out, relayTaxonomyOrder)
	return out
}

// RelayTaxonomyTags 返回指定分类的规范标签（有序副本；分类不在表内返回 nil）。
func RelayTaxonomyTags(category string) []string {
	tags, ok := relayTaxonomy[category]
	if !ok {
		return nil
	}
	cp := make([]string, len(tags))
	copy(cp, tags)
	return cp
}

// RelayTaxonomyHasTag 判断标签是否属于指定分类的规范标签。
func RelayTaxonomyHasTag(category string, tag string) bool {
	for _, t := range relayTaxonomy[category] {
		if t == tag {
			return true
		}
	}
	return false
}
