#!/usr/bin/env bash
# scripts/pen-inspect.sh
# 查看 boke.pen 设计文件：列出画板清单，或输出指定画板的全部文本。
#
# 用法：
#   ./scripts/pen-inspect.sh              # 列出全部画板
#   ./scripts/pen-inspect.sh 首页         # 查看包含「首页」的画板文本
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

go run ./cmd/peninspect "$@"
