package service

import "testing"

// TestPlainForWorld 发布出口纯文本化：HTML 剥标签、实体还原、markdown 原样。
func TestPlainForWorld(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"纯文本原样", "今晚月色真不错", "今晚月色真不错"},
		{"markdown 原样", "# 标题\n**加粗**", "# 标题\n**加粗**"},
		{"剥段落标签", "<p>测试下哈哈哈</p>", "测试下哈哈哈"},
		{"剥多标签并压缩空白", "<p>你好</p>\n<br/><b>世界</b>", "你好 世界"},
		{"标签内实体随剥离还原", "<p>a &amp; b</p>", "a & b"},
		{"脚本全剥", "<script>alert(1)</script>正文", "alert(1)正文"},
		{"无左尖括号快速返回", "plain text", "plain text"},
		{"内嵌图转markdown", `看图<img src="https://a.com/x.png" alt="图">完毕`, `看图 ![图片](https://a.com/x.png) 完毕`},
		{"内嵌音视频转链接", `<video src="https://a.com/v.mp4" controls></video><audio src="https://a.com/a.mp3"></audio>`, `[视频](https://a.com/v.mp4)[音频](https://a.com/a.mp3)`},
	}
	for _, tc := range cases {
		if got := plainForWorld(tc.in); got != tc.want {
			t.Errorf("[%s] plainForWorld(%q) = %q，期望 %q", tc.name, tc.in, got, tc.want)
		}
	}
}
