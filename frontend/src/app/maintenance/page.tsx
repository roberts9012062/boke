// src/app/maintenance/page.tsx
// 维护中页（设计稿《维护中》画板）：
// 「月言正在休整」+ 返回首页 / 去搜索。
// 说明（M1.7）：组件与页面就位；全站维护开关（后台站点设置 → 中间件拦截）规划 M2。
import { StatusPage } from "@/components/ui/status-page";

// MaintenancePage 维护中状态页。
export default function MaintenancePage() {
  return (
    <StatusPage
      code="维护"
      title="月言正在休整"
      description="预计 02:00–04:00 完成升级。感谢耐心，回来会更好。"
      showSearch
    />
  );
}
