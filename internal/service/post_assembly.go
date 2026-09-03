// internal/service/post_assembly.go
// 帖子「读路径」组装：摘要/详情组装（作者、标签、媒体、收藏数）。
// 说明（M1.7 拆分）：post.go 超 400 行规范，将组装逻辑独立成文件，职责单一：
//   - post.go：写路径（CRUD/时间线/详情可见性/媒体上传/标签同步）
//   - post_assembly.go：读路径（PostSummary/PostDetail 的字段组装）
package service

import (
	"context"
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/pkg/errs"
)

// assembleSummaries 批量组装帖子摘要（作者/标签/媒体 + 私密帖过滤 + 收藏数聚合）。
func (s *PostService) assembleSummaries(ctx context.Context, posts []model.Post, viewerID int64) ([]model.PostSummary, error) {
	summaries := make([]model.PostSummary, 0, len(posts))
	for _, post := range posts {
		// 私密帖过滤：非作者不展示
		if post.Visibility == model.VisibilityPrivate && post.AuthorID != viewerID {
			continue
		}
		// 仅关注者帖过滤：需与作者互相关注（设计稿：互相关注的人可见）
		if post.Visibility == model.VisibilityFollowers && post.AuthorID != viewerID {
			if viewerID == 0 || !s.isMutual(ctx, viewerID, post.AuthorID) {
				continue
			}
		}
		summary := model.PostSummary{
			ID:           post.ID,
			Title:        post.Title,
			Summary:      summaryPreviewText(post.Content, post.PostKind),
			ContentType:  post.ContentType,
			PostKind:     post.PostKind,
			Visibility:   post.Visibility,
			GalleryStyle: post.GalleryStyle,
			Music:        extractMusicEmbed(post.Content),
			Bilibili:     extractBilibiliEmbed(post.Content),
			LikeCount:    post.LikeCount,
			CommentCount: post.CommentCount,
			ViewCount:    post.ViewCount,
		}
		if post.PublishedAt != nil {
			summary.PublishedAt = post.PublishedAt.Format(time.RFC3339)
		}
		if err := s.fillAuthor(ctx, &summary, post.AuthorID); err != nil {
			return nil, err
		}
		if err := s.fillTags(ctx, &summary, post.ID); err != nil {
			return nil, err
		}
		if err := s.fillMedia(ctx, &summary, post.MediaIDs); err != nil {
			return nil, err
		}
		// 形态兜底（列表读路径）：
		//   1) content_type=text 但媒体库关联了图片（插件早期发布硬编码 text）→ 归一为 image；
		//   2) media 为空但正文内嵌 <img>（TG 图床通道：图片直链进正文、不关联媒体库）
		//      → 提取正文图片填入 media，时间线网格才能显示。
		applyImageFallback(&summary, post.Content)
		summaries = append(summaries, summary)
	}
	// 批量补齐收藏数（M1.7 技术债修复：一次查询填充全部摘要，替代逐条 N+1）
	if err := s.fillFavoriteCounts(ctx, summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

// fillFavoriteCounts 批量填充摘要收藏数（post_reactions 一次聚合）。
func (s *PostService) fillFavoriteCounts(ctx context.Context, summaries []model.PostSummary) error {
	ids := make([]int64, 0, len(summaries))
	for _, summary := range summaries {
		ids = append(ids, summary.ID)
	}
	counts, err := s.posts.CountFavoritesByPosts(ctx, ids)
	if err != nil {
		return err
	}
	for i := range summaries {
		summaries[i].FavoriteCount = counts[summaries[i].ID]
	}
	return nil
}

// fillAuthor 填充作者信息（复用用户仓库）。
func (s *PostService) fillAuthor(ctx context.Context, summary *model.PostSummary, authorID int64) error {
	user, err := s.users.FindByID(ctx, authorID)
	if err != nil {
		return err
	}
	summary.Author = model.AuthorDTO{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
	}
	return nil
}

// fillTags 填充标签列表。
func (s *PostService) fillTags(ctx context.Context, summary *model.PostSummary, postID int64) error {
	tags, err := s.tags.ListByPost(ctx, postID)
	if err != nil {
		return err
	}
	summary.Tags = make([]model.TagDTO, 0, len(tags))
	for _, tag := range tags {
		summary.Tags = append(summary.Tags, model.TagDTO{Name: "#" + tag.Name, Slug: tag.Slug})
	}
	return nil
}

// fillMedia 填充媒体列表（按 media_ids 顺序，灯箱多图有序）。
func (s *PostService) fillMedia(ctx context.Context, summary *model.PostSummary, mediaIDs []int64) error {
	assets, err := s.medias.FindByIDs(ctx, mediaIDs)
	if err != nil {
		return err
	}
	summary.Media = make([]model.MediaDTO, 0, len(assets))
	for _, asset := range assets {
		summary.Media = append(summary.Media, model.MediaDTO{
			ID:        asset.ID,
			Type:      asset.Type,
			URL:       asset.URL,
			MimeType:  asset.MimeType,
			SizeBytes: asset.SizeBytes,
			Width:     asset.Width,
			Height:    asset.Height,
		})
	}
	return nil
}

// ---------- 列表形态兜底（图片说说归一 + 正文图片提取） ----------

// 正文 <img> 的 src 属性（插件发布序列化固定双引号；src 在前在后均可命中）。
var imgSrcRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)

// applyImageFallback 列表摘要形态兜底（仅 text 帖，不覆盖显式 audio/video）：
//   - 媒体库已关联图片 → content_type 归一为 image（插件早期发布带图说说硬编码 text 的历史修复）；
//   - 媒体库为空但正文内嵌 <img>（TG 图床通道）→ 提取正文图片合成 media 并归一 image。
func applyImageFallback(summary *model.PostSummary, content string) {
	if summary.ContentType != "text" {
		return
	}
	if len(summary.Media) > 0 {
		summary.ContentType = derivedImageType(summary.Media)
		return
	}
	summary.Media = extractContentImages(content)
	if len(summary.Media) > 0 {
		summary.ContentType = "image"
	}
}

// derivedImageType 媒体全为图片时返回 image，否则保持 text（纯函数；
// 混合媒体不推导——音频帖误传 text 的场景维持原形态）。
func derivedImageType(media []model.MediaDTO) string {
	for _, m := range media {
		if m.Type != "image" {
			return "text"
		}
	}
	return "image"
}

// extractContentImages 从正文 HTML 提取 <img> 图片列表（纯函数，合成 MediaDTO 供时间线网格渲染；
// 与 extractMusicEmbed/extractBilibiliEmbed 同构：正文嵌入提取 → 列表卡片渲染。
// ID 置 0（不在媒体库），URL 反转义实体（正文序列化 & → &amp;，同音乐嵌入处理）。
func extractContentImages(content string) []model.MediaDTO {
	matches := imgSrcRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	images := make([]model.MediaDTO, 0, len(matches))
	for _, m := range matches {
		images = append(images, model.MediaDTO{
			ID:   0,
			Type: "image",
			URL:  html.UnescapeString(m[1]),
		})
	}
	return images
}

// GetAdminDetail 后台编辑详情（设计稿《后台编辑》四画板：编辑区/发布信息/互动数据/操作）。
// 与前台 GetDetail 的区别：不校验可见性（管理员可查看下架/私密帖）、不增加浏览量。
func (s *PostService) GetAdminDetail(ctx context.Context, postID int64) (*model.AdminPostDetail, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	if post.Status == model.PostStatusDeleted {
		return nil, errs.ErrNotFound
	}

	// 作者 / 标签 / 媒体（复用前台组装，避免重复实现）
	var summary model.PostSummary
	if err := s.fillAuthor(ctx, &summary, post.AuthorID); err != nil {
		return nil, err
	}
	if err := s.fillTags(ctx, &summary, post.ID); err != nil {
		return nil, err
	}
	if err := s.fillMedia(ctx, &summary, post.MediaIDs); err != nil {
		return nil, err
	}
	// 标签去掉 # 前缀（表单直接编辑，保存时再补 #）
	tagNames := make([]string, 0, len(summary.Tags))
	for _, tag := range summary.Tags {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name, "#"))
	}

	detail := &model.AdminPostDetail{
		ID:            post.ID,
		Title:         post.Title,
		Content:       post.Content,
		ContentFormat: post.ContentFormat,
		ContentType:   post.ContentType,
		PostKind:      post.PostKind,
		Status:        post.Status,
		Visibility:    post.Visibility,
		CoverURL:      post.CoverURL,
		Tags:          tagNames,
		Media:         summary.Media,
		ViewCount:     post.ViewCount,
		LikeCount:     post.LikeCount,
		CommentCount:  post.CommentCount,
		Author:        summary.Author,
		CreatedAt:     post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     post.UpdatedAt.Format(time.RFC3339),
	}
	if post.PublishedAt != nil {
		detail.PublishedAt = post.PublishedAt.Format(time.RFC3339)
	}
	return detail, nil
}

