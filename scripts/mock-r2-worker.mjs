// scripts/mock-r2-worker.mjs
// 本地 mock Worker：模拟 Cloudflare Worker + R2 行为（图床插件端到端联调用，
// 契约与 marketplace-repo/image-cdn/worker/index.js 完全一致）。
//
// 用法：node scripts/mock-r2-worker.mjs [port]（默认 18971）
// 对象落盘 data/plugins/image-cdn/mock-r2/（yyyymm/<16hex>.<ext>）
import { createServer } from "node:http";
import { mkdirSync, writeFileSync, readFileSync, unlinkSync, existsSync, statSync } from "node:fs";
import { randomUUID } from "node:crypto";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const R2_DIR = join(ROOT, "data", "plugins", "image-cdn", "mock-r2");
const API_KEY = "mock-key-123";
const PORT = Number(process.argv[2] || 18971);
const TYPES = { jpg: "image/jpeg", jpeg: "image/jpeg", png: "image/png", gif: "image/gif", webp: "image/webp" };

mkdirSync(R2_DIR, { recursive: true });

const server = createServer(async (req, res) => {
  const json = (status, body) => {
    res.writeHead(status, { "Content-Type": "application/json; charset=utf-8" });
    res.end(JSON.stringify(body));
  };
  const authed = (req.headers.authorization || "").replace(/^Bearer\s+/i, "") === API_KEY;
  try {
    if (req.url === "/health") {
      if (!authed) return json(401, { ok: false, error: "invalid key" });
      return json(200, { ok: true, service: "yueyan-image-bed-mock" });
    }
    if (req.url === "/upload" && req.method === "POST") {
      if (!authed) return json(401, { error: "invalid key" });
      const chunks = [];
      for await (const c of req) chunks.push(c);
      const m = Buffer.concat(chunks).toString("utf8").match(/filename="([^"]+)"[\s\S]*?\r\n\r\n([\s\S]*?)\r\n--/);
      if (!m) return json(400, { error: "multipart 解析失败" });
      const ext = (m[1].split(".").pop() || "").toLowerCase();
      if (!TYPES[ext]) return json(400, { error: "unsupported image type" });
      const content = Buffer.from(m[2], "binary"); // 正则捕获已排除结尾 \r\n 边界
      const ym = new Date().toISOString().slice(0, 7).replace("-", "");
      const key = `${ym}/${randomUUID().replace(/-/g, "").slice(16)}.${ext}`;
      mkdirSync(join(R2_DIR, ym), { recursive: true });
      writeFileSync(join(R2_DIR, key), content);
      console.log(`[mock-r2] 上传 ${key}（${content.length} 字节）`);
      return json(200, { url: `http://localhost:${PORT}/f/${key}`, key, size: content.length, mime: TYPES[ext] });
    }
    const km = req.url.match(/^\/f\/([^?#]+)/); // 对象键含斜杠（yyyymm/name.ext），匹配到 ?/# 前
    if (km && req.method === "GET") {
      const file = join(R2_DIR, decodeURIComponent(km[1]));
      const isFile = file.startsWith(R2_DIR) && existsSync(file) && statSync(file).isFile();
      if (!isFile) return json(404, { error: "not found" });
      const ext = (km[1].split(".").pop() || "").toLowerCase();
      res.writeHead(200, { "Content-Type": TYPES[ext] || "application/octet-stream" });
      return res.end(readFileSync(file));
    }
    if (km && req.method === "DELETE") {
      if (!authed) return json(401, { error: "invalid key" });
      unlinkSync(join(R2_DIR, decodeURIComponent(km[1])));
      return json(200, { deleted: true });
    }
    json(404, { error: "not found" });
  } catch (e) {
    json(500, { error: String(e) });
  }
});

server.listen(PORT, () => console.log(`[mock-r2] 就绪 http://localhost:${PORT}（API_KEY=${API_KEY}，R2 目录 ${R2_DIR}）`));
