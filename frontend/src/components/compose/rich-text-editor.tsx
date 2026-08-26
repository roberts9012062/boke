// src/components/compose/rich-text-editor.tsx
// 富文本编辑器（所见即所得，写一帖 + 后台编辑共用）：
//   Tiptap 驱动，工具栏点按钮直接生效（无 Markdown 源码），支持图片上传、
//   视频内嵌（bilibili/YouTube/腾讯/Vimeo）与普通外链。受控组件：value 存 HTML 字符串。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";

import { apiUploadMedia, apiResolveQQMusic, apiPluginExtensions, apiPluginCall, authHeaders, ApiError } from "@/lib/api";
import { compressImage, readFileAsBase64 } from "@/lib/image-compress";
import { htmlToText } from "@/lib/rich-text";
import { parseMusicEmbed, qqPlayerURL } from "@/lib/music-embed";
import { parseVideoEmbed } from "@/lib/video-embed";
import { ImageLibraryPicker } from "@/components/compose/image-library-picker";
import { Modal } from "@/components/ui/modal";

import { VideoEmbed } from "./video-embed";
import { MusicEmbed } from "./music-embed";
import { BilibiliEmbed } from "./bilibili-embed";
import { BilibiliPicker } from "./bilibili-picker";

// isBilibiliInput 判断输入是否为 B 站地址（网页链接 / b23.tv 短链 / 纯 BV 号；纯函数）。
function isBilibiliInput(input: string): boolean {
  const u = input.trim();
  if (u === "") {
    return false;
  }
  return /bilibili\.com|b23\.tv/i.test(u) || /^BV[0-9A-Za-z]{10}$/.test(u);
}

// sanitizeUrl 仅允许 http/https 链接（防 javascript: 等危险协议；纯函数）。
function sanitizeUrl(url: string): string {
  const u = url.trim();
  if (/^https?:\/\//i.test(u)) {
    return u;
  }
  return "https://" + u;
}

// TOOLBAR 工具栏按钮（key 映射到 tiptap 命令）。
const TOOLBAR = [
  { label: "加粗", symbol: "B", cmd: "bold" },
  { label: "斜体", symbol: "I", cmd: "italic" },
  { label: "二级标题", symbol: "H2", cmd: "h2" },
  { label: "三级标题", symbol: "H3", cmd: "h3" },
  { label: "无序列表", symbol: "•≡", cmd: "bulletList" },
  { label: "有序列表", symbol: "1≡", cmd: "orderedList" },
  { label: "引用", symbol: "❝", cmd: "blockquote" },
  { label: "代码块", symbol: "{}", cmd: "codeBlock" },
  { label: "分割线", symbol: "—", cmd: "hr" },
] as const;

// RichTextEditorProps 编辑器参数。
interface RichTextEditorProps {
  value: string; // 受控 HTML
  onChange: (html: string) => void; // 变更回调
  placeholder: string;
  maxLength: number;
}

// NeteaseSong 网易云歌曲搜索结果（插件 /search 返回）。
interface NeteaseSong {
  id: number; // 歌曲 ID
  name: string; // 歌名
  artist: string; // 歌手
  album: string; // 专辑
  cover_url: string; // 封面
  duration: number; // 时长（毫秒）
}

// QqSong QQ 音乐歌曲搜索结果（插件 /search 返回）。
interface QqSong {
  song_mid: string; // 歌曲 MID（vkey 用）
  song_id: number; // 数字 ID
  name: string; // 歌名
  artist: string; // 歌手
  album: string; // 专辑
  cover: string; // 封面
}

