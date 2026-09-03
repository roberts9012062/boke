// internal/service/post_assembly_test.go
// 列表形态兜底单测：图片说说归一（content_type=text → image）与正文 <img> 提取。
// 场景来源：浏览器插件发布带图说说历史缺陷——TG 图床通道图片只内嵌正文不关联媒体库，
// 且 content_type 硬编码 text，导致首页时间线（渲染条件 image+media 非空）不显示图片。
package service

import (
	"testing"

	"github.com/roberts9012062/boke/internal/model"
)

// 测试用图片媒体（ID 模拟媒体库自增）。
func imageMedia(id int64) model.MediaDTO {
	return model.MediaDTO{ID: id, Type: "image", URL: "https://cdn.example.com/a.png"}
}

func TestApplyImageFallback_MediaLinked(t *testing.T) {
	// 插件服务器通道历史数据：content_type=text 但媒体库关联了图片 → 归一 image
	summary := model.PostSummary{ContentType: "text", Media: []model.MediaDTO{imageMedia(1), imageMedia(2)}}
	applyImageFallback(&summary, "纯文字正文")
	if summary.ContentType != "image" {
		t.Fatalf("期望 content_type 归一为 image，实际 %q", summary.ContentType)
	}
	if len(summary.Media) != 2 {
		t.Fatalf("期望 media 保持 2 张，实际 %d", len(summary.Media))
	}
}

func TestApplyImageFallback_ContentImages(t *testing.T) {
	// TG 图床通道：media 为空、图片内嵌正文 → 提取正文图片并归一 image
	content := `<p>测试下</p><p><img src="https://tg.example.com/f/abc?x=1&amp;y=2" alt="说说配图" loading="lazy"></p>`
	summary := model.PostSummary{ContentType: "text", Media: []model.MediaDTO{}}
	applyImageFallback(&summary, content)
	if summary.ContentType != "image" {
		t.Fatalf("期望 content_type 归一为 image，实际 %q", summary.ContentType)
	}
	if len(summary.Media) != 1 {
		t.Fatalf("期望提取 1 张正文图片，实际 %d", len(summary.Media))
	}
	// URL 实体必须反转义（&amp; → &），否则带参图片链接失效
	if summary.Media[0].URL != "https://tg.example.com/f/abc?x=1&y=2" {
		t.Fatalf("期望 URL 反转义，实际 %q", summary.Media[0].URL)
	}
}

func TestApplyImageFallback_PlainTextUntouched(t *testing.T) {
	// 纯文字说说：无媒体无正文图片 → 维持 text
	summary := model.PostSummary{ContentType: "text", Media: []model.MediaDTO{}}
	applyImageFallback(&summary, "<p>只有文字</p>")
	if summary.ContentType != "text" || len(summary.Media) != 0 {
		t.Fatalf("期望纯文字帖不被改写，实际 content_type=%q media=%d", summary.ContentType, len(summary.Media))
	}
}

func TestApplyImageFallback_AudioNotDerived(t *testing.T) {
	// 显式 audio/video 帖不参与推导（音频帖误传 text 且媒体为音频 → 维持 text）
	summary := model.PostSummary{
		ContentType: "text",
		Media:       []model.MediaDTO{{ID: 1, Type: "audio", URL: "https://cdn.example.com/a.mp3"}},
	}
	applyImageFallback(&summary, "音频正文")
	if summary.ContentType != "text" {
		t.Fatalf("期望混合/音频媒体不推导为 image，实际 %q", summary.ContentType)
	}
}

func TestExtractContentImages_Multiple(t *testing.T) {
	// 多图正文：按出现顺序提取全部 <img>（九宫格最多 9 张由前端截断）
	content := `<p>a</p><img src="https://x.example/1.png"><p>b</p><img src="https://x.example/2.png"><img src="https://x.example/3.png">`
	images := extractContentImages(content)
	if len(images) != 3 {
		t.Fatalf("期望提取 3 张，实际 %d", len(images))
	}
	if images[0].URL != "https://x.example/1.png" || images[2].URL != "https://x.example/3.png" {
		t.Fatalf("提取顺序或 URL 不符：%v", images)
	}
}
