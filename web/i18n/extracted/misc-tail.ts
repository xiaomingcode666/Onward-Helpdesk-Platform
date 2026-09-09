import type { ExtractedMessages } from "./types"

export const miscTailMessages = {
  "zh-CN": {
    miscTailExtract: {
      dashboardMisc: {
        products: {
          productCenterTabsAria: "产品中心分区",
        },
        skillDefinition: {
          columnSkill: "技能",
          receptionConfig: "接待配置",
        },
        knowledge: {
          tabsAriaFallback: "知识库分区",
        },
        conversations: {
          filterAriaFallback: "会话筛选",
        },
        conversationMonitor: {
          senderAi: "在线接待",
        },
        channels: {
          effectiveVersion: "{name} · 当前生效 #{version}",
          unpublished: "{name} · 未发布",
          publishFlowMissingToast: "该接待配置尚未发布流程，不能绑定渠道",
          publishFlowMissingHint: "该接待配置尚未发布流程。",
          publishedBadge: "已发布",
          effectiveVersionText: "当前生效版本 #{version}",
        },
      },
      uiShared: {
        commandSearch: "搜索",
        sidebarNav: "导航",
        expandTags: "展开标签",
        collapseTags: "折叠标签",
        brandAlt: "设备售后平台",
      },
      libMisc: {
        enterpriseProducts: {
          productCatalogLoadFailed: "产品目录加载失败",
        },
        customerPortal: {
          formalAccountLoginRequired: "请先使用正式用户账号登录。",
        },
        customerIdentity: {
          unlinkedCustomer: "未关联客户账号",
        },
        admin: {
          agentTeamScheduleDeprecated: "产品组排班已改为成员可接单规则和请假；旧排班写入已下线",
        },
        push: {
          conversationChannelName: "会话消息",
          conversationChannelDescription: "设备咨询、在线接待与工程师回复",
        },
      },
    },
  },
  "en-US": {
    miscTailExtract: {
      dashboardMisc: {
        products: {
          productCenterTabsAria: "Product center sections",
        },
        skillDefinition: {
          columnSkill: "Skill",
          receptionConfig: "Reception agent",
        },
        knowledge: {
          tabsAriaFallback: "Knowledge base sections",
        },
        conversations: {
          filterAriaFallback: "Conversation filters",
        },
        conversationMonitor: {
          senderAi: "AI reception",
        },
        channels: {
          effectiveVersion: "{name} · Live #{version}",
          unpublished: "{name} · Unpublished",
          publishFlowMissingToast: "This reception agent has no published flow and cannot be bound to a channel",
          publishFlowMissingHint: "This reception agent has no published flow.",
          publishedBadge: "Published",
          effectiveVersionText: "Live version #{version}",
        },
      },
      uiShared: {
        commandSearch: "Search",
        sidebarNav: "Navigation",
        expandTags: "Expand tags",
        collapseTags: "Collapse tags",
        brandAlt: "Device After-Sales Platform",
      },
      libMisc: {
        enterpriseProducts: {
          productCatalogLoadFailed: "Failed to load the product catalog",
        },
        customerPortal: {
          formalAccountLoginRequired: "Please sign in with a formal user account.",
        },
        customerIdentity: {
          unlinkedCustomer: "No linked customer account",
        },
        admin: {
          agentTeamScheduleDeprecated: "Product team schedules now use member availability rules and leave; legacy schedule writes are disabled",
        },
        push: {
          conversationChannelName: "Conversation messages",
          conversationChannelDescription: "Device consultations, AI reception, and engineer replies",
        },
      },
    },
  },
  "es-ES": {
    miscTailExtract: {
      dashboardMisc: {
        products: {
          productCenterTabsAria: "Secciones del centro de productos",
        },
        skillDefinition: {
          columnSkill: "Habilidad",
          receptionConfig: "Agente de recepcion",
        },
        knowledge: {
          tabsAriaFallback: "Secciones de la base de conocimiento",
        },
        conversations: {
          filterAriaFallback: "Filtros de conversaciones",
        },
        conversationMonitor: {
          senderAi: "Recepcion IA",
        },
        channels: {
          effectiveVersion: "{name} - Activo #{version}",
          unpublished: "{name} - No publicado",
          publishFlowMissingToast: "Este agente de recepcion no tiene un flujo publicado y no puede vincularse a un canal",
          publishFlowMissingHint: "Este agente de recepcion no tiene un flujo publicado.",
          publishedBadge: "Publicado",
          effectiveVersionText: "Version activa #{version}",
        },
      },
      uiShared: {
        commandSearch: "Buscar",
        sidebarNav: "Navegacion",
        expandTags: "Expandir etiquetas",
        collapseTags: "Colapsar etiquetas",
        brandAlt: "Plataforma de posventa de dispositivos",
      },
      libMisc: {
        enterpriseProducts: {
          productCatalogLoadFailed: "No se pudo cargar el catalogo de productos",
        },
        customerPortal: {
          formalAccountLoginRequired: "Inicie sesion con una cuenta de usuario formal.",
        },
        customerIdentity: {
          unlinkedCustomer: "Sin cuenta de cliente vinculada",
        },
        admin: {
          agentTeamScheduleDeprecated: "Los horarios de grupo de producto ahora usan reglas de disponibilidad y permisos de los miembros; las escrituras de horarios antiguas estan deshabilitadas",
        },
        push: {
          conversationChannelName: "Mensajes de conversacion",
          conversationChannelDescription: "Consultas de dispositivos, recepcion IA y respuestas de ingenieros",
        },
      },
    },
  },
} satisfies ExtractedMessages
