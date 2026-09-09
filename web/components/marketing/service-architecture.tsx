import {
  BotMessageSquareIcon,
  Building2Icon,
  CircleUserRoundIcon,
  DatabaseIcon,
  KeyRoundIcon,
  NetworkIcon,
  ShieldCheckIcon,
  TicketCheckIcon,
  UsersRoundIcon,
  VideoIcon,
  WorkflowIcon,
} from "lucide-react"

const serviceEntries = [
  { icon: CircleUserRoundIcon, title: "客户 H5", detail: "咨询、附件与转人工" },
  { icon: Building2Icon, title: "企业工作台", detail: "知识与服务运营" },
  { icon: NetworkIcon, title: "合作伙伴端", detail: "授权工单协作" },
  { icon: KeyRoundIcon, title: "平台管理端", detail: "租户与能力治理" },
] as const

const serviceCapabilities = [
  { icon: BotMessageSquareIcon, title: "AI 问诊", detail: "检索、引用与追问" },
  { icon: UsersRoundIcon, title: "人工接管", detail: "在原会话继续处理" },
  { icon: TicketCheckIcon, title: "工单协同", detail: "派单、SLA 与状态" },
  { icon: VideoIcon, title: "远程服务", detail: "工程师与专家协作" },
] as const

const contextItems = ["客户", "会话", "诊断", "工单", "视频", "服务记录"] as const

const dataItems = ["客户资料", "企业知识", "产品 / 设备可选", "维修与审计记录"] as const

type ArchitectureNode = (typeof serviceEntries)[number] | (typeof serviceCapabilities)[number]

function DiagramNode({ node, side }: { node: ArchitectureNode; side: "entry" | "capability" }) {
  const Icon = node.icon

  return (
    <div className="group/node relative flex min-h-[74px] items-center gap-3 rounded-md border border-white/10 bg-white/[0.045] px-4 shadow-[0_14px_34px_rgba(0,0,0,0.18)] backdrop-blur-sm transition hover:border-white/20 hover:bg-white/[0.065]">
      <span className={`grid size-9 shrink-0 place-items-center rounded-md border ${side === "entry" ? "border-emerald-300/20 bg-emerald-300/10 text-emerald-300" : "border-blue-300/20 bg-blue-300/10 text-blue-300"}`}>
        <Icon className="size-4.5" />
      </span>
      <span className="min-w-0">
        <strong className="block text-sm font-semibold text-white">{node.title}</strong>
        <small className="mt-1 block text-xs leading-4 text-slate-400">{node.detail}</small>
      </span>
      <span
        className={`absolute top-1/2 hidden size-2 -translate-y-1/2 rounded-full border-2 border-[#0a121a] shadow-sm lg:block ${
          side === "entry" ? "-right-1 bg-emerald-400" : "-left-1 bg-blue-400"
        }`}
      />
    </div>
  )
}

function VerticalConnector() {
  return (
    <div className="relative mx-auto h-10 w-px overflow-hidden bg-white/15 lg:hidden" aria-hidden="true">
      <span className="rhd-architecture-drop absolute left-0 top-0 h-4 w-px bg-blue-400" />
    </div>
  )
}

function ContextCore() {
  return (
    <div className="relative overflow-hidden rounded-lg border border-blue-300/25 bg-[#101b25] p-5 text-white shadow-[0_0_0_1px_rgba(96,165,250,0.04),0_30px_80px_rgba(0,0,0,0.42)] sm:p-6">
      <span className="absolute left-0 top-0 h-px w-20 bg-blue-300/80" />
      <span className="absolute right-0 top-0 h-20 w-px bg-emerald-300/60" />
      <span className="absolute bottom-0 left-0 h-16 w-px bg-violet-300/50" />
      <div className="flex items-center justify-between gap-4">
        <span className="inline-flex items-center gap-2 text-[11px] font-semibold text-blue-300">
          <ShieldCheckIcon className="size-4" />
          02 / TENANT SERVICE CORE
        </span>
        <span className="inline-flex items-center gap-2 text-[11px] text-slate-300">
          <span className="size-2 rounded-full bg-emerald-400 motion-safe:animate-pulse" /> LIVE
        </span>
      </div>

      <div className="mt-6 text-center">
        <span className="mx-auto grid size-11 place-items-center rounded-md border border-blue-300/30 bg-blue-300/10 text-blue-200 shadow-[0_0_28px_rgba(96,165,250,0.16)]">
          <WorkflowIcon className="size-5" />
        </span>
        <h3 className="mt-4 text-xl font-semibold sm:text-2xl">统一服务上下文</h3>
        <p className="mx-auto mt-2 max-w-sm text-xs leading-5 text-slate-300">
          从客户发起到服务完成，每个角色使用同一份信息。
        </p>
      </div>

      <div className="mt-5 flex flex-wrap justify-center gap-2">
        {contextItems.map((item) => (
          <span key={item} className="rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-[11px] text-slate-200">
            {item}
          </span>
        ))}
      </div>

      <div className="mt-6 grid grid-cols-3 border-t border-white/10 pt-4 text-center text-[11px] text-slate-300">
        <span>身份权限</span>
        <span className="border-x border-white/10">工作流引擎</span>
        <span>审计通知</span>
      </div>
    </div>
  )
}