// isMutual 判断 viewer 与 author 是否互相关注（仅关注者帖可见性，设计稿「互相关注的人可见」）。
func (s *PostService) isMutual(ctx context.Context, viewerID int64, authorID int64) bool {
	following, err := s.relations.IsFollowing(ctx, viewerID, authorID)
	if err != nil || !following {
		return false
	}
	followedBack, err := s.relations.IsFollowing(ctx, authorID, viewerID)
	return err == nil && followedBack
}

// summaryPreviewText 列表摘要（剥 HTML + 剥 Markdown 标记 + 替换换行为空格，省略号结尾）。
// 长度按帖子形态：说说前 60 字符；文章前 200 字符（时间轴展示 200 字，内容过长可省略）。
// 说明：必须走 plainText（先剥 HTML）——历史修复：只 stripMarkdown 时富文本帖子的
//       <div>/<iframe> 等标签源码会泄漏进摘要（前端列表显示成一堆代码）。
func summaryPreviewText(content string, kind string) string {
	flat := strings.Join(strings.Fields(plainText(content)), " ")
	limit := summaryPreview
	if kind == model.PostKindArticle {
		limit = articlePreviewLen
	}
	runes := []rune(flat)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return flat
}

// 音乐嵌入节点正则：<div data-music-embed="qq|netease" ...>（正文序列化格式固定）。
var (
	musicEmbedDivRe    = regexp.MustCompile(`(?s)<div[^>]*data-music-embed="(qq|netease)"`)
	musicEmbedKindRe   = regexp.MustCompile(`data-music-kind="([^"]+)"`)
	musicEmbedIframeRe = regexp.MustCompile(`(?s)<iframe[^>]*src="([^"]+)"`)
	musicEmbedIDRe     = regexp.MustCompile(`data-music-id="([^"]+)"`)
	musicEmbedTitleRe  = regexp.MustCompile(`data-music-title="([^"]*)"`)
	musicEmbedArtistRe = regexp.MustCompile(`data-music-artist="([^"]*)"`)
	musicEmbedCoverRe  = regexp.MustCompile(`data-music-cover="([^"]*)"`)
)

