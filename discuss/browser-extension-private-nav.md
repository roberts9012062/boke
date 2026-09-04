# 方案：书签私有导航（同步目标可选私有 + 公有/私有两 Tab + 密码门禁）

> 讨论稿 · 对应开发：插件 v0.27.0 批次追加。前提：站点端私有导航已上线（nav-links v1.3.14+ + 宿主开放网关三端点，见 `nav-links-private-nav.md` §8）。

## 1. 需求（用户口径）

1. 「同步到站点」时可选择把书签同步到**私有导航**（站点端 `navlinks.save` 已预留 `visibility:"private"` 透传）；
2. 书签页增加**公有导航 / 私有导航**两个 Tab 切换；
3. 切到私有导航需**输入访问密码**——密码在站点设置；未设置时给出提示引导用户自行去站点设置，设置后配置自动同步过来；输入正确密码即可浏览私有导航。

## 2. 前提调研（结论）

| 结论 | 出处 |
|---|---|
| `navlinks.private.list` GET `/api/v1/open/nav/private/links`：Key 授权直通，响应与 navlinks.list 同构（links/categories/tags/settings），仅含私有条目 | `internal/handler/nav_bridge.go` OpenPrivateList |
| `navlinks.private.config` GET `/api/v1/open/nav/private/config`：返回 `{mode, has_password, title, subtitle, count}`（mode：self=仅自己可见 / password=密码访问；无密钥材料） | 同上 OpenPrivateConfigGet |
| `navlinks.private.save` POST `/api/v1/open/nav/private/config`：`{mode, password?, title?, subtitle?}` | 同上 OpenPrivateConfigSave |
| `navlinks.save` link 载含 `visibility` 字段（空=开放；private=私有） | 同上 navSaveLink |
| 访客密码校验通道：`POST /api/v1/nav/private/unlock`（公开、无 Key）body `{password}` → 200 `{token,expires_at}` / 401 `{code:"bad_password"}` / 403 `{code:"self_only"}`；**响应为直通 JSON，非开放网关信封** | `router.go:189`、`marketplace-repo/nav-links/private.go` |
| 插件 `SiteNavLink` 类型无 visibility、endpoints 无私有封装、书签页单视图——插件端为零起点 | `src/shared/types/index.ts`、`endpoints.ts`、`BookmarksTab.tsx` |

## 3. 设计

### 3.1 同步目标选择（公有/私有）

- `SyncToSiteSheet` 在模式选择上方新增「同步目标」单选组：🌐 **公开导航**（默认）/ 🔒 **私有导航**；
- `runSyncToSite` 入参增加 `visibility: NavVisibility`（显式必传）；`visibility === 'private'` 时每条载荷携带 `visibility: "private"`（开放条目省略字段，站点端 omitempty 默认开放）；
- 完成文案区分「已同步 n 条到站点私有导航」；Key 未授权 403 提示不变。

### 3.2 私有数据不落盘 + 双 Tab 结构

- 私有导航条目属受保护数据：**只在内存渲染，不写入书签树 / chrome.storage / IndexedDB**（与公开镜像「站点导航」文件夹的本质区别）；每次切到私有 Tab 实时拉取（站点侧已有 30s 缓存）；
- 目录结构（bookmarks/ 已超 8 文件上限，顺势分域，均为本批次未提交文件）：

```
bookmarks/
├── BookmarksTab.tsx      # 容器：🌐 公有导航 / 🔒 私有导航 双 Tab 切换
├── BookmarksMain.tsx     # 公有视图 = 原书签树页（自原 BookmarksTab 实现体搬迁）
├── use-bookmark-tools.ts # 管理工具 hook（检测/查重/清理/导入/重置/同步站点导航）——BookmarksMain 瘦身
├── BookmarkList / BookmarkMenus / BookmarkDialogs / EditBar / tools
└── site-nav/             # 站点导航域（8 文件）
    ├── SyncToSiteSheet.tsx / sync-to-site.ts / FolderPicker.tsx
    ├── AiAddSheet.tsx / ai-recognize.ts / nav-import.ts
    ├── private-nav.ts    # 私有导航纯函数：links → 按分类展示树
    └── PrivateNavView.tsx # 私有视图：配置读取 → 门禁状态机 → 列表
```