function DataFoundation() {
  return (
    <div className="rounded-md border border-dashed border-violet-300/25 bg-violet-300/[0.05] px-4 py-3 backdrop-blur-sm">
      <div className="flex items-center justify-center gap-2 text-xs font-semibold text-slate-200">
        <DatabaseIcon className="size-4 text-violet-300" />
        租户隔离数据层
      </div>
      <div className="mt-2 flex flex-wrap justify-center gap-x-4 gap-y-1 text-[11px] text-slate-400">
        {dataItems.map((item) => <span key={item}>{item}</span>)}
      </div>
    </div>
  )
}

const entryPaths = [
  "M 230 65 C 292 65, 310 205, 372 205",
  "M 230 151 C 292 151, 310 205, 372 205",
  "M 230 237 C 292 237, 310 205, 372 205",
  "M 230 323 C 292 323, 310 205, 372 205",
] as const

const capabilityPaths = [
  "M 828 205 C 890 205, 908 65, 970 65",
  "M 828 205 C 890 205, 908 151, 970 151",
  "M 828 205 C 890 205, 908 237, 970 237",
  "M 828 205 C 890 205, 908 323, 970 323",
] as const

function DesktopFlowLines() {
  return (
    <svg className="pointer-events-none absolute inset-0 size-full" viewBox="0 0 1200 500" preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <marker id="rhd-entry-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="#6ee7b7" />
        </marker>
        <marker id="rhd-capability-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" fill="#93c5fd" />
        </marker>
        <filter id="rhd-signal-glow" x="-200%" y="-200%" width="400%" height="400%">
          <feGaussianBlur stdDeviation="2.4" result="blur" />
          <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
      </defs>
      {entryPaths.map((path) => (
        <path
          key={path}
          d={path}
          className="rhd-architecture-flow"
          fill="none"
          markerEnd="url(#rhd-entry-arrow)"
          stroke="#6ee7b7"
          strokeDasharray="5 8"
          strokeLinecap="round"
          strokeWidth="1.6"
          vectorEffect="non-scaling-stroke"
        />
      ))}
      {capabilityPaths.map((path) => (
        <path
          key={path}
          d={path}
          className="rhd-architecture-flow rhd-architecture-flow-reverse"
          fill="none"
          markerEnd="url(#rhd-capability-arrow)"
          stroke="#93c5fd"
          strokeDasharray="5 8"
          strokeLinecap="round"
          strokeWidth="1.6"
          vectorEffect="non-scaling-stroke"
        />
      ))}
      {[entryPaths[0], entryPaths[2]].map((path, index) => (
        <circle key={path} r="3" fill="#6ee7b7" filter="url(#rhd-signal-glow)" className="motion-reduce:hidden">
          <animateMotion dur="3.8s" repeatCount="indefinite" path={path} begin={`${index * 0.8}s`} />
        </circle>
      ))}
      {[capabilityPaths[1], capabilityPaths[3]].map((path, index) => (
        <circle key={path} r="3" fill="#93c5fd" filter="url(#rhd-signal-glow)" className="motion-reduce:hidden">
          <animateMotion dur="4.2s" repeatCount="indefinite" path={path} begin={`${index * 0.9}s`} />
        </circle>
      ))}
      <path
        d="M 600 388 L 600 422"
        className="rhd-architecture-flow"
        fill="none"
        markerEnd="url(#rhd-capability-arrow)"
        stroke="#c4b5fd"
        strokeDasharray="5 8"
        strokeLinecap="round"
        strokeWidth="1.6"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  )
}

