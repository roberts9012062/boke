// db/embed.go
// 数据库 SQL 资源内嵌包：将 schema.sql / seed.sql / migrations 嵌入二进制。
//
// 用途：安装向导（internal/setup）在 Docker 等无源码目录的环境下执行
//       建库建表与迁移时，无需挂载 db/ 目录，全部 SQL 从二进制内读取。
// 说明：go:embed 只能嵌入本包目录内文件，故 embed 指令放在 db/ 下。
package db

import "embed"

// SchemaSQL 全量建表脚本（幂等，CREATE TABLE IF NOT EXISTS）。
//
//go:embed schema.sql
var SchemaSQL string

// SeedSQL 基础种子数据（幂等，ON CONFLICT DO NOTHING）。
//
//go:embed seed.sql
var SeedSQL string

// MigrationsFS 增量迁移目录（按文件名字典序执行，幂等）。
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
