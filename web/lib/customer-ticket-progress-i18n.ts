import type { CustomerPortalTicketProgress } from "@/lib/api/customer-portal-types"

type TicketProgressTranslator = (key: string, values?: Record<string, string | number>) => string

type TicketProgressRenderOptions = {
  hasDeviceConcept?: boolean
}

export function customerTicketProgressLabel(
  t: TicketProgressTranslator,
  item: CustomerPortalTicketProgress,
) {
  const content = stripTerminalPeriod(item.content)
  if (content.startsWith("取消工单") || content === "工单已取消") {
    return t("customerTickets.progressCancelled")
  }
  if (isSupplierProgress(content, item.metadata)) {
    return t("customerTickets.progressSupplierCollaboration")
  }
  if (isVideoProgress(content, item.event_type)) {
    return t("customerTickets.progressVideoCollaboration")
  }

  const labels: Record<string, string> = {
    created: t("customerTickets.progressCreated"),
    ticket_created: t("customerTickets.progressCreated"),
    assigned: t("customerTickets.progressAssigned"),
    ticket_assigned: t("customerTickets.progressAssigned"),
    accepted: t("customerTickets.progressAccepted"),
    ticket_accepted: t("customerTickets.progressAccepted"),
    processing: t("customerTickets.progressProcessing"),
    ticket_processing: t("customerTickets.progressProcessing"),
    escalated: t("customerTickets.progressEscalated"),
    ticket_escalated: t("customerTickets.progressEscalated"),
    pending_confirmation: t("customerTickets.progressPendingConfirmation"),
    completed: t("customerTickets.progressCompleted"),
    repair_completed: t("customerTickets.progressCompleted"),
    closed: t("customerTickets.progressClosed"),
    ticket_closed: t("customerTickets.progressClosed"),
    reopened: t("customerTickets.progressReopened"),
    ticket_reopened: t("customerTickets.progressReopened"),
    meeting_ended: t("customerTickets.progressVideoCollaboration"),
  }
  return labels[item.event_type] || t("customerTickets.progressFallback")
}

