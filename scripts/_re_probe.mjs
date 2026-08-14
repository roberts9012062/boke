// 临时：复现 Go 正则对 ptqrlogin 成功响应的匹配
const re = /ptuiCB\('([^']*)','([^']*)','([^']*)','([^']*)','([^']*)'(?:,\s*'([^']*)')?\)/;
const samples = [
  "ptuiCB('67','0','','0','二维码认证中。', '')",
  "ptuiCB('0','0','https://ssl.ptlogin2.graph.qq.com/check_sig?pttype=1&uin=250467554&service=ptqrlogin&nodirect=0&ptsigx=76db590e2857e9df71d8dc344fb9f1f4d40a3c06cb06d29693ff806d09f1691340601c34e1374e03e94d312dffe9320eafc5bd38e465dec2feb1976c2991d0c5fb035d575580985cd001eb240d9325097cc63ec48259b688845d58e11184df4f&s_url=https%3A%2F%2Fgraph.qq.com%2Foauth2.0%2Flogin_jump&f_url=&ptlang=2052&ptredirect=100&aid=716027609&daid=383&j_later=0&low_login_hour=0&regmaster=0&pt_login_type=3&pt_aid=0&pt_aaid=16','0','登录成功！', '')",
  "ptuiCB('0','0','https://ssl.ptlogin2.graph.qq.com/check_sig?pttype=1&uin=250467554&service=ptqrlogin&nodirect=0&ptsigx=abc&s_url=https%3A%2F%2Fgraph.qq.com%2Foauth2.0%2Flogin_jump&f_url=&ptlang=2052&ptredirect=100&aid=716027609&daid=383&j_later=0&low_login_hour=0&regmaster=0&pt_login_type=3&pt_aid=0&pt_aaid=16','0','登录成功！')",
];
for (const s of samples) {
  const m = s.match(re);
  console.log("match:", m ? "YES" : "NO", "| groups:", m ? JSON.stringify(m.slice(1)) : "-");
}