// RichTextEditor 所见即所得富文本编辑器。
export function RichTextEditor({ value, onChange, placeholder, maxLength }: RichTextEditorProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const cfInputRef = useRef<HTMLInputElement>(null);
  const [imageOpen, setImageOpen] = useState(false);
  const [imageUrl, setImageUrl] = useState("");
  const [imageError, setImageError] = useState("");
  const [videoOpen, setVideoOpen] = useState(false);
  const [videoUrl, setVideoUrl] = useState("");
  const [videoError, setVideoError] = useState("");
  const [musicOpen, setMusicOpen] = useState(false);
  const [musicUrl, setMusicUrl] = useState("");
  const [musicError, setMusicError] = useState("");
  const [musicResolving, setMusicResolving] = useState(false);
  // 音乐搜索（M7 网易云 / M8 QQ 插件：Tab 切换，搜索结果插入音乐引用）
  const [musicTab, setMusicTab] = useState<"link" | "netease" | "qq">("link");
  const [neteaseQuery, setNeteaseQuery] = useState("");
  const [neteaseResults, setNeteaseResults] = useState<NeteaseSong[]>([]);
  const [neteaseLoading, setNeteaseLoading] = useState(false);
  const [neteaseError, setNeteaseError] = useState("");
  const [qqQuery, setQqQuery] = useState("");
  const [qqResults, setQqResults] = useState<QqSong[]>([]);
  const [qqLoading, setQqLoading] = useState(false);
  const [qqError, setQqError] = useState("");
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkText, setLinkText] = useState("");
  const [linkUrl, setLinkUrl] = useState("");
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  // B站视频插件是否 running（running 时 B 站地址走高清解析弹窗）
  const [bilibiliReady, setBilibiliReady] = useState(false);

  // 检测 B站视频插件可用性（公开扩展清单；失败静默回退通用 iframe 流程）
  useEffect(() => {
    apiPluginExtensions()
      .then((r) => {
        setBilibiliReady(r.items.some((it) => it.plugin_id === "bilibili-video"));
      })
      .catch(() => {
        setBilibiliReady(false);
      });
  }, []);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Image,
      Link.configure({ openOnClick: true, autolink: true }),
      VideoEmbed,
      MusicEmbed,
      BilibiliEmbed,
    ],
    content: value || "",
    editorProps: {
      attributes: {
        class: "prose-editor",
        "data-placeholder": placeholder,
      },
    },
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML());
    },
  });

  // 外部 value 变化（如编辑回填）时同步到编辑器（不含自身 onUpdate 触发的回环）
  useEffect(() => {
    if (!editor) {
      return;
    }
    const current = editor.getHTML();
    const normalized = current === "<p></p>" ? "" : current;
    if (value !== normalized) {
      editor.commands.setContent(value || "", false);
    }
  }, [value, editor]);

  // 运行工具栏命令（命令名映射到 tiptap chain）
  const runCommand = useCallback(
    (cmd: (typeof TOOLBAR)[number]["cmd"]) => {
      if (!editor) {
        return;
      }
      const chain = editor.chain().focus();
      switch (cmd) {
        case "bold":
          chain.toggleBold().run();
          break;
        case "italic":
          chain.toggleItalic().run();
          break;
        case "h2":
          chain.toggleHeading({ level: 2 }).run();
          break;
        case "h3":
          chain.toggleHeading({ level: 3 }).run();
          break;
        case "bulletList":
          chain.toggleBulletList().run();
          break;
        case "orderedList":
          chain.toggleOrderedList().run();
          break;
        case "blockquote":
          chain.toggleBlockquote().run();
          break;
        case "codeBlock":
          chain.toggleCodeBlock().run();
          break;
        case "hr":
          chain.setHorizontalRule().run();
          break;
      }
    },
    [editor],
  );

  // 图片本地上传（多选批量）：逐个压缩 → 媒体库（经存储接缝直达 R2）→ 逐个插入正文
  const handleImagePick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []);
    e.target.value = "";
    if (files.length === 0 || !editor) {
      return;
    }
    setUploading(true);
    setError("");
    try {
      // 先全部上传收集 URL，最后一次性插入（连续 setImage 每次 focus 重置光标到
      // 同一位置，多张图会互相顶掉——批量场景必须攒齐再插）
      const urls: string[] = [];
      for (const file of files) {
        setError(`本地上传中 ${urls.length + 1}/${files.length}：${file.name}`);
        const compressed = await compressImage(file);
        const result = await apiUploadMedia(compressed);
        urls.push(result.url);
      }
      insertImages(urls);
      setImageOpen(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "图片上传失败");
    } finally {
      setUploading(false);
      setError("");
    }
  };

  // CF图床直传（多选批量）：逐个压缩 → base64 → 插件 /manage/upload（服务端再按设置压缩）→ 插入正文。
  // 与本地上传的区别：只进 R2 不登记媒体库（不占帖子图集 9 张名额，仅正文引用）。
  const handleCfUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []);
    e.target.value = "";
    if (files.length === 0 || !editor) {
      return;
    }
    setUploading(true);
    setError("");
    try {
      const urls: string[] = [];
      for (const file of files) {
        setError(`CF图床上传中 ${urls.length + 1}/${files.length}：${file.name}`);
        const compressed = await compressImage(file);
        const b64 = await readFileAsBase64(compressed);
        const r = await apiPluginCall<{ url?: string; error?: string }>("image-cdn", "/manage/upload", {
          filename: file.name,
          mime: compressed.type,
          content_b64: b64,
        });
        if (r.error || !r.url) {
          throw new Error(r.error || "图床上传失败");
        }
        urls.push(r.url);
      }
      insertImages(urls);
      setImageOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "图床上传失败");
    } finally {
      setUploading(false);
      setError("");
    }
  };

  // insertImages 批量插入图片节点（一次事务插多个，避免连续 setImage 光标重置）。
  const insertImages = (urls: string[]) => {
    if (!editor || urls.length === 0) {
      return;
    }
    editor
      .chain()
      .focus()
      .insertContent(urls.map((url) => ({ type: "image", attrs: { src: url } })))
      .run();
  };

  // 图片外链：粘贴图片 URL 插入图片节点
  const confirmImageUrl = () => {
    const url = imageUrl.trim();
    if (!url) {
      setImageError("请输入图片地址");
      return;
    }
    if (!/^https?:\/\//i.test(url)) {
      setImageError("图片地址需以 http(s):// 开头");
      return;
    }
    editor?.chain().focus().setImage({ src: url }).run();
    setImageOpen(false);
    setImageUrl("");
    setImageError("");
  };

  // 视频内嵌：解析 URL → 插入 videoEmbed 节点
  const confirmVideo = () => {
    const parsed = parseVideoEmbed(videoUrl);
    if (!parsed) {
      setVideoError("无法识别该视频链接，支持 bilibili / YouTube / 腾讯视频 / Vimeo");
      return;
    }
    try {
      editor?.chain().focus().insertContent({
        type: "videoEmbed",
        attrs: { src: parsed.embedUrl, platform: parsed.platform },
      }).run();
    } catch (err) {
      setVideoError(err instanceof Error ? err.message : "视频插入失败");
      return;
    }
    setVideoOpen(false);
    setVideoUrl("");
    setVideoError("");
  };

  // 音乐内嵌：解析 URL → 插入 musicEmbed 节点。
  // 网易云单曲（#/song?id=）走自研播放器：解析 songId → 插件 /song 取详情 → 引用形态
  // （渲染侧经 /api/v1/music/netease-url 拿真实地址播放）；QQ/歌单/专辑走 iframe。
  const confirmMusic = async () => {
    const parsed = parseMusicEmbed(musicUrl);
    if (!parsed) {
      setMusicError("无法识别该音乐链接，支持网易云音乐 / QQ音乐");
      return;
    }
    // 网易云单曲：走自研播放器（引用形态）
    if (parsed.platform === "netease" && parsed.kind === "song" && parsed.songId) {
      setMusicResolving(true);
      setMusicError("");
      try {
        const res = await fetch("/api/v1/plugins/netease-music/song", {
          method: "POST",
          headers: { "Content-Type": "application/json", ...authHeaders() },
          body: JSON.stringify({ id: parsed.songId }),
        });
        const data = (await res.json()) as { song?: { name?: string; artist?: string; cover_url?: string }; error?: string };
        if (data.error) {
          setMusicError(data.error);
          return;
        }
        editor?.chain().focus().insertContent({
          type: "musicEmbed",
          attrs: {
            src: "",
            platform: "netease",
            kind: "song",
            songId: parsed.songId,
            title: data.song?.name ?? "",
            artist: data.song?.artist ?? "",
            cover: data.song?.cover_url ?? "",
          },
        }).run();
        setMusicOpen(false);
        setMusicUrl("");
        setMusicError("");
        return;
      } catch {
        setMusicError("获取网易云歌曲失败，请稍后再试");
        return;
      } finally {
        setMusicResolving(false);
      }
    }
    // QQ 单曲：走自研播放器（引用形态，songmid 经插件拿 vkey 地址）
    if (parsed.platform === "qq" && parsed.kind === "song" && parsed.songmid) {
      setMusicResolving(true);
      setMusicError("");
      try {
        const res = await fetch("/api/v1/plugins/qq-music/song", {
          method: "POST",
          headers: { "Content-Type": "application/json", ...authHeaders() },
          body: JSON.stringify({ songmid: parsed.songmid }),
        });
        const data = (await res.json()) as { song?: { name?: string; artist?: string; cover?: string }; error?: string };
        if (data.error) {
          setMusicError(data.error);
          return;
        }
        editor?.chain().focus().insertContent({
          type: "musicEmbed",
          attrs: {
            src: "",
            platform: "qq",
            kind: "song",
            songId: parsed.songmid,
            title: data.song?.name ?? "",
            artist: data.song?.artist ?? "",
            cover: data.song?.cover ?? "",
          },
        }).run();
        setMusicOpen(false);
        setMusicUrl("");
        setMusicError("");
        return;
      } catch {
        setMusicError("获取 QQ 音乐歌曲失败，请稍后再试");
        return;
      } finally {
        setMusicResolving(false);
      }
    }
    // 其他（网易云歌单/专辑）：走 iframe
    let embedUrl = parsed.embedUrl;
    if (parsed.platform === "qq" && parsed.songmid) {
      setMusicResolving(true);
      setMusicError("");
      try {
        const info = await apiResolveQQMusic(parsed.songmid);
        embedUrl = qqPlayerURL(info.songid);
      } catch (err) {
        setMusicError(err instanceof ApiError ? `QQ音乐解析失败：${err.message}` : "QQ音乐解析失败，请稍后再试");
        return;
      } finally {
        setMusicResolving(false);
      }
    }
    try {
      editor?.chain().focus().insertContent({
        type: "musicEmbed",
        attrs: { src: embedUrl, platform: parsed.platform, kind: parsed.kind },
      }).run();
    } catch (err) {
      setMusicError(err instanceof Error ? err.message : "音乐插入失败");
      return;
    }
    setMusicOpen(false);
    setMusicUrl("");
    setMusicError("");
  };

  // 网易云搜索（M7 插件）：调插件 /search 接口
  const searchNetease = async () => {
    const q = neteaseQuery.trim();
    if (!q) {
      setNeteaseError("请输入歌名");
      return;
    }
    setNeteaseLoading(true);
    setNeteaseError("");
    try {
      const res = await fetch("/api/v1/plugins/netease-music/search", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ q, limit: 10 }),
      });
      const data = (await res.json()) as { songs?: NeteaseSong[]; error?: string };
      if (data.error) {
        setNeteaseError(data.error);
        setNeteaseResults([]);
      } else {
        setNeteaseResults(data.songs ?? []);
      }
    } catch {
      setNeteaseError("搜索失败，请稍后再试");
      setNeteaseResults([]);
    } finally {
      setNeteaseLoading(false);
    }
  };

  // 插入网易云歌曲（引用形态：存 songId + 元信息，渲染侧自研播放器；封面缺失时补查详情）
  const insertNeteaseSong = async (song: NeteaseSong) => {
    if (!editor) return;
    let cover = song.cover_url || "";
    if (!cover) {
      // 搜索接口不返回封面，插入前补查歌曲详情拿封面
      try {
        const res = await fetch("/api/v1/plugins/netease-music/song", {
          method: "POST",
          headers: { "Content-Type": "application/json", ...authHeaders() },
          body: JSON.stringify({ id: song.id }),
        });
        const data = (await res.json()) as { song?: { cover_url?: string }; error?: string };
        if (data.song?.cover_url) cover = data.song.cover_url;
      } catch {
        /* 封面获取失败静默降级为占位 */
      }
    }
    editor.chain().focus().insertContent({
      type: "musicEmbed",
      attrs: {
        src: "",
        platform: "netease",
        kind: "song",
        songId: String(song.id),
        title: song.name,
        artist: song.artist,
        cover,
      },
    }).run();
    setMusicOpen(false);
    setNeteaseQuery("");
    setNeteaseResults([]);
  };

  // QQ 音乐搜索（M8 插件）：调插件 /search 接口
  const searchQq = async () => {
    const q = qqQuery.trim();
    if (!q) {
      setQqError("请输入歌名");
      return;
    }
    setQqLoading(true);
    setQqError("");
    try {
      const res = await fetch("/api/v1/plugins/qq-music/search", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ q, limit: 10 }),
      });
      const data = (await res.json()) as { songs?: QqSong[]; error?: string };
      if (data.error) {
        setQqError(data.error);
        setQqResults([]);
      } else {
        setQqResults(data.songs ?? []);
      }
    } catch {
      setQqError("搜索失败，请稍后再试");
      setQqResults([]);
    } finally {
      setQqLoading(false);
    }
  };

  // 插入 QQ 音乐歌曲（引用形态：存 songmid + 元信息，渲染侧自研播放器）
  const insertQqSong = (song: QqSong) => {
    if (!editor) return;
    editor.chain().focus().insertContent({
      type: "musicEmbed",
      attrs: {
        src: "",
        platform: "qq",
        kind: "song",
        songId: song.song_mid,
        title: song.name,
        artist: song.artist,
        cover: song.cover,
      },
    }).run();
    setMusicOpen(false);
    setQqQuery("");
    setQqResults([]);
  };

  // 外链：对选区 setLink；无选区且填了文字则插入链接文本
  const confirmLink = () => {
    if (!editor) {
      return;
    }
    const url = sanitizeUrl(linkUrl);
    if (editor.state.selection.empty) {
      const text = linkText.trim() || url;
      editor.chain().focus().insertContent({ type: "text", text, marks: [{ type: "link", attrs: { href: url } }] }).run();
    } else {
      editor.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
    }
    setLinkOpen(false);
    setLinkText("");
    setLinkUrl("");
  };

  const charCount = editor ? htmlToText(editor.getHTML()).length : 0;

  return (
    <div>
      {/* 工具栏 */}
      <div className="flex flex-wrap items-center gap-1 border-b border-line pb-2">
        <button type="button" title="撤销" aria-label="撤销" onClick={() => editor?.chain().focus().undo().run()} className="min-w-8 rounded-md border border-line px-2 py-1 text-sm text-ink-2 hover:border-accent hover:text-ink">↶</button>
        <button type="button" title="重做" aria-label="重做" onClick={() => editor?.chain().focus().redo().run()} className="min-w-8 rounded-md border border-line px-2 py-1 text-sm text-ink-2 hover:border-accent hover:text-ink">↷</button>
        <span className="mx-1 h-4 w-px bg-line" aria-hidden />
        {TOOLBAR.map((t) => (
          <button
            key={t.cmd}
            type="button"
            title={t.label}
            aria-label={t.label}
            onClick={() => runCommand(t.cmd)}
            className={`min-w-8 rounded-md border px-2 py-1 text-sm transition-colors ${
              editor?.isActive(t.cmd === "h2" ? "heading" : t.cmd === "h3" ? "heading" : t.cmd)
                ? "border-accent bg-accent-soft text-glow"
                : "border-line text-ink-2 hover:border-accent hover:text-ink"
            }`}
          >
            {t.symbol}
          </button>
        ))}
        <span className="mx-1 h-4 w-px bg-line" aria-hidden />
        <button type="button" onClick={() => { setImageOpen(true); setImageError(""); }} className="rounded-md border border-line px-2 py-1 text-sm text-ink-2 hover:border-accent hover:text-ink">
          🖼 图片
        </button>
        <button type="button" onClick={() => { setVideoOpen(true); setVideoError(""); }} className="rounded-md border border-line px-2 py-1 text-sm text-ink-2 hover:border-accent hover:text-ink">
          ▶ 视频
        </button>
        <button type="button" onClick={() => { setMusicOpen(true); setMusicError(""); }} className="rounded-md border border-line px-2 py-1 text-sm text-ink-2 hover:border-accent hover:text-ink">
          ♪ 音乐
        </button>
        <button type="button" onClick={() => setLinkOpen(true)} className="rounded-md border border-line px-2 py-1 text-sm text-ink-2 hover:border-accent hover:text-ink">
          🔗 链接
        </button>
      </div>

      {/* 编辑器画布 */}
      <div className="mt-2 min-h-[192px] rounded-lg border border-line bg-elevated p-3">
        <EditorContent editor={editor} />
      </div>

      {/* 隐藏图片选择（本地批量 / CF图床批量，均 multiple） */}
      <input ref={fileInputRef} type="file" accept="image/*" multiple className="hidden" onChange={(e) => void handleImagePick(e)} />
      <input ref={cfInputRef} type="file" accept="image/*" multiple className="hidden" onChange={(e) => void handleCfUpload(e)} />

      {error && <p className="mt-1 text-xs text-like">{error}</p>}

      {/* 字数统计 */}
      <p className="mt-1 text-right text-xs text-ink-3">{charCount} / {maxLength}</p>

      {/* 图片弹窗（本地上传 / CF图床直传 / 图库选择 / 外链 URL） */}
      <Modal open={imageOpen} title="插入图片" onClose={() => { setImageOpen(false); setImageError(""); }} maxWidth="max-w-[420px]">
        <div className="space-y-4">
          {/* 双通道上传：本地（进媒体库+图集）与 CF图床直传（仅正文引用） */}
          <div className="grid grid-cols-2 gap-2">
            <button
              type="button"
              disabled={uploading}
              onClick={() => fileInputRef.current?.click()}
              className="flex h-20 w-full flex-col items-center justify-center gap-1 rounded-lg border border-dashed border-line text-xs text-ink-3 transition-colors hover:border-accent hover:text-ink disabled:opacity-60"
            >
              <span className="text-lg" aria-hidden>⬆</span>
              {uploading ? "上传中…" : "本地上传（多选）"}
              <span className="text-[10px] text-ink-3">进媒体库 · 可作图集</span>
            </button>
            <button
              type="button"
              disabled={uploading}
              onClick={() => cfInputRef.current?.click()}
              className="flex h-20 w-full flex-col items-center justify-center gap-1 rounded-lg border border-dashed border-line text-xs text-ink-3 transition-colors hover:border-accent hover:text-ink disabled:opacity-60"
            >
              <span className="text-lg" aria-hidden>☁</span>
              {uploading ? "上传中…" : "CF图床上传（多选）"}
              <span className="text-[10px] text-ink-3">直传 R2 · 仅正文引用</span>
            </button>
          </div>
          {(error || imageError) && <p className="text-xs text-like" role="status">{error || imageError}</p>}

          {/* 分隔 */}
          <div className="flex items-center gap-2 text-xs text-ink-3">
            <span className="h-px flex-1 bg-line" />
            或
            <span className="h-px flex-1 bg-line" />
          </div>

          {/* CF图床图库选择（插件 running 时可用；选中插入正文） */}
          <ImageLibraryPicker
            onPick={(url) => {
              editor?.chain().focus().setImage({ src: url }).run();
              setImageOpen(false);
            }}
            onClose={() => setImageOpen(false)}
          />

          {/* 分隔 */}
          <div className="flex items-center gap-2 text-xs text-ink-3">
            <span className="h-px flex-1 bg-line" />
            或使用外链
            <span className="h-px flex-1 bg-line" />
          </div>

          {/* 外链 URL */}
          <div>
            <label className="block">
              <span className="text-xs text-ink-3">图片地址</span>
              <input
                value={imageUrl}
                onChange={(e) => setImageUrl(e.target.value)}
                placeholder="https://example.com/image.png"
                className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
              />
            </label>
            {imageError && <p className="mt-1 text-xs text-like">{imageError}</p>}
            <div className="mt-3 flex justify-end gap-2">
              <button type="button" onClick={() => setImageOpen(false)} className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink">取消</button>
              <button type="button" onClick={confirmImageUrl} className="rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent hover:opacity-90">插入外链</button>
            </div>
          </div>
        </div>
      </Modal>

      {/* 视频内嵌弹窗（B站地址且插件 running 时切换为高清解析弹窗） */}
      <Modal open={videoOpen} title="插入视频" onClose={() => { setVideoOpen(false); setVideoError(""); }} maxWidth="max-w-[460px]">
        {bilibiliReady && isBilibiliInput(videoUrl) ? (
          <BilibiliPicker
            defaultUrl={videoUrl}
            onClose={() => { setVideoOpen(false); setVideoUrl(""); setVideoError(""); }}
            onInsert={(attrs) => {
              editor?.chain().focus().insertContent({ type: "bilibiliEmbed", attrs }).run();
              setVideoOpen(false);
              setVideoUrl("");
              setVideoError("");
            }}
          />
        ) : (
        <div className="space-y-3">
          <p className="text-xs text-ink-3">
            {bilibiliReady ? "粘贴视频链接（B站地址将进入高清解析；也支持 YouTube / 腾讯视频 / Vimeo）" : "粘贴视频链接，支持 bilibili / YouTube / 腾讯视频 / Vimeo，将内嵌播放器"}
          </p>
          <input
            autoFocus
            value={videoUrl}
            onChange={(e) => setVideoUrl(e.target.value)}
            placeholder="https://www.bilibili.com/video/BV..."
            className="w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
          />
          {videoError && <p className="text-xs text-like">{videoError}</p>}
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setVideoOpen(false)} className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink">取消</button>
            <button type="button" onClick={confirmVideo} className="rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent hover:opacity-90">插入</button>
          </div>
        </div>
        )}
      </Modal>

      {/* 音乐内嵌弹窗 */}
      <Modal open={musicOpen} title="插入音乐" onClose={() => { setMusicOpen(false); setMusicError(""); setMusicTab("link"); setNeteaseError(""); }} maxWidth="max-w-[420px]">
        <div className="space-y-3">
          {/* Tab 切换：粘贴链接 / 网易云搜索（M7 插件） */}
          <div className="flex gap-2 border-b border-line pb-2">
            <button
              type="button"
              onClick={() => { setMusicTab("link"); setMusicError(""); }}
              className={`rounded-full px-4 py-1 text-xs transition-colors ${musicTab === "link" ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"}`}
            >
              粘贴链接
            </button>
            <button
              type="button"
              onClick={() => { setMusicTab("netease"); setNeteaseError(""); }}
              className={`rounded-full px-4 py-1 text-xs transition-colors ${musicTab === "netease" ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"}`}
            >
              网易云搜索
            </button>
            <button
              type="button"
              onClick={() => { setMusicTab("qq"); setQqError(""); }}
              className={`rounded-full px-4 py-1 text-xs transition-colors ${musicTab === "qq" ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"}`}
            >
              QQ 搜索
            </button>
          </div>

          {musicTab === "link" ? (
            <>
              <p className="text-xs text-ink-3">粘贴音乐链接，支持网易云音乐 / QQ音乐，将内嵌播放器（QQ音乐自动解析歌曲）</p>
              <input
                autoFocus
                value={musicUrl}
                onChange={(e) => setMusicUrl(e.target.value)}
                placeholder="https://music.163.com/#/song?id=..."
                className="w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
              />
              {musicError && <p className="text-xs text-like">{musicError}</p>}
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setMusicOpen(false)} className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink">取消</button>
                <button type="button" disabled={musicResolving} onClick={() => void confirmMusic()} className="rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60">
                  {musicResolving ? "解析中…" : "插入"}
                </button>
              </div>
            </>
          ) : musicTab === "qq" ? (
            <>
              <p className="text-xs text-ink-3">搜索 QQ 音乐歌曲，点击插入（需先在后台上登录 QQ 音乐）</p>
              <div className="flex gap-2">
                <input
                  autoFocus
                  value={qqQuery}
                  onChange={(e) => setQqQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") void searchQq(); }}
                  placeholder="输入歌名，如：晴天"
                  className="h-9 flex-1 rounded-lg border border-line bg-elevated px-3 text-sm text-ink outline-none focus:border-accent"
                />
                <button type="button" disabled={qqLoading} onClick={() => void searchQq()} className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90 disabled:opacity-60">
                  {qqLoading ? "搜索中…" : "搜索"}
                </button>
              </div>
              {qqError && <p className="text-xs text-like">{qqError}</p>}
              <div className="max-h-60 space-y-1 overflow-y-auto">
                {qqResults.map((song) => (
                  <div key={song.song_mid} className="flex items-center gap-2 rounded-lg border border-line px-3 py-2">
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm text-ink">{song.name} - {song.artist}</p>
                      <p className="truncate text-xs text-ink-3">{song.album}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => insertQqSong(song)}
                      className="shrink-0 rounded-full bg-accent-soft px-3 py-1 text-xs text-glow hover:opacity-90"
                    >
                      插入
                    </button>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <>
              <p className="text-xs text-ink-3">搜索网易云歌曲，点击插入（需先在后台上登录网易云，免费歌可直接播放）</p>
              <div className="flex gap-2">
                <input
                  autoFocus
                  value={neteaseQuery}
                  onChange={(e) => setNeteaseQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") void searchNetease(); }}
                  placeholder="输入歌名，如：海阔天空"
                  className="h-9 flex-1 rounded-lg border border-line bg-elevated px-3 text-sm text-ink outline-none focus:border-accent"
                />
                <button type="button" disabled={neteaseLoading} onClick={() => void searchNetease()} className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90 disabled:opacity-60">
                  {neteaseLoading ? "搜索中…" : "搜索"}
                </button>
              </div>
              {neteaseError && <p className="text-xs text-like">{neteaseError}</p>}
              <div className="max-h-60 space-y-1 overflow-y-auto">
                {neteaseResults.map((song) => (
                  <div key={song.id} className="flex items-center gap-2 rounded-lg border border-line px-3 py-2">
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm text-ink">{song.name} - {song.artist}</p>
                      <p className="truncate text-xs text-ink-3">{song.album}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => void insertNeteaseSong(song)}
                      className="shrink-0 rounded-full bg-accent-soft px-3 py-1 text-xs text-glow hover:opacity-90"
                    >
                      插入
                    </button>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </Modal>

      {/* 外链弹窗 */}
      <Modal open={linkOpen} title="插入链接" onClose={() => setLinkOpen(false)} maxWidth="max-w-[420px]">
        <div className="space-y-3">
          <label className="block">
            <span className="text-xs text-ink-3">链接文字（未选中文字时生效）</span>
            <input value={linkText} onChange={(e) => setLinkText(e.target.value)} placeholder="链接文字" className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent" />
          </label>
          <label className="block">
            <span className="text-xs text-ink-3">链接地址</span>
            <input value={linkUrl} onChange={(e) => setLinkUrl(e.target.value)} placeholder="https://..." className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent" />
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setLinkOpen(false)} className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink">取消</button>
            <button type="button" onClick={confirmLink} className="rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent hover:opacity-90">插入</button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