// extractMusicEmbed 从正文 HTML 提取首个音乐嵌入（列表卡片渲染迷你播放器用；纯函数）。
// 两种形态：
//   - 第三方 iframe：<div data-music-embed="qq" data-music-kind="song"><iframe src="..."></iframe></div>
//   - 网易云引用：<div data-music-embed="netease" data-music-kind="song" data-music-id="..." data-music-title="..." ...>
// 返回 nil 表示正文无音乐嵌入（普通文字/图片/视频帖不解析）。
func extractMusicEmbed(content string) *model.MusicEmbedDTO {
	loc := musicEmbedDivRe.FindStringSubmatchIndex(content)
	if loc == nil {
		return nil
	}
	platform := content[loc[2]:loc[3]]
	// 仅取嵌入 div 之后的内容，避免误取正文其他 iframe（如视频嵌入）
	rest := content[loc[1]:]
	kind := "song"
	if m := musicEmbedKindRe.FindStringSubmatch(rest); m != nil {
		kind = m[1]
	}
	// 网易云歌曲引用形态（data-music-id 存在，无 iframe；播放地址实时经插件获取）
	songID := ""
	if m := musicEmbedIDRe.FindStringSubmatch(rest); m != nil {
		songID = m[1]
	}
	if songID != "" {
		title := firstAttr(musicEmbedTitleRe, rest)
		artist := firstAttr(musicEmbedArtistRe, rest)
		cover := firstAttr(musicEmbedCoverRe, rest)
		return &model.MusicEmbedDTO{
			Platform: platform, Kind: kind, SongID: songID,
			Title: html.UnescapeString(title), Artist: html.UnescapeString(artist), CoverURL: html.UnescapeString(cover),
		}
	}
	// 第三方 iframe 形态（QQ 音乐/旧网易云）
	src := ""
	if m := musicEmbedIframeRe.FindStringSubmatch(rest); m != nil {
		src = m[1]
	}
	if src == "" {
		return nil
	}
	// 反转义实体（正文序列化时 & 会写成 &amp;，前端 React 属性渲染不做 HTML 解码，
	// 必须在这里还原成干净 URL，否则带参播放器链接失效）
	return &model.MusicEmbedDTO{Platform: platform, Kind: kind, URL: html.UnescapeString(src)}
}

// firstAttr 取正则首个捕获组（无匹配返回空串；纯函数）。
func firstAttr(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// ---------- B站视频块提取（bilibili-video 插件，时间线列表渲染播放器） ----------

// bilibiliBlockDivRe B站内容块开标签（data-plugin-block="bilibili"）。
var bilibiliBlockDivRe = regexp.MustCompile(`(?s)<div[^>]*data-plugin-block="bilibili"[^>]*>`)

// dataPropsRe 块参数属性（值内引号已被序列化为 &quot;，无裸引号）。
var dataPropsRe = regexp.MustCompile(`data-props="([^"]*)"`)

// extractBilibiliEmbed 从正文 HTML 提取首个 B站视频块的参数 JSON（纯函数）。
// 序列化形态：<div data-plugin-block="bilibili" data-props="{&quot;bvid&quot;:...}"></div>
// 返回 nil 表示正文无 B站块（props 非法 JSON 同样返回 nil，详情页按占位处理）。
func extractBilibiliEmbed(content string) json.RawMessage {
	loc := bilibiliBlockDivRe.FindStringIndex(content)
	if loc == nil {
		return nil
	}
	raw := firstAttr(dataPropsRe, content[loc[0]:loc[1]])
	if raw == "" {
		return nil
	}
	props := json.RawMessage(html.UnescapeString(raw))
	if !json.Valid(props) {
		return nil
	}
	return props
}