export function customerTicketProgressContent(
  t: TicketProgressTranslator,
  item: CustomerPortalTicketProgress,
  options: TicketProgressRenderOptions = {},
) {
  const rawContent = item.content.trim()
  if (!rawContent) return t("customerTickets.progressUpdated")

  const content = stripTerminalPeriod(rawContent)
  const hasDeviceConcept = options.hasDeviceConcept !== false

  if (content === "Draft ticket created; awaiting customer invitation acceptance") {
    return t("customerTickets.progressDraftCreated")
  }
  if (content === "服务工单已创建，正在安排工程师") {
    return t("customerTickets.progressCreatedScheduling")
  }
  if (["创建工单", "已创建工单", "服务工单已创建", "Created ticket", "Ticket created"].includes(content)) {
    return t("customerTickets.progressCreated")
  }

  const assignedWaiting = content.match(/^已分配给(.+?)[，,]\s*等待工程师接单$/)
  if (assignedWaiting) {
    return t("customerTickets.progressAssignedWaitingWithName", { assignee: assignedWaiting[1].trim() })
  }
  if (content === "已分配工程师，等待接单") {
    return t("customerTickets.progressAssignedWaiting")
  }
  if (content === "工单已进入派单流程") {
    return t("customerTickets.progressDispatchStarted")
  }
  if (content === "已分配处理人员") {
    return t("customerTickets.progressAssigned")
  }

  if (content.startsWith("工单随会话自动分配")) {
    const reason = extractReason(rawContent)
    return reason
      ? t("customerTickets.progressAutoAssignedWithReason", { reason: translateReason(t, reason) })
      : t("customerTickets.progressAutoAssigned")
  }
  if (content.startsWith("工单回到产品组待派单") || content.startsWith("工单回到技术支持组待派单")) {
    const reason = extractReason(rawContent)
    const key = hasDeviceConcept ? "progressReturnedToTeamPool" : "progressReturnedToSupportPool"
    return reason
      ? t(`customerTickets.${key}WithReason`, { reason: translateReason(t, reason) })
      : t(`customerTickets.${key}`)
  }

  const manualAssignment = rawContent.match(/^分配工单(?:[，,]\s*原处理人[:：]\s*(.+?))?\s*->\s*(.+?)(?:[，,]\s*原因[:：]\s*(.+))?$/)
  if (manualAssignment) {
    const previousAssignee = manualAssignment[1]?.trim()
    const assignee = manualAssignment[2].trim()
    const reason = manualAssignment[3]?.trim()
    if (previousAssignee && reason) {
      return t("customerTickets.progressReassignedWithReason", {
        previousAssignee,
        assignee,
        reason: translateReason(t, reason),
      })
    }
    if (previousAssignee) {
      return t("customerTickets.progressReassigned", { previousAssignee, assignee })
    }
    if (reason) {
      return t("customerTickets.progressAssignedToWithReason", {
        assignee,
        reason: translateReason(t, reason),
      })
    }
    return t("customerTickets.progressAssignedTo", { assignee })
  }

  const automaticAssignment = rawContent.match(/^工单(.+?)\s*->\s*等待工程师接单$/)
  if (automaticAssignment) {
    return t("customerTickets.progressAutomaticAssignmentWaiting", {
      reason: translateReason(t, automaticAssignment[1]),
    })
  }

  if (content === "关联会话状态已恢复为待接入") {
    return t("customerTickets.progressConversationReturnedToQueue")
  }
  if (content === "团队已停用，工单回到全局待派池") {
    return t("customerTickets.progressTeamDisabledReturnedToPool")
  }
  if (content === "负责人不可接单，重新派单") {
    return t("customerTickets.progressAssigneeUnavailableRedispatch")
  }
  if (content === "接单超时，重新派单") {
    return t("customerTickets.progressAcceptanceTimedOutRedispatch")
  }

  if (content === "受理工单" || content === "工程师已接单") {
    return t("customerTickets.progressAccepted")
  }
  if (content === "工程师已接单，正在准备处理") {
    return t("customerTickets.progressAcceptedPreparing")
  }
  if (content === "技术支持组成员接管并受理") {
    return t("customerTickets.progressSupportTakeoverAccepted")
  }
  if (content === "同产品维修组成员接管并受理") {
    return t(hasDeviceConcept ? "customerTickets.progressSameProductTakeoverAccepted" : "customerTickets.progressSupportTakeoverAccepted")
  }
  if (content === "原处理人接单超时，技术支持组成员接管并受理") {
    return t("customerTickets.progressExpiredSupportTakeoverAccepted")
  }
  if (content === "原处理人接单超时，产品组成员接管并受理") {
    return t(hasDeviceConcept ? "customerTickets.progressExpiredTakeoverAccepted" : "customerTickets.progressExpiredSupportTakeoverAccepted")
  }
  if (content === "技术支持组成员从待派单池转派给自己并受理") {
    return t("customerTickets.progressSupportPoolTakeoverAccepted")
  }
  if (content === "产品组成员从待派单池转派给自己并受理") {
    return t(hasDeviceConcept ? "customerTickets.progressPoolTakeoverAccepted" : "customerTickets.progressSupportPoolTakeoverAccepted")
  }
  if (content === "工程师正在处理该工单" || content === "工单处理中") {
    return t("customerTickets.progressProcessing")
  }

  if (content === "多次接单超时，自动升级主管") {
    return t("customerTickets.progressRepeatedTimeoutEscalated")
  }
  if (content === "已升级协作处理") {
    return t("customerTickets.progressEscalated")
  }
  const escalated = rawContent.match(/^升级工单(?:[，,]\s*原因[:：]\s*(.+))?$/)
  if (escalated) {
    const reason = escalated[1]?.trim()
    return reason
      ? t("customerTickets.progressEscalatedWithReason", { reason: translateReason(t, reason) })
      : t("customerTickets.progressEscalated")
  }
  const noAssigneeEscalation = rawContent.match(/^持续无人可派[，,]\s*主管兜底接管[:：]\s*(.+)$/)
  if (noAssigneeEscalation) {
    return t("customerTickets.progressNoAssigneeSupervisorTakeover", {
      reason: translateReason(t, noAssigneeEscalation[1]),
    })
  }

  if (content === "已填写维修记录") {
    return t("customerTickets.progressRepairRecordFilled")
  }
  if (content === "维修结论已提交，等待确认") {
    return t("customerTickets.progressRepairPendingConfirmation")
  }

  if (content === "工单已取消") {
    return t("customerTickets.progressCancelled")
  }
  const cancelled = rawContent.match(/^取消工单[，,]\s*原因[:：]\s*(.+)$/)
  if (cancelled) {
    return t("customerTickets.progressCancelledWithReason", { reason: cancelled[1].trim() })
  }
  if (content === "关闭工单" || content === "工单已关闭") {
    return t("customerTickets.progressClosed")
  }
  const closed = rawContent.match(/^关闭工单[，,]\s*结论[:：]\s*(.+)$/)
  if (closed) {
    return t("customerTickets.progressClosedWithConclusion", {
      conclusion: translateConclusion(t, closed[1]),
    })
  }

  if (content === "重新打开工单" || content === "工单已重新打开" || content === "工单已重新打开，正在继续处理") {
    return t("customerTickets.progressReopened")
  }
  const reopened = rawContent.match(/^重新打开工单[，,]\s*原因[:：]\s*(.+)$/)
  if (reopened) {
    return t("customerTickets.progressReopenedWithReason", { reason: reopened[1].trim() })
  }

  const scheduledVideo = rawContent.match(/^预定视频协作[，,]\s*计划时间[:：]\s*(.+?)[，,]\s*会议室[:：]\s*(.+)$/)
  if (scheduledVideo) {
    return t("customerTickets.progressVideoScheduledWithTimeRoom", {
      scheduledAt: scheduledVideo[1].trim(),
      roomName: scheduledVideo[2].trim(),
    })
  }
  const scheduledVideoRoom = rawContent.match(/^预定视频协作[，,]\s*会议室[:：]\s*(.+)$/)
  if (scheduledVideoRoom) {
    return t("customerTickets.progressVideoScheduledWithRoom", { roomName: scheduledVideoRoom[1].trim() })
  }
  const startedVideo = rawContent.match(/^发起视频协作[，,]\s*会议室[:：]\s*(.+)$/)
  if (startedVideo) {
    return t("customerTickets.progressVideoStartedWithRoom", { roomName: startedVideo[1].trim() })
  }
  if (content === "视频协作暂时创建失败，已自动降级为文字/图片协同；工程师可以稍后重试发起视频") {
    return t("customerTickets.progressVideoCreationFailed")
  }

  const videoEnded = rawContent.match(/^视频协作已结束(?:[，,]\s*会议室[:：]\s*([^，,]+))?(?:[，,]\s*时长[:：]\s*(.+))?$/)
  if (videoEnded) {
    const roomName = videoEnded[1]?.trim()
    const duration = translateDuration(t, videoEnded[2])
    if (roomName && duration) {
      return t("customerTickets.progressVideoEndedWithRoomDuration", { roomName, duration })
    }
    if (duration) {
      return t("customerTickets.progressVideoEndedWithDuration", { duration })
    }
    return t("customerTickets.progressVideoEnded")
  }

  const supplierInvited = rawContent.match(/^升级供应商协作[:：]\s*(.+?)(?:\s*\/\s*(.+))?$/)
  if (supplierInvited) {
    const company = supplierInvited[1].trim()
    const moduleName = supplierInvited[2]?.trim()
    return moduleName
      ? t("customerTickets.progressSupplierInvitedWithModule", { company, module: moduleName })
      : t("customerTickets.progressSupplierInvited", { company })
  }
  if (content === "供应商协作授权已到期，供应商访问已回收，工单返回企业工程师继续处理") {
    return t("customerTickets.progressSupplierAuthorizationExpired")
  }
  if (content === "供应商已接单") {
    return t("customerTickets.progressSupplierAccepted")
  }
  const supplierTimeout = rawContent.match(/^供应商未响应超时[，,]\s*已升级企业侧继续处理[:：]\s*(.+)$/)
  if (supplierTimeout) {
    return t("customerTickets.progressSupplierTimeout", { target: supplierTimeout[1].trim() })
  }
  const supplierAssigned = rawContent.match(/^供应商内部派工[:：]\s*(.+)$/)
  if (supplierAssigned) {
    return t("customerTickets.progressSupplierAssigned", { assignee: supplierAssigned[1].trim() })
  }
  const supplierMemberJoined = rawContent.match(/^供应商协作成员加入[:：]\s*(.+)$/)
  if (supplierMemberJoined) {
    return t("customerTickets.progressSupplierMemberJoined", { member: supplierMemberJoined[1].trim() })
  }
  const supplierMemberLeft = rawContent.match(/^供应商协作成员退出[:：]\s*(.+)$/)
  if (supplierMemberLeft) {
    return t("customerTickets.progressSupplierMemberLeft", { member: supplierMemberLeft[1].trim() })
  }
  const supplierResolved = rawContent.match(/^供应商处理完成[:：]\s*(.+)$/)
  if (supplierResolved) {
    return t("customerTickets.progressSupplierResolved", { resolution: supplierResolved[1].trim() })
  }

  const mediaSummary = rawContent.match(/^\[(图片|语音|附件)\](?:\s+(.+))?$/)
  if (mediaSummary) {
    const key = mediaSummary[1] === "图片"
      ? "progressImage"
      : mediaSummary[1] === "语音"
        ? "progressAudio"
        : "progressAttachment"
    const filename = mediaSummary[2]?.trim()
    return filename
      ? t(`customerTickets.${key}WithFilename`, { filename })
      : t(`customerTickets.${key}`)
  }

  if (content === "工单进度已更新") {
    return t("customerTickets.progressUpdated")
  }

  return rawContent
}

