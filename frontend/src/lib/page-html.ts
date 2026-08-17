// src/lib/page-html.ts
// AI 页面构建器 HTML 工具（纯函数）：从 AI 输出提取完整 HTML 文档 + 注入高度上报脚本。
// 预览与前台渲染共用（iframe srcDoc 沙箱模式）。

// extractHtmlDocument 从 AI 回复文本中提取 HTML 文档。
// 提取顺序：① ```html 代码块；② 以 <!DOCTYPE 或 <html 开头的片段；③ 兜底原文。
export function extractHtmlDocument(text: string): string {
  const fenced = text.match(/```html\s*([\s\S]*?)```/i);
  if (fenced && fenced[1]) {
    return fenced[1].trim();
  }
  const start = text.search(/<!DOCTYPE html|<html/i);
  if (start >= 0) {
    return text.slice(start).trim();
  }
  return text.trim();
}

// HEIGHT_REPORT_SCRIPT 高度上报脚本（注入 iframe srcDoc；postMessage 不受沙箱隔离源限制）。
// 说明：上报 multiple 时机（load/DOMContentLoaded/延时兜底/resize），覆盖图片等异步资源撑高场景。
const HEIGHT_REPORT_SCRIPT = `<script>(function(){
  function report(){
    try{
      parent.postMessage({type:"yy-page-height",height:Math.max(document.documentElement.scrollHeight,document.body.scrollHeight)},"*");
    }catch(e){}
  }
  window.addEventListener("load",report);
  window.addEventListener("resize",report);
  if(document.readyState==="complete"){report();}else{document.addEventListener("DOMContentLoaded",report);}
  setTimeout(report,500);setTimeout(report,1500);
})();</script>`;

// injectHeightReport 在 HTML 文档中注入高度上报脚本（</body> 前；无 body 标签则追加末尾）。
// 纯函数：不修改入参，返回新字符串。
export function injectHeightReport(html: string): string {
  if (/<\/body>/i.test(html)) {
    return html.replace(/<\/body>/i, `${HEIGHT_REPORT_SCRIPT}</body>`);
  }
  return html + HEIGHT_REPORT_SCRIPT;
}