export function ServiceArchitecture() {
  return (
    <div className="relative isolate overflow-hidden rounded-lg border border-slate-800 bg-[#091119] px-4 py-6 shadow-[0_34px_90px_rgba(2,8,23,0.28)] sm:px-6 sm:py-8 lg:px-8 lg:py-10">
      <style>{`
        @keyframes rhd-architecture-flow {
          to { stroke-dashoffset: -104; }
        }
        @keyframes rhd-architecture-drop {
          0% { transform: translateY(-18px); opacity: 0; }
          20%, 75% { opacity: 1; }
          100% { transform: translateY(40px); opacity: 0; }
        }
        .rhd-architecture-flow { animation: rhd-architecture-flow 5s linear infinite; }
        .rhd-architecture-flow-reverse { animation-direction: reverse; }
        .rhd-architecture-drop { animation: rhd-architecture-drop 2.2s ease-in-out infinite; }
        @media (prefers-reduced-motion: reduce) {
          .rhd-architecture-flow, .rhd-architecture-drop { animation: none; }
        }
      `}</style>

      <div className="pointer-events-none absolute inset-0 -z-10 opacity-[0.055] [background-image:linear-gradient(rgba(148,163,184,0.9)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.9)_1px,transparent_1px)] [background-size:32px_32px]" />

      <div className="flex flex-col gap-3 border-b border-white/10 pb-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="font-mono text-[10px] font-semibold text-blue-300">REMOTEHELPDESK / SERVICE FABRIC</p>
          <p className="mt-2 text-sm text-slate-400">4 个服务入口 · 1 个上下文核心 · 4 个执行域</p>
        </div>
        <span className="inline-flex w-fit items-center gap-2 rounded-full border border-emerald-300/20 bg-emerald-300/[0.06] px-3 py-1.5 text-[10px] font-semibold text-emerald-300">
          <span className="size-1.5 rounded-full bg-emerald-300 motion-safe:animate-pulse" /> CONTEXT NETWORK ONLINE
        </span>
      </div>

      <div className="relative mx-auto mt-7 hidden min-h-[500px] max-w-[1200px] lg:block">
        <DesktopFlowLines />

        <div className="absolute left-0 top-0 z-10 w-[230px]">
          <p className="mb-3 font-mono text-[10px] font-semibold text-emerald-300">01 / SERVICE ENTRIES</p>
          <div className="grid gap-3">
            {serviceEntries.map((node) => <DiagramNode key={node.title} node={node} side="entry" />)}
          </div>
        </div>

        <div className="absolute right-0 top-0 z-10 w-[230px]">
          <p className="mb-3 text-right font-mono text-[10px] font-semibold text-blue-300">03 / SERVICE EXECUTION</p>
          <div className="grid gap-3">
            {serviceCapabilities.map((node) => <DiagramNode key={node.title} node={node} side="capability" />)}
          </div>
        </div>

        <div className="absolute left-[31%] right-[31%] top-[82px] z-10">
          <ContextCore />
        </div>

        <div className="absolute left-[31%] right-[31%] top-[422px] z-10">
          <DataFoundation />
        </div>
      </div>

      <div className="lg:hidden">
        <p className="mb-3 text-center font-mono text-[10px] font-semibold text-emerald-300">01 / SERVICE ENTRIES</p>
        <div className="grid grid-cols-2 gap-3">
          {serviceEntries.map((node) => <DiagramNode key={node.title} node={node} side="entry" />)}
        </div>

        <VerticalConnector />
        <ContextCore />
        <VerticalConnector />

        <p className="mb-3 text-center font-mono text-[10px] font-semibold text-blue-300">03 / SERVICE EXECUTION</p>
        <div className="grid grid-cols-2 gap-3">
          {serviceCapabilities.map((node) => <DiagramNode key={node.title} node={node} side="capability" />)}
        </div>

        <VerticalConnector />
        <DataFoundation />
      </div>
    </div>
  )
}