function stripTerminalPeriod(value: string) {
  return value.trim().replace(/[。.]+$/, "").trim()
}

function extractReason(value: string) {
  return value.match(/原因[:：]\s*(.+)$/)?.[1]?.trim() || ""
}

function translateReason(t: TicketProgressTranslator, value: string) {
  const reason = value.trim()
  const reasonKeys: Record<string, string> = {
    自动分配: "autoAssign",
    手动触发自动分配: "manualAutoAssign",
    创建工单时指定负责人: "initialAssignee",
    客户接受邀请后激活草稿工单: "customerInvitationAccepted",
    candidate_became_unavailable: "candidateUnavailable",
    no_alternative_after_accept_timeout: "noAlternativeAfterTimeout",
    no_alternative_after_assignee_unavailable: "noAlternativeAfterUnavailable",
    accept_timeout_redispatching: "acceptTimeoutRedispatching",
    assignee_unavailable_redispatching: "assigneeUnavailableRedispatching",
    missing_supervisor: "missingSupervisor",
    team_disabled_pool_released: "teamDisabled",
    unknown: "unknown",
  }
  const key = reasonKeys[reason]
  return key ? t(`customerTickets.progressReasons.${key}`) : reason
}

function translateConclusion(t: TicketProgressTranslator, value: string) {
  const conclusion = value.trim()
  return conclusion === "客户主动结束工单"
    ? t("customerTickets.progressConclusionCustomerEnded")
    : conclusion
}

