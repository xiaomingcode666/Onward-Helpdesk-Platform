import type { ExtractedMessages } from "./types"

export const transferMessages = {
  "zh-CN": {
    transferExtract: {
      candidateInfo: {
        limitLabel: "不限",
        load: "负载",
        weight: "权重",
        candidateLabel:
          "{name} · {loadLabel} {capacity} · {weightLabel} {weight} · {reachability}",
        offlineDispatchable: "离线可派",
        onlineNow: "实时在线",
        onlineJustNow: "刚在线",
        onlineMinutesAgo: "{minutes}分钟前在线",
        onlineHoursAgo: "{hours}小时前在线",
      },
    },
  },
  "en-US": {
    transferExtract: {
      candidateInfo: {
        limitLabel: "Unlimited",
        load: "Load",
        weight: "Weight",
        candidateLabel:
          "{name} · {loadLabel} {capacity} · {weightLabel} {weight} · {reachability}",
        offlineDispatchable: "Dispatchable offline",
        onlineNow: "Online now",
        onlineJustNow: "Online just now",
        onlineMinutesAgo: "Online {minutes} min ago",
        onlineHoursAgo: "Online {hours} h ago",
      },
    },
  },
  "es-ES": {
    transferExtract: {
      candidateInfo: {
        limitLabel: "Ilimitado",
        load: "Carga",
        weight: "Peso",
        candidateLabel:
          "{name} · {loadLabel} {capacity} · {weightLabel} {weight} · {reachability}",
        offlineDispatchable: "Asignable sin conexion",
        onlineNow: "En linea ahora",
        onlineJustNow: "En linea hace un momento",
        onlineMinutesAgo: "En linea hace {minutes} min",
        onlineHoursAgo: "En linea hace {hours} h",
      },
    },
  },
} satisfies ExtractedMessages
