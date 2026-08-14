// 提取 desktop-resource-device.js 中 backupFilePath 的实现
import { readFileSync } from 'node:fs'

const buf = readFileSync('O:/package/Pen/resources/app.asar')
const start = buf.indexOf(Buffer.from('{"files"'))
function extractJson(bytes) {
  let d = 0, s = false, e = false
  for (let k = 0; k < bytes.length; k++) {
    const c = bytes[k]
    if (s) {
      if (e) e = false
      else if (c === 0x5c) e = true
      else if (c === 0x22) s = false
      continue
    }
    if (c === 0x22) { s = true; continue }
    if (c === 0x7b) { d++; continue }
    if (c === 0x7d) { d--; if (d === 0) return bytes.subarray(0, k + 1) }
  }
  throw new Error('unbalanced')
}
const json = extractJson(buf.subarray(start))
const h = JSON.parse(json.toString())
const dataStart = Math.ceil((start + json.length) / 4) * 4
const entry = h.files.out.files['desktop-resource-device.js']
const content = buf.subarray(dataStart + Number(entry.offset), dataStart + Number(entry.offset) + Number(entry.size)).toString('utf8')
const i = content.indexOf('function backupFilePath')
console.log(content.slice(i, i + 800))