function translateDuration(t: TicketProgressTranslator, value: string | undefined) {
  const duration = value?.replace(/\s+/g, " ").trim() || ""
  if (!duration) return ""
  if (duration === "不足 1 秒") return t("customerTickets.progressDurationLessThanSecond")

  const hourMinute = duration.match(/^(\d+)\s*小时(?:\s*(\d+)\s*分)?$/)
  if (hourMinute) {
    const hours = Number(hourMinute[1])
    const minutes = Number(hourMinute[2] || 0)
    return minutes > 0
      ? t("customerTickets.progressDurationHoursMinutes", { hours, minutes })
      : t("customerTickets.progressDurationHours", { hours })
  }
  const minuteSecond = duration.match(/^(\d+)\s*分(?:\s*(\d+)\s*秒)?$/)
  if (minuteSecond) {
    const minutes = Number(minuteSecond[1])
    const seconds = Number(minuteSecond[2] || 0)
    return seconds > 0
      ? t("customerTickets.progressDurationMinutesSeconds", { minutes, seconds })
      : t("customerTickets.progressDurationMinutes", { minutes })
  }
  const seconds = duration.match(/^(\d+)\s*秒$/)
  return seconds
    ? t("customerTickets.progressDurationSeconds", { seconds: Number(seconds[1]) })
    : duration
}

function isSupplierProgress(content: string, metadata: Record<string, string> | undefined) {
  return content.startsWith("供应商") || metadata?.source === "partner"
}

function isVideoProgress(content: string, eventType: string) {
  return eventType === "meeting_ended" || content.includes("视频协作")
}
