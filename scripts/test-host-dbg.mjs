// 调试版：spawn ophost 并捕获 stderr，执行完整流程
import { spawn } from 'node:child_process'

const host = 'C:\\Users\\boss\\workspace\\openpencil\\target\\release\\op-host-web-server'
const doc = 'O:\\package\\obj\\newobj1\\boke\\scripts\\host-test.op'

const child = spawn(host, ['--serve-web', '--managed', '--port', '0', '--file', doc, '--allow-origin', 'http://127.0.0.1'])
let stderr = ''
child.stderr.on('data', c => {
  stderr += c.toString()
  const lines = stderr.split('\n')
  stderr = lines.pop()
  for (const l of lines) if (l.trim()) console.log('[host]', l.trim())
})
child.stdout.on('data', () => {})

const handshake = await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error('no handshake')), 20000)
  let buf = ''
  child.stdout.on('data', chunk => {
    buf += chunk.toString('utf8')
    const nl = buf.indexOf('\n')
    if (nl >= 0) { clearTimeout(timer); resolve(JSON.parse(buf.slice(0, nl).trim())) }
  })
})
console.log('handshake port:', handshake.port)
const baseUrl = `http://127.0.0.1:${handshake.port}`
const headers = { authorization: `Bearer ${handshake.token}`, 'x-openpencil-token': handshake.token }

// version before
const v1 = await (await fetch(baseUrl + '/api/mcp/version', { headers })).json()
console.log('version before:', v1.version)

// batch_design
const operations = 'root=I(null,{"type":"frame","name":"测试页","width":600,"height":400,"fill":"#EEEEEE"})'
const t0 = Date.now()
const batchRes = await fetch(baseUrl + '/mcp', {
  method: 'POST',
  headers: { ...headers, 'content-type': 'application/json' },
  body: JSON.stringify({ jsonrpc: '2.0', id: 'dsh-dbg', method: 'tools/call', params: { name: 'batch_design', arguments: { operations } } }),
})
const batchJson = await batchRes.json()
console.log(`batch_design (${(Date.now() - t0) / 1000}s):`, JSON.stringify(batchJson).slice(0, 200))

const v2 = await (await fetch(baseUrl + '/api/mcp/version', { headers })).json()
console.log('version after:', v2.version)

child.kill()
process.exit(0)
