// src/components/relay-ritual.tsx
// 通讯连接仪式（全屏遮罩）：信号同步倒计时 → 通讯光束锁定星球 → 通讯已建立。
// 动画结束回调 onFinish（父组件在此刻正式启用对接配置）。
"use client";

import { useEffect, useState } from "react";

// RitualPhase 仪式阶段：信号同步 → 链路锁定 → 通讯建立。
type RitualPhase = "countdown" | "linking" | "success";

// countdownSeconds 信号同步秒数。
const countdownSeconds = 5;

// linkingMs 光束锁定动画时长（毫秒）。
const linkingMs = 3200;

// RelayRitual 通讯连接仪式遮罩：深空背景 + 阶段动画。
export function RelayRitual({ relayName, onFinish }: { relayName: string; onFinish: () => void }) {
  const [phase, setPhase] = useState<RitualPhase>("countdown");
  const [count, setCount] = useState(countdownSeconds);

  // 信号同步 → 链路锁定 → 通讯建立（阶段自动推进；success 后由用户收尾）
  useEffect(() => {
    if (phase !== "countdown") {
      return;
    }
    if (count <= 0) {
      setPhase("linking");
      return;
    }
    const timer = setTimeout(() => setCount((c) => c - 1), 1000);
    return () => clearTimeout(timer);
  }, [phase, count]);

  useEffect(() => {
    if (phase !== "linking") {
      return;
    }
    const timer = setTimeout(() => setPhase("success"), linkingMs);
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

      {/* 阶段一：信号同步（数字 + 同心波纹环） */}
      {phase === "countdown" && (
        <div className="relative flex h-60 w-60 items-center justify-center">
          {/* 同心波纹（信号外扩） */}
          {[0, 1, 2].map((i) => (
            <span
              key={i}
              className="absolute rounded-full border border-[#8af3ff]"
              style={{
                width: "100%", height: "100%",
                animation: `wave 2.2s ease-out ${i * 0.7}s infinite`,
                opacity: 0,
              }}
            />
          ))}
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
              {count > 0 ? count : "📡"}
            </div>
            <div className="mt-2 text-xs tracking-[0.34em] text-[#7ea6c8]">信号同步中</div>
          </div>
          <style>{`
            @keyframes tick{0%{transform:scale(1.45);opacity:.2}100%{transform:scale(1);opacity:1}}
            @keyframes wave{0%{transform:scale(.45);opacity:.7}100%{transform:scale(1.25);opacity:0}}
          `}</style>
        </div>
      )}

      {/* 阶段二：通讯光束锁定（卫星与星球之间链路点亮 + 信号流动） */}
      {phase === "linking" && (
        <div className="relative h-64 w-full max-w-xl">
          <svg viewBox="0 0 480 240" className="h-full w-full">
            {/* 目标星球（右侧） */}
            <g className="planet-lock" style={{ transformOrigin: "400px 120px" }}>
              <circle cx="400" cy="120" r="34" fill="none" stroke="#8af3ff" strokeWidth="1.5" opacity="0.9" />
              <ellipse cx="400" cy="120" rx="52" ry="12" fill="none" stroke="#2e5f78" strokeWidth="1" transform="rotate(-14 400 120)" />
            </g>
            {/* 卫星（左侧，线框） */}
            <g className="sat-hold" style={{ transformOrigin: "80px 120px" }}>
              <rect x="58" y="104" width="44" height="30" rx="5" fill="none" stroke="#8af3ff" strokeWidth="1.5" />
              <line x1="58" y1="114" x2="102" y2="114" stroke="#2e5f78" strokeWidth="1" />
              <line x1="58" y1="124" x2="102" y2="124" stroke="#2e5f78" strokeWidth="1" />
              <rect x="16" y="108" width="34" height="22" fill="none" stroke="#8af3ff" strokeWidth="1.2" />
              <rect x="110" y="108" width="34" height="22" fill="none" stroke="#8af3ff" strokeWidth="1.2" />
              {/* 卫星天线朝向星球（通讯姿态） */}
              <path d="M 70 104 A 18 12 0 0 1 94 104 L 82 92 Z" fill="none" stroke="#8af3ff" strokeWidth="1.2" />
            </g>
            {/* 通讯光束：底轨虚线 → 主光束生长点亮 */}
            <line x1="104" y1="119" x2="366" y2="119" stroke="#1d425f" strokeWidth="1" strokeDasharray="3 7" />
            <line className="beam" x1="104" y1="119" x2="366" y2="119" stroke="#8af3ff" strokeWidth="2.5" strokeLinecap="round" style={{ filter: "drop-shadow(0 0 7px #8af3ff)" }} />
            {/* 信号包沿光束流动（三枚错峰） */}
            <circle className="sig s1" r="4" fill="#eaffff" style={{ filter: "drop-shadow(0 0 5px #8af3ff)" }} />
            <circle className="sig s2" r="3" fill="#8af3ff" />
            <circle className="sig s3" r="3" fill="#8af3ff" />
            {/* 星球端信号弧（接收确认） */}
            <path className="recv" d="M 386 100 A 22 22 0 0 1 386 140" fill="none" stroke="#8af3ff" strokeWidth="2" strokeLinecap="round" />
          </svg>
          <p className="mt-3 text-center text-sm tracking-[0.3em] text-[#7ea6c8]">通讯链路锁定 · 中继卫星握手</p>
          <style>{`
            .beam{stroke-dasharray:262;stroke-dashoffset:262;animation:grow ${linkingMs * 0.6}ms ease-out forwards}
            @keyframes grow{to{stroke-dashoffset:0}}
            .sat-hold{animation:hold 2.4s ease-in-out infinite alternate}
            @keyframes hold{from{transform:translateY(0)}to{transform:translateY(-6px)}}
            .planet-lock{animation:lock 1.6s ease-in-out 2 alternate}
            @keyframes lock{from{transform:scale(1)}to{transform:scale(1.07)}}
            .sig{opacity:0}
            .s1{animation:flow 1.1s linear ${linkingMs * 0.55}ms infinite}
            .s2{animation:flow 1.1s linear ${linkingMs * 0.55 + 380}ms infinite}
            .s3{animation:flow 1.1s linear ${linkingMs * 0.55 + 760}ms infinite}
            @keyframes flow{0%{offset-path:path("M 104 119 L 366 119");offset-distance:0%;opacity:1}100%{offset-path:path("M 104 119 L 366 119");offset-distance:100%;opacity:1}}
            .recv{opacity:0;animation:recvblink .6s ease-out ${linkingMs - 500}ms 3 forwards}
            @keyframes recvblink{0%{opacity:0}50%{opacity:1}100%{opacity:.25}}
          `}</style>
        </div>
      )}

      {/* 阶段三：通讯已建立（信号强度图标弹入 + 光晕） */}
      {phase === "success" && (
        <div className="text-center" style={{ animation: "pop .5s cubic-bezier(.2,1.6,.4,1)" }}>
          <div className="relative mx-auto flex h-24 w-24 items-center justify-center">
            <div className="absolute inset-0 rounded-full" style={{ boxShadow: "0 0 40px 12px rgba(138,243,255,.45)" }} />
            {/* 信号强度图标（弧线 + 圆点，描边生长） */}
            <svg viewBox="0 0 96 96" className="h-full w-full">
              <circle cx="48" cy="48" r="42" fill="none" stroke="#8af3ff" strokeWidth="2.5" />
              <circle cx="34" cy="62" r="5" fill="#eaffff" />
              <path className="arc a1" d="M 44 52 A 14 14 0 0 1 58 62" fill="none" stroke="#eaffff" strokeWidth="5" strokeLinecap="round" />
              <path className="arc a2" d="M 36 44 A 25 25 0 0 1 61 61" fill="none" stroke="#8af3ff" strokeWidth="5" strokeLinecap="round" />
              <path className="arc a3" d="M 28 36 A 37 37 0 0 1 64 60" fill="none" stroke="#5ec8d8" strokeWidth="5" strokeLinecap="round" />
            </svg>
          </div>
          <h2 className="mt-6 font-display text-2xl font-bold text-[#eaffff]">通讯已建立</h2>
          <p className="mt-2 text-sm text-[#8fb6d8]">
            本站与 <span className="text-[#8af3ff]">{relayName || "中继卫星"}</span> 的链路已锁定 · 大世界频道开通
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
      .arc{stroke-dasharray:60;stroke-dashoffset:60;animation:draw .5s ease-out forwards}
      .a1{animation-delay:.25s}.a2{animation-delay:.45s}.a3{animation-delay:.65s}
      @keyframes draw{to{stroke-dashoffset:0}}`}</style>
    </div>
  );
}
