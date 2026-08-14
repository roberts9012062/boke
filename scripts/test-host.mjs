// 模拟插件调用 op-host-web-server 的完整流程
import { spawn } from 'node:child_process'
import { readFileSync } from 'node:fs'

const host = 'C:\\Users\\boss\\workspace\\openpencil\\target\\release\\op-host-web-server'
const doc = 'O:\\package\\obj\\newobj1\\boke\\login.op'

const child = spawn(host, ['--serve-web', '--managed', '--port', '0', '--file', doc, '--allow-origin', 'http://127.0.0.1'])
let stderr = ''
child.stderr.on('data', c => (stderr += c))

// 1. 等待 handshake
const handshake = await new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error('handshake timeout; stderr: ' + stderr)), 20000)
  let buf = ''
  child.stdout.on('data', chunk => {
    buf += chunk.toString('utf8')
    const nl = buf.indexOf('\n')
    if (nl >= 0) {
      clearTimeout(timer)
      resolve(JSON.parse(buf.slice(0, nl).trim()))
    }
  })
  child.on('error', reject)
})
console.log('1. handshake:', JSON.stringify(handshake))
const baseUrl = `http://127.0.0.1:${handshake.port}`
const headers = { authorization: `Bearer ${handshake.token}`, 'x-openpencil-token': handshake.token }

// 2. web ready 检查
const root = await fetch(baseUrl + '/')
const glue = await fetch(baseUrl + '/pkg/op_host_web.js')
console.log('2. web ready: root=' + root.status + ' glue=' + glue.status)

// 3. version before
const v1 = await (await fetch(baseUrl + '/api/mcp/version', { headers })).json()
console.log('3. version before:', v1.version)

// 4. batch_design（openpencil 语法，验证翻译器）
const operations = [
  'root=I(null,{"type":"frame","name":"测试页","width":600,"height":400,"fill":"#EEEEEE","x":0,"y":0})',
  'btn=I(root,{"type":"frame","name":"测试按钮","x":200,"y":160,"width":200,"height":60,"fill":"#4F46E5","cornerRadius":8})',
  'lbl=I(btn,{"type":"text","name":"按钮文字","x":0,"y":15,"width":200,"content":"点击我","fontSize":18,"fontWeight":"600","fill":"#FFFFFF","textAlign":"center","textGrowth":"fixed-width"})',
].join('\n')
const batchRes = await fetch(baseUrl + '/mcp', {
  method: 'POST',
  headers: { ...headers, 'content-type': 'application/json' },
  body: JSON.stringify({ jsonrpc: '2.0', id: 'dsh-test', method: 'tools/call', params: { name: 'batch_design', arguments: { operations } } }),
})
const batchJson = await batchRes.json()
console.log('4. batch_design:', JSON.stringify(batchJson).slice(0, 400))

// 5. version after
const v2 = await (await fetch(baseUrl + '/api/mcp/version', { headers })).json()
console.log('5. version after:', v2.version, 'increased:', v2.version > v1.version)

// 6. document
const docRes = await (await fetch(baseUrl + '/api/mcp/document', { headers })).json()
const docJson = JSON.stringify(docRes.document)
console.log('6. document version:', docRes.version, 'bytes:', docJson.length, 'root:', docRes.document.children?.[0]?.name)

// 7. get_selection
const selRes = await fetch(baseUrl + '/mcp', {
  method: 'POST',
  headers: { ...headers, 'content-type': 'application/json' },
  body: JSON.stringify({ jsonrpc: '2.0', id: 'dsh-test2', method: 'tools/call', params: { name: 'get_selection', arguments: {} } }),
})
const selJson = await selRes.json()
console.log('7. get_selection:', JSON.stringify(selJson).slice(0, 300))

child.kill()
process.exit(0)
