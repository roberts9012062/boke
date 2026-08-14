// 在 Pen asar 的 editor bundle 中搜索字符串
import { readFileSync } from 'node:fs'

const buf = readFileSync('O:/package/Pen/resources/app.asar')
const start = buf.indexOf(Buffer.from('{"files"'))
function extractJson(bytes) {
  let depth = 0, inString = false, escaped = false
  for (let k = 0; k < bytes.length; k++) {
    const c = bytes[k]
    if (inString) {
      if (escaped) escaped = false
      else if (c === 0x5c) escaped = true
      else if (c === 0x22) inString = false
      continue
    }
    if (c === 0x22) { inString = true; continue }
    if (c === 0x7b) { depth++; continue }
    if (c === 0x7d) { depth--; if (depth === 0) return bytes.subarray(0, k + 1) }
  }
  throw new Error('unbalanced')
}
const json = extractJson(buf.subarray(start))
const h = JSON.parse(json.toString())
const dataStart = Math.ceil((start + json.length) / 4) * 4
const entry = h.files.out.files.editor.files.assets.files['index.js']
const content = buf.subarray(dataStart + Number(entry.offset), dataStart + Number(entry.offset) + Number(entry.size))
const text = content.toString('utf8')

const needle = process.argv[2]
let idx = 0
let count = 0
while (count < 6) {
  const i = text.indexOf(needle, idx)
  if (i < 0) break
  console.log(`\n=== match at ${i} ===`)
  console.log(text.slice(Math.max(0, i - 500), i + 900))
  idx = i + needle.length
  count++
}
