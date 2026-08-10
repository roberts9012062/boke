// src/app/not-found.tsx
// 404 页（设计稿《404》画板，D/M 双端文案一致）：
// 「这条月色走丢了」+ 返回首页 / 去搜索（桌面）。
import { StatusPage } from "@/components/ui/status-page";

// NotFound 路由不存在时的统一 404 页。
export default function NotFound() {
  return (
    <StatusPage
      code="404"
      title="这条月色走丢了"
      description="页面不存在，或已被作者收起。回到首页继续漫步吧。"
      showSearch
    />
  );
}
