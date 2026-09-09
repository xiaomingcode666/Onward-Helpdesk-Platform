type TranslateFn = (key: string, values?: Record<string, string | number>) => string

export function renderCustomerConversationSummary(
  summary: string | undefined,
  t: TranslateFn,
  fallback = "",
) {
  const trimmed = (summary ?? "").trim()
  if (!trimmed) {
    return fallback
  }

  const feedback = trimmed.match(/^客户已提交服务评价[:：]\s*(\d+)\/5(?:\s*·\s*(.+))?$/)
  if (feedback) {
    const label = t("portalExtract.customerMobile.conversation.summaryEvents.feedbackSubmitted", { rating: feedback[1] })
    return feedback[2] ? `${label} · ${feedback[2]}` : label
  }

  const ticketCreated = trimmed.match(/^已生成服务工单\s+([^，,]+).*$/)
  if (ticketCreated?.[1]) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.ticketCreatedWithTicketNo", {
      ticketNo: ticketCreated[1],
    })
  }
  if (trimmed.startsWith("已生成服务工单")) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.ticketCreated")
  }

  const supplierInvited = trimmed.match(/^已邀请供应商协作[:：]\s*(.+)$/)
  if (supplierInvited?.[1]) {
    const parts = supplierInvited[1].split(/\s*[·/／]\s*/).map((part) => part.trim()).filter(Boolean)
    if (parts[0] && parts[1]) {
      return t("portalExtract.customerMobile.conversation.summaryEvents.supplierInvitedWithModule", {
        company: parts[0],
        module: parts.slice(1).join(" · "),
      })
    }
    if (parts[0]) {
      return t("portalExtract.customerMobile.conversation.summaryEvents.supplierInvitedWithCompany", { company: parts[0] })
    }
  }

  const supplierJoined = trimmed.match(/^(.+?)已加入协作会话/)
  if (supplierJoined?.[1]) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.supplierJoinedWithCompany", {
      company: supplierJoined[1].trim(),
    })
  }

  const supplierResolved = trimmed.match(/^供应商处理完成[:：]\s*(.+)$/)
  if (supplierResolved?.[1]) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.supplierResolvedWithResolution", {
      resolution: supplierResolved[1].trim(),
    })
  }

  const roomName = extractConversationRoomName(trimmed)
  const duration = extractConversationDuration(trimmed)
  if (trimmed.startsWith("视频协作已结束")) {
    if (roomName && duration) {
      return t("portalExtract.customerMobile.conversation.summaryEvents.videoEndedWithRoomDuration", {
        roomName,
        duration: localizeConversationDuration(duration, t),
      })
    }
    if (duration) {
      return t("portalExtract.customerMobile.conversation.summaryEvents.videoEndedWithDuration", {
        duration: localizeConversationDuration(duration, t),
      })
    }
    return t("portalExtract.customerMobile.conversation.summaryEvents.videoEnded")
  }

  if (trimmed.startsWith("发起视频协作")) {
    return roomName
      ? t("portalExtract.customerMobile.conversation.summaryEvents.videoStartedWithRoom", { roomName })
      : t("portalExtract.customerMobile.conversation.summaryEvents.videoStarted")
  }

  if (trimmed.startsWith("工程师已发起视频协作")) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.engineerVideoStarted")
  }

  if (trimmed.startsWith("工程师已预定视频协作")) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.engineerVideoScheduled")
  }

  const media = trimmed.match(/^\[(图片|语音|附件)\]\s*(.*)$/)
  if (media) {
    const labelKey = media[1] === "图片"
      ? "image"
      : media[1] === "语音"
        ? "audio"
        : "attachment"
    const label = t(`portalExtract.customerMobile.conversation.summaryEvents.${labelKey}`)
    return media[2] ? `${label} ${media[2]}` : label
  }

  if (trimmed === "维修结论已提交，等待客户确认设备状态。" || trimmed === "维修结论已提交，等待客户确认服务结果。") {
    return t("portalExtract.customerMobile.conversation.summaryEvents.repairSubmittedWaiting")
  }
  if (trimmed.startsWith("维修结论已提交：")) {
    return t("portalExtract.customerMobile.conversation.summaryEvents.repairSubmittedWithSummary", {
      summary: trimmed.replace(/^维修结论已提交[:：]\s*/, ""),
    })
  }

  return trimmed
}

function extractConversationRoomName(content: string) {
  return content.match(/会议室[:：]\s*([^，,]+)/)?.[1]?.trim() || ""
}

function extractConversationDuration(content: string) {
  return content.match(/时长[:：]\s*(.+)$/)?.[1]?.trim() || ""
}

function localizeConversationDuration(duration: string, t: TranslateFn) {
  const normalized = duration.replace(/\s+/g, " ").trim()
  const minuteSecond = normalized.match(/^(\d+)\s*分(?:\s*(\d+)\s*秒)?$/)
  if (minuteSecond) {
    const minutes = Number(minuteSecond[1])
    const seconds = Number(minuteSecond[2] || 0)
    return seconds > 0
      ? t("portalExtract.customerMobile.time.minutesSeconds", { minutes, seconds })
      : t("portalExtract.customerMobile.time.minutes", { minutes })
  }
  const secondOnly = normalized.match(/^(\d+)\s*秒$/)
  if (secondOnly) {
    return t("portalExtract.customerMobile.time.seconds", { seconds: Number(secondOnly[1]) })
  }
  return duration
}
