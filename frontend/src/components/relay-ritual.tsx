// src/components/relay-ritual.tsx
// 对接仪式（全屏遮罩）：5 秒倒计时 → 卫星飞向星球对接 → 对接成功。
// 动画结束回调 onFinish（父组件在此刻正式启用对接配置）。
"use client";

import { useEffect, useState } from "react";

// RitualPhase 仪式阶段：倒计时 → 对接中 → 成功。
type RitualPhase = "countdown" | "docking" | "success";

// countdownSeconds 倒计时秒数。
const countdownSeconds = 5;

// dockingMs 对接动画时长（毫秒）。
const dockingMs = 3200;

// RelayRitual 对接仪式遮罩：深空背景 + 阶段动画。
export function RelayRitual({ relayName, onFinish }: { relayName: string; onFinish: () => void }) {
  const [phase, setPhase] = useState<RitualPhase>("countdown");
  const [count, setCount] = useState(countdownSeconds);

  // 倒计时 → 对接 → 成功（阶段自动推进；success 后延迟收尾交给父组件）
  useEffect(() => {
    if (phase !== "countdown") {
      return;
    }
    if (count <= 0) {
      setPhase("docking");
      return;
    }
    const timer = setTimeout(() => setCount((c) => c - 1), 1000);
    return () => clearTimeout(timer);
  }, [phase, count]);

  useEffect(() => {
    if (phase !== "docking") {
      return;
    }
    const timer = setTimeout(() => setPhase("success"), dockingMs);
    return () => clearTimeout(timer);
  }, [phase]);

  return (
    <div className="fixed inset-0 z-50 flex flex-col items-center justify-center bg-[#030b1c]/97 backdrop-blur-sm">
      {/* 星点背景 */}
      <div
        className="pointer-events-none absolute inset-0 opacity-60"
        style={{
          backgroundImage:
            "radial-gradient(1px 1px at 12% 22%,#fff 50%,transparent),radial-gradient(1px 1px at 68% 12%,#9fd8ff 50%,transparent),radial-gradient(1.5px 1.5px at 34% 74%,#fff 50%,transparent),radial-gradient(1px 1px at 82% 58%,#cfe9ff 50%,transparent),radial-gradient(1px 1px at 52% 38%,#fff 50%,transparent),radial-gradient(1px 1px at 8% 82%,#9fd8ff 50%,transparent),radial-gradient(1px 1px at 92% 86%,#fff 50%,transparent)",
          backgroundSize: "420px 420px",
        }}
      />

      {/* 阶段一：5 秒倒计时（大数字 + 进度环） */}
      {phase === "countdown" && (
        <div className="relative flex h-56 w-56 items-center justify-center">
          <svg viewBox="0 0 120 120" className="absolute inset-0 h-full w-full -rotate-90">
            <circle cx="60" cy="60" r="52" fill="none" stroke="#173952" strokeWidth="3" />
            <circle
              cx="60" cy="60" r="52" fill="none" stroke="#8af3ff" strokeWidth="3" strokeLinecap="round"
              strokeDasharray={2 * Math.PI * 52}
              strokeDashoffset={2 * Math.PI * 52 * (1 - (countdownSeconds - count + 1) / countdownSeconds)}
              style={{ transition: "stroke-dashoffset 1s linear", filter: "drop-shadow(0 0 6px #8af3ff)" }}
            />
          </svg>
          <div className="text-center">
            <div key={count} className="font-display text-7xl font-bold text-[#eaffff]" style={{ animation: "tick 1s ease-out" }}>
              {count > 0 ? count : "🚀"}
            </div>
            <div className="mt-2 text-xs tracking-[0.3em] text-[#7ea6c8]">点火倒计时</div>
          </div>
          <style>{`@keyframes tick{0%{transform:scale(1.45);opacity:.2}100%{transform:scale(1);opacity:1}}`}</style>
        </div>
      )}

      {/* 阶段二：卫星飞向星球对接（线框 SVG + 位移动画 + 对接火花） */}
      {phase === "docking" && (
        <div className="relative h-64 w-full max-w-xl">
          <svg viewBox="0 0 480 240" className="h-full w-full">
            {/* 目标星球（右侧，带环） */}
            <g className="orbit" style={{ transformOrigin: "400px 120px" }}>
              <circle cx="400" cy="120" r="34" fill="none" stroke="#8af3ff" strokeWidth="1.5" opacity="0.9" />
              <ellipse cx="400" cy="120" rx="52" ry="12" fill="none" stroke="#2e5f78" strokeWidth="1" transform="rotate(-14 400 120)" />
            </g>
            {/* 卫星（左侧，线框） */}
            <g className="sat-fly" style={{ transformOrigin: "80px 120px" }}>
              <rect x="58" y="104" width="44" height="30" rx="5" fill="none" stroke="#8af3ff" strokeWidth="1.5" />
              <line x1="58" y1="114" x2="102" y2="114" stroke="#2e5f78" strokeWidth="1" />
              <line x1="58" y1="124" x2="102" y2="124" stroke="#2e5f78" strokeWidth="1" />
              <rect x="16" y="108" width="34" height="22" fill="none" stroke="#8af3ff" strokeWidth="1.2" />
              <line x1="50" y1="108" x2="50" y2="130" stroke="#2e5f78" strokeWidth="1" />
              <rect x="110" y="108" width="34" height="22" fill="none" stroke="#8af3ff" strokeWidth="1.2" />
              <line x1="110" y1="108" x2="110" y2="130" stroke="#2e5f78" strokeWidth="1" />
              <path d="M 68 104 A 16 11 0 0 1 92 104 L 80 94 Z" fill="none" stroke="#8af3ff" strokeWidth="1.2" />
            </g>
            {/* 对接火花（卫星抵达时亮起的连线脉冲） */}
            <line className="spark" x1="104" y1="119" x2="366" y2="119" stroke="#eaffff" strokeWidth="1.5" strokeDasharray="3 8" opacity="0" />
            <circle className="spark-dot" cx="240" cy="119" r="4" fill="#eaffff" opacity="0" />
          </svg>
          <p className="mt-3 text-center text-sm tracking-[0.3em] text-[#7ea6c8]">卫星机动 · 锁定星球</p>
          <style>{`
            .sat-fly{animation:fly ${dockingMs}ms cubic-bezier(.5,.05,.35,1) forwards}
            @keyframes fly{0%{transform:translateX(0) translateY(0)}18%{transform:translateX(20px) translateY(-16px)}100%{transform:translateX(288px) translateY(0)}}
            .orbit{animation:lock 1.6s ease-in-out 2 alternate}
            @keyframes lock{from{transform:scale(1)}to{transform:scale(1.08)}}
            .spark{animation:sparkle .5s ease-out ${dockingMs - 700}ms 3 forwards}
            .spark-dot{animation:sparkleDot .5s ease-out ${dockingMs - 700}ms 3 forwards}
            @keyframes sparkle{0%{opacity:0}50%{opacity:1}100%{opacity:.2}}
            @keyframes sparkleDot{0%{opacity:0;transform:translateX(-90px)}100%{opacity:1;transform:translateX(90px)}}
          `}</style>
        </div>
      )}

      {/* 阶段三：对接成功（打勾弹入 + 光晕） */}
      {phase === "success" && (
        <div className="text-center" style={{ animation: "pop .5s cubic-bezier(.2,1.6,.4,1)" }}>
          <div className="relative mx-auto h-24 w-24">
            <div className="absolute inset-0 rounded-full" style={{ boxShadow: "0 0 40px 12px rgba(138,243,255,.45)" }} />
            <svg viewBox="0 0 96 96" className="h-full w-full">
              <circle cx="48" cy="48" r="42" fill="none" stroke="#8af3ff" strokeWidth="2.5" />
              <path className="check" d="M 30 50 L 43 63 L 67 36" fill="none" stroke="#eaffff" strokeWidth="5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h2 className="mt-6 font-display text-2xl font-bold text-[#eaffff]">对接成功</h2>
          <p className="mt-2 text-sm text-[#8fb6d8]">
            本站已接入 <span className="text-[#8af3ff]">{relayName || "中继站"}</span> · 大世界频道已开通
          </p>
          <button
            type="button"
            onClick={onFinish}
            className="mt-8 rounded-full bg-[#2563eb] px-8 py-2.5 text-sm font-medium text-white transition-opacity hover:opacity-90"
          >
            进入大世界
          </button>
        </div>
      )}
      <style>{`@keyframes pop{0%{transform:scale(.6);opacity:0}100%{transform:scale(1);opacity:1}}
      .check{stroke-dasharray:60;stroke-dashoffset:60;animation:draw .6s ease-out .2s forwards}
      @keyframes draw{to{stroke-dashoffset:0}}`}</style>
    </div>
  );
}
