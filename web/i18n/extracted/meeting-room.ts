import type { ExtractedMessages } from "./types"

export const meetingRoomMessages = {
  "zh-CN": {
    "meetingRoomExtract": {
      "roomClient": {
        "transcriptLoadFailed": "文字记录加载失败",
        "missingMeetingId": "缺少会议 ID",
        "joinConfigUnavailable": "会议加入配置不可用",
        "returnTicket": "返回工单",
        "returnVideoCenter": "返回视频中心",
        "collaborationEnded": "会议已结束，请重新发起视频协作",
        "meetingEnded": "会议已结束，请重新发起视频协作",
        "cannotEnterCollaboration": "无法进入协作",
        "retry": "重试",
        "configLoading": "会议配置加载中",
        "heartbeatFailed": "会议心跳失败",
        "transcriptArchiveFailed": "实时文字归档失败"
      },
      "reminders": {
        "startsImminently": "即将开始",
        "startsInMinutes": "{minutes} 分钟后开始",
        "startsWithinHour": "1 小时内开始",
        "meetingTitleFallback": "视频协作提醒",
        "meetingDescriptionFallback": "视频协作",
        "newTicketTitleFallback": "新工单",
        "productGroupNewTicket": "产品组新工单",
        "ticketAssignedToMe": "派给我的工单",
        "myProductGroup": "我的产品组",
        "scheduledAt": "计划时间 {date}",
        "createdAt": "创建时间 {date}",
        "assignedAt": "指派时间 {date}",
        "reminderPanelAria": "服务提醒",
        "serviceReminderPanel": "服务提醒",
        "oneItemNeedsAttention": "1 条需要关注",
        "itemsNeedAttention": "{count} 条需要关注",
        "closeReminder": "关闭提醒",
        "view": "查看",
        "acknowledge": "知道了",
        "acknowledgeItem": "知道了：{title}",
        "acknowledgeFailed": "操作失败，请重试",
        "openReadFailed": "已打开工单，但提醒未能标记为已读",
        "moreUnreadAssignments": "还有未读指派，前往消息中心查看"
      }
    }
  },
  "en-US": {
    "meetingRoomExtract": {
      "roomClient": {
        "transcriptLoadFailed": "Failed to load transcript",
        "missingMeetingId": "Missing meeting ID",
        "joinConfigUnavailable": "Meeting join config unavailable",
        "returnTicket": "Back to ticket",
        "returnVideoCenter": "Back to video center",
        "collaborationEnded": "The meeting has ended. Please start a new video collaboration.",
        "meetingEnded": "The meeting has ended. Please start a new video collaboration.",
        "cannotEnterCollaboration": "Cannot join collaboration",
        "retry": "Retry",
        "configLoading": "Loading meeting config",
        "heartbeatFailed": "Meeting heartbeat failed",
        "transcriptArchiveFailed": "Failed to archive live transcript"
      },
      "reminders": {
        "startsImminently": "Starting soon",
        "startsInMinutes": "Starts in {minutes} minutes",
        "startsWithinHour": "Starts within 1 hour",
        "meetingTitleFallback": "Video collaboration reminder",
        "meetingDescriptionFallback": "Video collaboration",
        "newTicketTitleFallback": "New ticket",
        "productGroupNewTicket": "New ticket in product group",
        "ticketAssignedToMe": "Ticket assigned to me",
        "myProductGroup": "My product group",
        "scheduledAt": "Scheduled at {date}",
        "createdAt": "Created at {date}",
        "assignedAt": "Assigned at {date}",
        "reminderPanelAria": "Service reminders",
        "serviceReminderPanel": "Service reminders",
        "oneItemNeedsAttention": "1 item needs attention",
        "itemsNeedAttention": "{count} items need attention",
        "closeReminder": "Close reminders",
        "view": "View",
        "acknowledge": "Got it",
        "acknowledgeItem": "Acknowledge: {title}",
        "acknowledgeFailed": "Could not acknowledge. Try again.",
        "openReadFailed": "The ticket opened, but the reminder could not be marked as read.",
        "moreUnreadAssignments": "More unread assignments in the message center"
      }
    }
  },
  "es-ES": {
    "meetingRoomExtract": {
      "roomClient": {
        "transcriptLoadFailed": "Error al cargar la transcripcion",
        "missingMeetingId": "Falta el ID de la reunion",
        "joinConfigUnavailable": "Configuracion de reunion no disponible",
        "returnTicket": "Volver al ticket",
        "returnVideoCenter": "Volver al centro de video",
        "collaborationEnded": "La reunion ha terminado. Inicia una nueva colaboracion por video.",
        "meetingEnded": "La reunion ha terminado. Inicia una nueva colaboracion por video.",
        "cannotEnterCollaboration": "No se puede entrar a la colaboracion",
        "retry": "Reintentar",
        "configLoading": "Cargando configuracion de la reunion",
        "heartbeatFailed": "El heartbeat de la reunion fallo",
        "transcriptArchiveFailed": "Error al archivar la transcripcion en vivo"
      },
      "reminders": {
        "startsImminently": "Comenzando pronto",
        "startsInMinutes": "Comienza en {minutes} minutos",
        "startsWithinHour": "Comienza dentro de 1 hora",
        "meetingTitleFallback": "Recordatorio de colaboracion por video",
        "meetingDescriptionFallback": "Colaboracion por video",
        "newTicketTitleFallback": "Nuevo ticket",
        "productGroupNewTicket": "Nuevo ticket en el grupo de productos",
        "ticketAssignedToMe": "Ticket asignado a mí",
        "myProductGroup": "Mi grupo de productos",
        "scheduledAt": "Programado el {date}",
        "createdAt": "Creado el {date}",
        "assignedAt": "Asignado el {date}",
        "reminderPanelAria": "Recordatorios de servicio",
        "serviceReminderPanel": "Recordatorios de servicio",
        "oneItemNeedsAttention": "1 elemento requiere atención",
        "itemsNeedAttention": "{count} elementos requieren atención",
        "closeReminder": "Cerrar recordatorios",
        "view": "Ver",
        "acknowledge": "Entendido",
        "acknowledgeItem": "Entendido: {title}",
        "acknowledgeFailed": "No se pudo confirmar. Inténtalo de nuevo.",
        "openReadFailed": "El ticket se abrió, pero el recordatorio no se pudo marcar como leído.",
        "moreUnreadAssignments": "Hay más asignaciones sin leer en el centro de mensajes"
      }
    }
  }
} satisfies ExtractedMessages