### 3.3 私有视图门禁状态机（PrivateNavView）

```
挂载：读 settings + 解锁标记 + getPrivateNavConfig
  ├─ 未连接站点            → disconnected：引导去「设置」连接
  ├─ 503 / 404            → 站点未启用精品导航插件或后端过旧（沿 ApiError 文案）
  ├─ mode=self 或未设密码  → need-setup：提示「请到站点后台『精品导航 → 私有设置』
  │                          开启密码访问并设置访问密码；设置完成后回到这里刷新即可」
  │                          ＋ [打开站点] [重新检测] 两按钮
  └─ mode=password 且已设密码
        ├─ 本地已解锁标记  → ready：拉 listPrivateNavLinks 渲染
        └─ 未解锁          → locked：密码卡（输入 → POST /nav/private/unlock）
              ├─ 200 → 记解锁标记 → ready
              ├─ 401 → 「访问密码不正确」
              └─ 403 self_only → 站点配置已变，重拉 config 回 need-setup
```

- 解锁标记存 `chrome.storage.local`（键 `nav_private_unlocked_v1`，手册 §8.2 登记），解锁一次后续访问免输；ready 态右上「🔒 重新锁定」可手动清除；`clearConnection`（断开站点）时一并清除；
- 密码本身**不存储**——校验走站点公开 unlock 端点（真校验，非本地假门）；ready 态数据经 Key（`navlinks.private.list`，站长授权通道）拉取。

### 3.4 API 层新增

| 函数 | 端点 | 说明 |
|---|---|---|
| `getPrivateNavConfig` | GET `/open/nav/private/config` | 走既有 `openGet`（信封） |
| `listPrivateNavLinks` | GET `/open/nav/private/links` | 走既有 `openGet`（信封） |
| `unlockPrivateNav` | POST `/nav/private/unlock` | **直通 JSON 非信封**：client.ts 新导出 `rawPostJson`（保留状态码 + 原始 body，不套 unwrapEnvelope） |

类型：`SiteNavLink` 增加可选 `visibility`；新增 `SiteNavPrivateConfig`、`NavVisibility`。

## 4. 文件清单

| # | 文件 | 动作 |
|---|---|---|
| 1 | `shared/types/index.ts` | SiteNavLink.visibility + SiteNavPrivateConfig + NavVisibility |
| 2 | `shared/api/client.ts` | 导出 rawPostJson（直通 JSON POST） |
| 3 | `shared/api/endpoints.ts` | 私有导航三封装 |
| 4 | `shared/storage/settings.ts` | nav_private_unlocked_v1 键 + 读写 + clearConnection 清除 |
| 5 | `bookmarks/site-nav/*` | 6 个既有文件移入 + private-nav.ts + PrivateNavView.tsx |
| 6 | `bookmarks/BookmarksMain.tsx` | 原 BookmarksTab 实现体（瘦身：工具逻辑抽 hook） |
| 7 | `bookmarks/use-bookmark-tools.ts` | 检测/查重/清理/导入/重置/同步站点导航 hook |
| 8 | `bookmarks/BookmarksTab.tsx` | 改为双 Tab 容器 |
| 9 | `site-nav/sync-to-site.ts` + `SyncToSiteSheet.tsx` | visibility 透传 + 同步目标选择 |
| 10 | 手册 §4/§8.2 + CHANGELOG | 目录图、存储键登记、0.27.0 条目追加 |

## 5. 验证

- [ ] 构建零 TS 错误（scripts/build-browser-extension.sh）
- [ ] 同步到站点：选「私有导航」→ 站点私有列表出现条目、公开列表不受影响；默认仍为公开
- [ ] 双 Tab：公有视图回归（树/搜索/编辑/菜单全可用）；私有 Tab 实时数据、不落本地存储
- [ ] 门禁：未设密码 → 引导文案与按钮；错误密码 401 文案；正确密码解锁进列表；重新锁定后需再输
- [ ] 断开站点连接后解锁标记被清除；未连接提示正确
- [ ] 双浏览器走手册 §12 清单（Chrome / Edge）
