// 模拟宿主 spawn jian 的行为
import { spawn } from 'node:child_process'

const jian = 'C:\\Users\\boss\\workspace\\jian\\target\\release\\jian'
const child = spawn(jian, ['render', 'O:\\package\\obj\\newobj1\\boke\\login.op', '--out', 'O:\\package\\obj\\newobj1\\boke\\scripts\\jian-out.png', '--scale', '2'])
let stdout = ''
let stderr = ''
child.stdout.on('data', c => (stdout += c))
child.stderr.on('data', c => (stderr += c))
child.on('error', err => console.log('SPAWN ERROR:', err.message))
child.on('close', code => {
  console.log('CLOSE code:', code)
  console.log('STDOUT:', stdout.trim())
  console.log('STDERR:', stderr.trim())
})
