// cmd/seo-plugin/frontend/seo-panel.js
// SEO 优化插件 · 发帖 SEO 面板（M4.1 compose.seo 槽位）：
//   渲染 SEO 标题（12/60 字数）/描述（18/160）/URL 别名/收录开关/OG 提示/重置，
//   值经 ctx.props.onChange 回写发帖表单（随发帖请求提交，主进程写入 seo_meta）。
// ctx: { slot, el, api, user, props: { initial, onChange } }
// 说明：props 为挂载时快照（PluginSlot 约定）；onChange 稳定回调（React setState）。
export default function register(ctx) {
  const initial = ctx.props?.initial || {};
  // 面板内部状态（不回读外部——避免 props 变化重挂载导致输入失焦）
  const state = {
    seo_title: initial.seo_title || "",
    seo_description: initial.seo_description || "",
    url_alias: initial.url_alias || "",
    robots: initial.robots || "",
  };

  const wrapper = document.createElement("div");
  wrapper.className = "mt-4 rounded-lg border border-line bg-elevated p-4";

  const emit = () => {
    if (typeof ctx.props?.onChange === "function") {
      ctx.props.onChange({ ...state });
    }
  };

  const count = (text, max) => {
    const n = Array.from(text).length;
    return `${Math.min(n, max)}/${max}`;
  };

  wrapper.innerHTML =
    '<div class="flex items-center justify-between">' +
    "<p class='text-sm font-medium text-ink'>SEO</p>" +
    "<p class='text-[10px] text-ink-3'>SEO 插件已启用 · 可自定义收录</p>" +
    "</div>" +
    '<div class="mt-3 space-y-3">' +
    // SEO 标题
    '<div><label class="mb-1 block text-xs text-ink-3">SEO 标题（可选，默认用正文摘要）</label>' +
    '<input data-seo-title type="text" class="h-9 w-full rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none" placeholder="月光落在窗台…">' +
    '<p class="mt-0.5 text-right text-[10px] text-ink-3" data-seo-title-count></p></div>' +
    // SEO 描述
    '<div><label class="mb-1 block text-xs text-ink-3">SEO 描述</label>' +
    '<textarea data-seo-desc rows="2" class="w-full resize-none rounded-lg border border-line bg-muted px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none" placeholder="月光落在窗台，像一封还没写完的信。"></textarea>' +
    '<p class="mt-0.5 text-right text-[10px] text-ink-3" data-seo-desc-count></p></div>' +
    // URL 别名 + 收录
    '<div class="flex items-end gap-3">' +
    '<div class="flex-1"><label class="mb-1 block text-xs text-ink-3">URL 别名</label>' +
    '<div class="flex items-center gap-1.5"><span class="text-xs text-ink-3">/p/</span>' +
    '<input data-seo-alias type="text" class="h-9 w-full rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none" placeholder="silver-window"></div></div>' +
    '<div class="w-36"><label class="mb-1 block text-xs text-ink-3">收录</label>' +
    '<select data-seo-robots class="h-9 w-full rounded-lg border border-line bg-muted px-2 text-sm text-ink focus:border-accent focus:outline-none">' +
    '<option value="">开启（index, follow）</option>' +
    '<option value="noindex, nofollow">关闭（noindex, nofollow）</option>' +
    "</select></div></div>" +
    '<p class="text-[10px] text-ink-3">OG 图片：使用帖子封面（默认）</p>' +
    // 操作
    '<div class="flex items-center justify-between pt-1">' +
    '<p class="text-[10px] text-ink-3" data-seo-hint>可覆盖全局默认</p>' +
    '<div class="flex gap-2">' +
    '<button type="button" data-seo-reset class="rounded-full border border-line px-3 py-1 text-xs text-ink-2 hover:text-ink">重置</button>' +
    '<button type="button" data-seo-apply class="rounded-full bg-accent px-4 py-1 text-xs text-on-accent hover:opacity-90">应用并返回</button>' +
    "</div></div></div>";

  // 绑定输入（每次输入更新内部状态 + 字数统计 + 回写）
  const titleInput = wrapper.querySelector("[data-seo-title]");
  const descInput = wrapper.querySelector("[data-seo-desc]");
  const aliasInput = wrapper.querySelector("[data-seo-alias]");
  const robotsSelect = wrapper.querySelector("[data-seo-robots]");
  const titleCount = wrapper.querySelector("[data-seo-title-count]");
  const descCount = wrapper.querySelector("[data-seo-desc-count]");
  const hint = wrapper.querySelector("[data-seo-hint]");

  titleInput.value = state.seo_title;
  descInput.value = state.seo_description;
  aliasInput.value = state.url_alias;
  robotsSelect.value = state.robots;
  titleCount.textContent = count(state.seo_title, 60);
  descCount.textContent = count(state.seo_description, 160);

  titleInput.addEventListener("input", () => {
    state.seo_title = titleInput.value;
    titleCount.textContent = count(state.seo_title, 60);
  });
  descInput.addEventListener("input", () => {
    state.seo_description = descInput.value;
    descCount.textContent = count(state.seo_description, 160);
  });
  aliasInput.addEventListener("input", () => {
    state.url_alias = aliasInput.value;
  });
  robotsSelect.addEventListener("change", () => {
    state.robots = robotsSelect.value;
  });

  // 重置：清空面板与发帖表单（onChange 回写空）
  wrapper.querySelector("[data-seo-reset]").addEventListener("click", () => {
    state.seo_title = "";
    state.seo_description = "";
    state.url_alias = "";
    state.robots = "";
    titleInput.value = "";
    descInput.value = "";
    aliasInput.value = "";
    robotsSelect.value = "";
    titleCount.textContent = "0/60";
    descCount.textContent = "0/160";
    emit();
    hint.textContent = "已重置为默认";
  });

  // 应用并返回：回写并收起面板（下次展开回填当前值）
  wrapper.querySelector("[data-seo-apply]").addEventListener("click", () => {
    emit();
    wrapper.querySelector(".space-y-3").style.display = "none";
    hint.textContent = "已应用（可重新展开编辑）";
    wrapper.querySelector("[data-seo-title]").dispatchEvent(new Event("focus"));
  });

  ctx.el.appendChild(wrapper);

  return () => {
    wrapper.remove();
  };
}
