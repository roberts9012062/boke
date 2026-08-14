// 直接调用 Pen execute 测试翻译后的程序 + Get 输出完整性
import { readFileSync, writeFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'

const server = 'O:\\package\\Pen\\resources\\app.asar.unpacked\\out\\mcp-server-windows-x64.exe'
const program = [
  'root=Insert("document",{"type":"frame","name":"测试页2","width":600,"height":400,"fill":"#EEEEEE","x":0,"y":0})',
  'Print(JSON.stringify(Get((n, ctx) => ctx.depth === 0 ? n : undefined)))',
].join('\n')
const reqs = [
  { jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'probe', version: '1.0' } } },
  { jsonrpc: '2.0', method: 'notifications/initialized' },
  { jsonrpc: '2.0', id: 3, method: 'tools/call', params: { name: 'execute', arguments: { filePath: 'O:/package/obj/newobj1/boke/scripts/login.pen', input: program } } },
]
writeFileSync('mcp-in.json', reqs.map(r => JSON.stringify(r)).join('\n') + '\n', 'utf8')
const res = spawnSync('cmd', ['/c', `${server} -app desktop < mcp-in.json > mcp-out.txt 2> mcp-err.txt`], { encoding: 'utf8' })
const out = readFileSync('mcp-out.txt', 'utf8')
for (const line of out.split('\n').filter(Boolean)) {
  try {
    const msg = JSON.parse(line)
    if (msg.id === 3) {
      const text = msg.result?.content?.[0]?.text ?? JSON.stringify(msg.error)
      console.log('RESPONSE TEXT (full):')
      console.log(text)
      console.log('--- length:', text.length)
      // 提取 Print output 行
      const pi = text.indexOf('## Print output')
      if (pi >= 0) {
        const json = text.slice(pi + '## Print output'.length).trim()
        console.log('--- print line:', json.slice(0, 120))
        const arr = JSON.parse(json)
        console.log('--- top-level nodes:', arr.map(n => n.name).join(', '))
      }
    }
  } catch (e) { /* ignore */ }
}
