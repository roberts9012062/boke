// 生成 MCP 请求序列并调用 Pencil MCP server，输出响应
// 用法: node call-mcp.mjs <工具名> <参数JSON文件>
import { readFileSync, writeFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'

const tool = process.argv[2]
const argsFile = process.argv[3]
const args = argsFile ? JSON.parse(readFileSync(argsFile, 'utf8')) : {}

const server = 'O:\\package\\Pen\\resources\\app.asar.unpacked\\out\\mcp-server-windows-x64.exe'
const reqs = [
  { jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'dsh-agent', version: '1.0' } } },
  { jsonrpc: '2.0', method: 'notifications/initialized' },
  { jsonrpc: '2.0', id: 3, method: 'tools/call', params: { name: tool, arguments: args } },
]
const input = reqs.map(r => JSON.stringify(r)).join('\n') + '\n'
writeFileSync('mcp-in.json', input, 'utf8')

const res = spawnSync('cmd', ['/c', `${server} -app desktop < mcp-in.json > mcp-out.txt 2> mcp-err.txt`], { encoding: 'utf8' })
const out = readFileSync('mcp-out.txt', 'utf8')
const lines = out.split('\n').filter(Boolean)
for (const line of lines) {
  try {
    const msg = JSON.parse(line)
    if (msg.id === 3) {
      if (msg.error) {
        console.log('ERROR:', JSON.stringify(msg.error))
      } else {
        for (const c of msg.result?.content ?? []) {
          if (c.type === 'text') console.log(c.text)
          else if (c.type === 'image') {
            const name = `mcp-image-${Date.now()}.png`
            writeFileSync(name, Buffer.from(c.data, 'base64'))
            console.log(`[IMAGE saved: ${name}]`)
          }
        }
      }
    }
  } catch { /* 忽略非 JSON 行 */ }
}
