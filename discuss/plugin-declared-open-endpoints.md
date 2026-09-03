# 方案：插件声明式开放端点（接口开放目录统一纳管，主程序免发版）

> 讨论稿 · 宿主 v1.4.1 + nav-links v1.3.16 示范落地。

## 1. 需求（用户口径）

插件暴露的开放接口**统一出现在后台「接口开放」目录里统一管理**（Key 逐条目勾选授权），
且主程序**不再为了插件暴露接口而频繁升级**——插件发版即可上新开放接口。

## 2. 现状与根因

开放网关目录 `model.OpenAPICatalog()` 与网关 handler 均硬编码在宿主源码：
插件每新增一条对外接口（如 v1.4.0 的 `navlinks.private.*` 三端点）都要宿主改码发版——
插件迭代被主源码发版节奏绑死（僵化坏味道）。

## 3. 设计

### 3.1 声明链（单一事实源 = 插件清单）

```
插件 plugin.json 声明 open_endpoints
  → cmd/bp pack 写入包内 manifest.json（bpkg.Manifest 新字段）
  → 安装/升级解包落盘 data/plugins/{id}/manifest.json
  → 宿主聚合器扫描目录文件 → 合并进「接口开放」目录（source=plugin 标记）
```

### 3.2 目录条目（插件贡献）

| 字段 | 约束 |
|---|---|
| `endpoint` | 必须以 `{pluginID}.` 前缀命名（防伪装宿主原生接口） |
| `method` | GET / POST |
| `path` | 必须以 `/api/v1/open/plugins/{pluginID}/` 开头；插件端路径由其推导（去前缀） |
| `name` / `description` / `params` | 后台目录展示与参数说明 |

### 3.3 泛化网关路由（一条路由服务全部插件）

- `* /api/v1/open/plugins/:id/*path`（开放组，组级 ApiKeyAuth 生效）
- ApiKeyAuth 升级：静态索引（Method+FullPath）未命中 → 查**插件条目索引**（Method+实际请求路径）
- 转发：以 System 身份调插件 `{plugin_method} {推导的插件端路径}`，body 透传；
  插件 `200+{error}` 转网关 400（对齐 callPluginJSON 惯例）；200 数据包 `resp.OK`
- 声明字段（实现期补充的两项解耦设计）：
  - `plugin_method`：对外 GET 可映射插件 POST（外部语义与插件实现解耦——插件端路由多为桥接时代的 POST）
  - `trusted_body`：网关转发时注入并**覆盖**外部同名键（如 `{"admin":true}`）——
    凭 Key 调用即站长授权，同时防外部 body 伪造身份字段
- 护栏：插件端点白名单精确匹配（未声明的子路径 404）、方法不匹配 404、
  命名空间校验（打包期拒绝 + 聚合期防御跳过）、与宿主静态目录标识冲突即跳过（防静默扩权）

### 3.4 聚合器（internal/service 新文件）

- 扫描 `data/plugins/*/manifest.json` 的 `open_endpoints`，校验命名空间后产出 `[]model.CatalogEntry`
- TTL 缓存 15s（安装后短暂延迟可接受，避免侵入 PluginService 挂失效钩子）
- 后台目录接口（GET /admin/open-api/catalog）= 静态目录 + 插件条目合并（`source: "host"|"plugin"`）

### 3.5 兼容与迁移

- v1.4.0 已发的 `/api/v1/open/nav/private/*` 硬编码端点**保留**（浏览器插件已对接，不破坏）；
  nav-links v1.3.16 起同三能力在泛化路径 `/api/v1/open/plugins/nav-links/private/*` 再声明一份，新对接走泛化路径
- 旧硬编码桥接后续大版本再评估清理

## 4. 验证

- [x] 单测：声明校验（命名空间/方法/路径/plugin_method）、聚合产出（违规跳过/静态冲突跳过）、
  索引匹配、路径→插件端路径推导（internal/service/plugin_openapi_test.go + bpkg 测试）
- [x] E2E（本机 v1.3.16 + 泛化网关）：目录三条件目聚合 → Key 勾选 → 泛化路径调用成功
  （读私有列表 GET→插件 POST + trusted_body admin 注入 / 读改设置 / 业务错误 400）；
  未勾选 403、无 Key 401、未声明子路径 404、方法不匹配 404、外部伪造 admin 字段被覆盖
- [x] 既有静态端点（navlinks.list 等）行为不回归；旧硬编码三端点与泛化路径共存
- [x] 后台目录页前端补充 source 徽标（「插件 · 名称」，悬停说明随插件自动增删）

## 5. 排障记录（E2E 期间）

1. **GET 泛化 → 插件 404**：插件端 `/private/links` 只注册了 POST（桥接时代约定）→ 引入 `plugin_method` 声明映射。
2. **GET 泛化 → 插件 401 need_password**：泛化网关透传空 body，插件按门禁矩阵拒匿名 → 引入 `trusted_body` 声明注入（Key=站长授权语义）。
3. 本机多次「幽灵 404」实为 `go run` 子进程孤儿化抢占 8080（stop-all 只杀父进程）——与代码无关，定点 `taskkill server.exe` 后复现正常。
