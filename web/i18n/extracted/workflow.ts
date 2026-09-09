import type { ExtractedMessages } from "./types"

export const workflowMessages = {
  "zh-CN": {
    "workflowExtract": {
      "enterpriseWorkflowList": {
        "kind": {
          "platform": {
            "label": "平台内置",
            "detail": "企业可选、模板只读"
          },
          "tenant": {
            "label": "企业自定义",
            "detail": "企业内可编辑、可复用"
          }
        },
        "templateName": {
          "enterprisePrefix": "企业 ",
          "enterpriseStandard": "企业标准"
        },
        "capability": {
          "aiHuman": "自动接待 + 人工协同",
          "dispatchOnly": "功能派发",
          "deviceAi": "诊断服务",
          "basicAi": "基础问答"
        },
        "profile": {
          "knowledgeSupport": {
            "subtitle": "面向企业资料问答，知识库回答后可直接转接工程师。",
            "bestFor": "FAQ、技术文档、跨资料问答、人工协同",
            "impact": "覆盖企业知识检索、引用展示与人工接待"
          },
          "aiHuman": {
            "subtitle": "适合复杂设备维修，先排查问题，再交给工程师闭环。",
            "bestFor": "复杂故障、跨时区服务、需要工程师接管",
            "impact": "可覆盖诊断、工单、人工接待与视频协作"
          },
          "dispatchOnly": {
            "subtitle": "适合不做 AI 问诊，让客户直接选择转人工或创建工单。",
            "bestFor": "接单分派、工单闭环、人工接入",
            "impact": "客户看到动作菜单，系统按规则进入建单确认或人工接入"
          },
          "deviceAi": {
            "subtitle": "适合标准设备问题，客户绑定设备后自动获得排查建议。",
            "bestFor": "标准故障、产品知识充分、无需人工兜底",
            "impact": "可覆盖设备识别、知识检索与诊断回答"
          },
          "basicAi": {
            "subtitle": "适合常见问题和资料查询，保持轻量、稳定、低成本。",
            "bestFor": "FAQ、保养说明、资料查询",
            "impact": "可覆盖产品知识问答，不主动创建工单"
          },
          "path": {
            "scan": "扫码进入",
            "identifyDevice": "识别设备",
            "troubleshoot": "问题排查",
            "handoff": "转人工",
            "ticketVideo": "工单/视频",
            "customerEntry": "客户进入",
            "actionIdentify": "动作识别",
            "serviceMenu": "服务菜单",
            "ticketHuman": "建单/人工",
            "end": "结束",
            "searchKnowledge": "查知识库",
            "diagnosis": "给出诊断",
            "customerQuestion": "客户提问",
            "generateAnswer": "生成回答",
            "followUp": "客户追问"
          }
        },
        "launch": {
          "selectTemplate": "选择模板",
          "adjustFlow": "调整流程",
          "testRun": "试运行",
          "publishVersion": "发布版本",
          "enableProducts": "启用产品",
          "enableReception": "启用接待"
        },
        "governance": {
          "pendingPublish": {
            "label": "待发布",
            "detail": "未应用到产品"
          },
          "sourceUpdated": {
            "label": "来源有更新",
            "detail": "平台起始方案有新稳定版"
          },
          "upgradePending": {
            "label": "有待升级",
            "detail": "{count} 个产品配置仍选择旧版本"
          },
          "pendingPilot": {
            "label": "待试点",
            "detail": "尚无接待配置采用"
          },
          "pendingReview": {
            "label": "待评估",
            "detail": "暂无草稿引用"
          },
          "aligned": {
            "label": "配置一致",
            "detail": "接待配置均已选当前稳定版"
          }
        },
        "createIntro": {
          "title": "从扫码到工单的客户服务路径",
          "description": "客户扫码、问题排查、知识检索、转人工和工单闭环集中在一条路径；发布后再选择产品启用。",
          "knowledgeTitle": "从知识问答到人工协同的客户服务路径",
          "knowledgeDescription": "客户提问、企业知识检索、来源展示和转人工集中在一条路径；发布后由接待配置直接使用。",
          "customerEntry": "客户入口",
          "productKnowledge": "产品知识",
          "tenantKnowledge": "企业知识",
          "ticketClosure": "工单闭环",
          "video": "视频协作"
        },
        "tabs": {
          "templates": "工作流列表",
          "runs": "运行记录",
          "ariaLabel": "工作流列表分区"
        },
        "errors": {
          "summaryLoadFailed": "工作流模板摘要加载失败",
          "adoptionLoadFailed": "工作流应用状态加载失败",
          "noStableTemplate": "暂无可用稳定方案",
          "createFailed": "企业工作流创建失败"
        },
        "messages": {
          "created": "企业工作流草稿已创建"
        },
        "columns": {
          "workflow": "工作流",
          "source": "来源",
          "capability": "能力",
          "status": "状态",
          "adoption": "应用",
          "updatedAt": "更新时间",
          "actions": "操作"
        },
        "status": {
          "published": "已发布",
          "draft": "草稿"
        },
        "adoption": {
          "products": "{count} 个产品",
          "notApplied": "未应用",
          "pendingShort": "{count} 待更新",
          "pending": "{count} 个待更新"
        },
        "actions": {
          "viewPlan": "查看方案",
          "editFlow": "编辑流程",
          "createFromTemplate": "从模板新建",
          "cancel": "取消",
          "startConfigure": "开始配置"
        },
        "search": {
          "ariaLabel": "搜索工作流",
          "placeholder": "搜索工作流"
        },
        "listLabels": {
          "refresh": "刷新",
          "query": "查询",
          "loading": "工作流加载中",
          "empty": "暂无工作流",
          "loadFailed": "工作流加载失败"
        },
        "pageTitle": "客户服务工作流",
        "createModal": {
          "title": "新建企业工作流",
          "recommendedStart": "推荐起点",
          "canPilot": "可直接试点",
          "bestFor": "适合：{value}",
          "workflowName": "流程名称"
        }
      },
      "enterpriseWorkflowDetail": {
        "test": {
          "defaultMessage": "设备启动后显示故障代码，请给出排查步骤。"
        },
        "errors": {
          "versionLoadFailed": "工作流版本加载失败",
          "testConfigLoadFailed": "测试配置加载失败",
          "productAdoptionLoadFailed": "产品应用加载失败",
          "productAdoptionStatsLoadFailed": "产品应用统计加载失败",
          "blueprintLoadFailed": "服务方案加载失败",
          "templateLoadFailed": "工作流模板加载失败",
          "draftSaveFailed": "工作流草稿保存失败",
          "publishFailed": "工作流版本发布失败",
          "selectTestAgent": "请选择测试接待配置",
          "enterTestMessage": "请输入测试问题",
          "testFailed": "试运行失败",
          "workflowTestFailed": "工作流试运行失败",
          "scenarioFailed": "场景执行失败",
          "prepareUpgradeFailed": "准备工作流升级失败",
          "rollbackFailed": "工作流版本回滚失败",
          "archiveFailed": "工作流模板归档失败",
          "copyFailed": "复制企业模板失败"
        },
        "messages": {
          "draftSaved": "工作流草稿已保存",
          "publishNoChange": "当前内容与稳定版一致，未新建重复版本",
          "published": "稳定 V{version} 已发布，现有产品配置不会自动升级",
          "testCompleted": "试运行完成，服务路径已通过",
          "testInterrupted": "试运行停在确认节点",
          "batchFailed": "批量回归完成，{failed} 个场景失败",
          "batchPassed": "批量回归完成，{count} 个场景全部通过",
          "blueprintApplied": "已应用“{title}”，尚未保存",
          "upgradePrepared": "已选择 V{version}，线上版本保持不变",
          "rollbackCreated": "已从 V{fromVersion} 创建稳定 V{version}",
          "archived": "工作流模板已归档",
          "copied": "已创建企业配置草稿，可继续编排和发布",
          "copiedName": "{name} - 企业配置"
        },
        "confirm": {
          "applyBlueprintTitle": "切换为“{title}”",
          "applyToDraft": "应用到草稿",
          "prepareUpgradeTitle": "为“{name}”准备 V{version}",
          "prepareUpgrade": "准备升级",
          "rollbackTitle": "将 V{version} 恢复为新的 V{nextVersion}",
          "rollbackAndPublish": "确认回滚并发布",
          "archiveTitle": "归档工作流模板“{name}”",
          "archive": "确认归档"
        },
        "empty": {
          "notFound": "工作流模板不存在或无权访问"
        },
        "actions": {
          "backToList": "返回工作流列表",
          "testRun": "试运行",
          "refresh": "刷新",
          "copyAndConfigure": "复制并配置",
          "archive": "归档",
          "archiveDisabledTitle": "仍有产品使用此流程，不能归档",
          "archiveTitle": "归档企业模板",
          "inUse": "正在使用",
          "adjust": "调整",
          "adjusting": "正在调整",
          "enable": "启用",
          "replace": "替换"
        },
        "scope": {
          "platform": "平台标准",
          "tenant": "企业自定义"
        },
        "status": {
          "pendingReview": "待提交审核",
          "protected": "受保护",
          "notPublished": "尚未发布",
          "readOnly": "只读"
        },
        "version": {
          "stable": "稳定 V{version}"
        },
        "tabs": {
          "compose": "服务流程",
          "runs": "服务记录",
          "bindings": "知识配置",
          "versions": "发布版本",
          "adoption": "启用产品",
          "reception": "接待配置",
          "graph": "路径视图",
          "list": "清单视图",
          "ariaLabel": "工作流模板分区"
        },
        "loading": {
          "page": "工作流模板加载中",
          "status": "流程状态加载中",
          "compose": "服务流程加载中",
          "runs": "运行记录加载中",
          "bindings": "知识配置加载中",
          "versions": "版本记录加载中",
          "adoption": "产品应用加载中"
        },
        "graph": {
          "noNode": "暂无节点",
          "layer": "第 {layer} 层",
          "nodeResponsibility": "节点职责",
          "upstreamConnections": "上游连接",
          "downstreamPaths": "后续路径",
          "from": "来自",
          "next": "下一步",
          "servicePath": "服务路径",
          "nodeCount": "{count} 节点",
          "branchPointCount": "{count} 处分流",
          "controlAria": "路径视图控制",
          "focusEntry": "定位到入口",
          "fitAll": "查看全部步骤",
          "zoomOut": "缩小图谱",
          "zoomIn": "放大图谱",
          "zoomLabel": "当前缩放 {percent}%",
          "previewAria": "客服流程路径预览",
          "previewControlAria": "流程预览控制",
          "miniMap": "流程总览",
          "overview": "流程全貌",
          "overviewStats": "{nodes} 个节点 · {edges} 条连接",
          "openOverview": "打开流程全貌",
          "edgeCount": "{count} 连接",
          "viewAria": "流程全貌视图分区",
          "columns": {
            "order": "顺序",
            "node": "节点",
            "nodeType": "节点类型",
            "inputSource": "输入来源",
            "risk": "风险"
          },
          "source": {
            "upstream": "上游节点",
            "context": "会话上下文"
          },
          "risk": {
            "high": "高",
            "medium": "中",
            "low": "低"
          }
        },
        "runtime": {
          "knowledgeTitle": "产品知识随发布配置生效",
          "tenantKnowledgeTitle": "企业知识随发布配置生效",
          "knowledgeStage": "知识检索阶段",
          "knowledgeCalls": "处调用知识",
          "isolation": "资源隔离",
          "byProduct": "按产品",
          "byTenant": "按租户",
          "noKnowledgeBaseId": "不保存知识库 ID",
          "knowledgeVersion": "知识版本",
          "fixedOnPublish": "发布时固定",
          "traceableAnswers": "线上回答可追溯",
          "callPosition": "知识调用位置",
          "knowledgeNode": "知识检索 {index}",
          "emptyKnowledge": "当前流程未配置产品知识检索。",
          "emptyTenantKnowledge": "当前流程未配置企业知识检索。",
          "advancedConfig": "高级运行配置",
          "required": "必填"
        },
        "versionsPanel": {
          "currentStable": "当前稳定版本",
          "pendingPublish": "待发布",
          "stable": "稳定",
          "notOnline": "未上线",
          "steps": "流程步骤",
          "relativeTo": "相对 {version}",
          "initialVersion": "初始版本",
          "addedCapability": "新增能力",
          "removedCapability": "移除能力",
          "noAddedCapability": "没有新增业务能力",
          "noRemovedCapability": "没有移除业务能力",
          "diff": "版本差异",
          "compareAria": "对比历史版本",
          "compareVersion": "对比 V{version}",
          "selectHistory": "选择历史版本",
          "stepChanges": "流程步骤变化",
          "added": "新增",
          "changed": "调整",
          "removed": "移除",
          "addedPath": "新增路径",
          "removedPath": "移除路径",
          "noPathChange": "流程路径没有变化",
          "serviceCapability": "服务能力",
          "noCapabilityChange": "客户服务能力没有变化",
          "runtimeConfig": "运行配置",
          "credential": "费用归属",
          "addedConfig": "新增配置",
          "removedConfig": "移除配置",
          "noRuntimeChange": "费用与运行配置没有变化",
          "sameBehavior": "两个版本的业务行为与运行配置一致。",
          "currentStableBadge": "当前稳定",
          "historyVersion": "历史版本",
          "versionHash": "版本标识",
          "compare": "对比",
          "rollback": "回滚",
          "rollbackAsNew": "回滚为新版本",
          "empty": "暂无已发布版本",
          "deltaNone": "无变化",
          "deltaAdd": "增加 {count}",
          "deltaRemove": "减少 {count}"
        },
        "adoptionPanel": {
          "upgradeTitle": "版本升级",
          "reviewing": "审核中",
          "pendingReviewDeploy": "待审核部署",
          "pendingDeploy": "新配置待部署",
          "newVersion": "有新版本",
          "productionRunning": "生产运行中",
          "unboundProduct": "待绑定产品",
          "configVersion": "配置版本",
          "pendingSelect": "待选择",
          "canUpdateTo": "可更新到 V{version}",
          "prepareUpgrade": "准备升级",
          "configureAgent": "配置接待",
          "pendingDeployHint": "待审核部署。",
          "pendingFirstDeployHint": "待提交上线版本。",
          "empty": "暂无产品使用此流程",
          "emptyReception": "暂无接待配置使用此流程"
        },
        "testPanel": {
          "title": "试运行：{name}",
          "batchTitle": "批量回归测试集",
          "batchRunning": "批量执行中",
          "runScenarios": "运行 {count} 个场景",
          "totalItems": "共 {count} 项",
          "passedItems": "{count} 项通过",
          "failedItems": "{count} 项失败",
          "running": "执行中",
          "passed": "通过",
          "failed": "失败",
          "durationNodes": "耗时 {duration} ms · 经过 {nodes} 个节点",
          "stoppedAt": "停止位置：{node}",
          "hasHandoff": "此流程包含人工路径",
          "noHandoff": "此流程不包含人工节点",
          "chooseOther": "选择其他流程",
          "testAgent": "测试接待配置",
          "noAgent": "暂无可用接待配置。",
          "pageStatus": "第 {page} 页 / 共 {total} 页",
          "prevPage": "上一页",
          "nextPage": "下一页",
          "scenario": "验证场景",
          "customScenario": "自定义分支组合",
          "deviceBound": "模拟已绑定设备",
          "customerInput": "客户输入",
          "messagePlaceholder": "输入一条真实客户问题",
          "advancedBranch": "高级分支控制",
          "overrideCount": "{count} 项已指定",
          "actualCondition": "按实际条件",
          "onlyThisRun": "仅覆盖本次执行",
          "branchOverrideAria": "{node}分支覆盖",
          "actualConditionDecision": "按实际条件判断",
          "defaultBranch": "（默认分支）",
          "autoConfirm": "确认节点自动通过",
          "isolation": "测试隔离",
          "start": "开始试运行",
          "runningDraft": "正在执行当前草稿",
          "result": "执行结果",
          "technicalDetails": "查看技术详情",
          "duration": "总耗时",
          "visitedNodes": "经过节点",
          "modelAccount": "模型账号",
          "productKey": "产品专属 Key",
          "tenantKey": "租户默认 Key",
          "customModel": "自定义模型配置",
          "noUsageKey": "未关联平台用量 Key",
          "fallbackTenantKey": "已回退租户默认 Key",
          "executionChain": "执行链路",
          "errorDetails": "查看错误详情",
          "inputOutput": "查看输入与输出",
          "finalReply": "最终回复",
          "emptyResult": "暂无运行结果",
          "statusCompleted": "全部通过",
          "statusInterrupted": "等待确认",
          "statusFailed": "执行失败",
          "nodeRecovered": "异常已兜底",
          "nodeNotVisited": "本次未经过"
        },
        "compose": {
          "unsaved": "有未保存修改",
          "draftSaved": "草稿已保存，等待发布",
          "stableAligned": "已与稳定版本一致",
          "saveDraft": "保存草稿",
          "publishStable": "发布稳定版本",
          "staleNotice": "V{version} 已发布，仍有 {count} 个产品选择旧配置",
          "handleUpgrade": "处理版本升级",
          "readonlyPlatform": "平台标准流程为只读版本，需要复制后配置。"
        },
        "runsPanel": {
          "title": "线上服务记录",
          "scope": "验证范围",
          "currentAdoption": "当前流程产品应用"
        },
        "nodeLabels": {
          "types": {
            "start": "客户进入",
            "entry_context": "进入识别",
            "conversation_understanding": "问题理解",
            "service_access_policy": "服务进入策略",
            "reply_policy": "回复策略",
            "condition": "分支决策",
            "knowledge_retrieve": "产品知识",
            "tenant_knowledge_retrieve": "企业知识",
            "knowledge_merge": "知识合并",
            "answerability_gate": "判断决策",
            "llm_reply": "服务回复",
            "human_confirm": "客户确认",
            "prepare_ticket_draft": "工单草稿",
            "create_ticket": "创建工单",
            "create_video_meeting": "视频协作",
            "create_knowledge_candidate": "知识候选",
            "handoff_to_human": "转人工",
            "send_reply": "发送回复",
            "subflow": "子流程",
            "loop": "循环排障",
            "end": "结束",
            "unknown": "服务步骤"
          },
          "legend": {
            "reception": "接待与回复",
            "routing": "分支决策",
            "knowledge": "产品知识",
            "tenantKnowledge": "企业知识",
            "actions": "服务动作"
          },
          "subtitle": {
            "entry": "客户消息与服务上下文",
            "understand": "意图、风险与回答范围",
            "entryIdentify": "服务码与设备识别",
            "accessPolicy": "入口准入与服务策略",
            "replyPolicy": "回复、追问与工单策略",
            "condition": "按变量分支",
            "knowledgeRetrieve": "检索产品知识",
            "tenantKnowledgeRetrieve": "检索企业知识",
            "knowledgeMerge": "合并知识结果",
            "answerGate": "检查能否回答",
            "staticReply": "固定话术回复",
            "generateReply": "生成客户回复",
            "humanConfirm": "客户确认",
            "ticketDraft": "工单草稿",
            "createTicket": "创建工单并回写会话",
            "videoMeeting": "创建或复用视频协作",
            "knowledgeCandidate": "生成知识候选",
            "handoff": "维修团队与值班策略",
            "sendReply": "发送客户回复",
            "subflow": "不可变子流程",
            "loop": "循环排障",
            "end": "结束流程"
          },
          "chip": {
            "entry": "进入",
            "understand": "理解",
            "identify": "识别",
            "access": "准入",
            "policy": "策略",
            "condition": "分流",
            "knowledge": "知识",
            "merge": "合并",
            "gate": "判断",
            "staticReply": "话术",
            "reply": "回复",
            "confirm": "确认",
            "draft": "草稿",
            "ticket": "工单",
            "video": "视频",
            "candidate": "沉淀",
            "handoff": "人工",
            "output": "输出",
            "subflow": "子流程",
            "loop": "循环",
            "end": "结束",
            "unknown": "节点"
          },
          "risk": {
            "high": "高风险",
            "medium": "中风险",
            "low": "低风险"
          },
          "edge": {
            "onFailure": "失败时",
            "defaultBranch": "默认"
          },
          "branchName": "分支 {index}",
          "sanitize": {
            "rag": "知识",
            "llm": "回复",
            "intelligentDiagnosis": "设备问诊",
            "intelligentReception": "客服"
          }
        },
        "serviceModes": {
          "autoHuman": "自动接待 + 人工",
          "dispatchOnly": "功能派发",
          "auto": "自动接待"
        },
        "blueprint": {
          "sectionTitle": "服务方案配置",
          "current": "当前：{title}",
          "loading": "服务方案加载中",
          "noneAvailable": "暂无可用服务方案",
          "noOptions": "暂无方案",
          "labels": {
            "basic_ai": "轻量产品问答",
            "dispatch_only": "功能派发",
            "device_ai": "设备诊断服务",
            "ai_human": "诊断 + 人工协作"
          },
          "facts": {
            "basic_ai": [
              "标准问答",
              "不涉及设备",
              "不转人工"
            ],
            "dispatch_only": [
              "功能操作菜单",
              "无 AI 诊断",
              "工单确认"
            ],
            "device_ai": [
              "设备诊断",
              "无设备问答",
              "不转人工"
            ],
            "ai_human": [
              "识别设备",
              "AI 优先",
              "可转人工"
            ]
          },
          "currentStandard": "当前标准",
          "customizedCurrentType": "当前类型 · 已自定义",
          "restoreStandard": "恢复标准方案",
          "applyThis": "应用此方案"
        },
        "hero": {
          "cannotLaunch": "暂不可发布",
          "productsToUpgrade": "有产品待升级",
          "configurationsToUpgrade": "有接待配置待升级",
          "productionRunning": "已上线运行",
          "pilotReady": "可试点发布",
          "stageCount": "{count} 个服务阶段",
          "actionCount": "{count} 个服务动作",
          "currentVersion": "当前版本",
          "launchStatus": "发布状态"
        },
        "readiness": {
          "title": "上线准备度"
        },
        "releaseSettings": {
          "title": "发布与启用",
          "stableVersion": "稳定版本",
          "stableHint": "客户服务始终使用已发布的稳定版本。",
          "enabledProducts": "启用产品",
          "enabledProductsHint": "发布后，在对应产品应用中配置使用此流程的产品范围。",
          "productCount": "{count} 个产品",
          "enabledReception": "接待配置",
          "enabledReceptionHint": "发布后，由企业接待配置使用此流程。",
          "receptionCount": "{count} 个接待配置",
          "notEnabled": "未启用",
          "currentCoverage": "当前版本覆盖",
          "coverageHint": "已启用产品可逐步切换到最新稳定版本。",
          "receptionCoverageHint": "接待配置可切换到最新稳定版本。",
          "flowSource": "流程来源",
          "sourceHint": "平台标准流程可复制为自有企业配置。"
        },
        "credentialPolicy": {
          "noModelAccount": "此流程不依赖模型账号，固定回复与规则派发直接执行。",
          "notNeeded": "无需模型账号",
          "chargeOrder": "费用归属",
          "productThenTenant": "产品账号优先，企业账号兜底",
          "tenantThenProduct": "企业账号优先，产品账号兜底",
          "productOnly": "仅使用产品专属账号",
          "tenantOnly": "仅使用企业默认账号",
          "custom": "自定义归属",
          "save": "保存费用策略",
          "platformPreset": "平台预置"
        },
        "basicSettings": {
          "name": "流程名称",
          "description": "流程说明"
        },
        "orchestration": {
          "aria": "流程编排",
          "title": "流程编排",
          "hint": "左侧选择服务阶段，中间调整步骤与分支，右侧可启用服务动作或替换方案。",
          "stagesAria": "服务阶段",
          "totalStages": "{count} 阶段",
          "totalSteps": "{count} 步骤",
          "totalEdges": "{count} 连接",
          "editable": "可编排",
          "stageStepCount": "{count} 个步骤",
          "stageActionCount": "{count} 个动作",
          "stageBranchCount": "{count} 分支",
          "stageHintFallback": "按业务步骤配置客户服务路径。",
          "adjustPlan": "调整方案",
          "stepConfigTitle": "步骤配置",
          "stepName": "步骤名称",
          "inputSource": "输入来源",
          "customerPhrase": "客户话术",
          "branchTargets": "分支去向",
          "branchCountUnit": "{count} 条",
          "defaultPath": "默认路径",
          "conditionPath": "条件路径",
          "branchAria": "{name}去向",
          "unsetTarget": "未设置去向",
          "emptyStage": "此阶段暂无步骤，可从右侧服务方案中添加。",
          "emptyFlow": "此流程暂无步骤可编排。",
          "serviceActions": "服务动作",
          "servicePlans": "服务方案",
          "plansLoading": "方案加载中",
          "noReplacePlans": "暂无方案可替换",
          "summaryBranches": "{count} 条分支",
          "summaryInputs": "{count} 个输入",
          "staticScript": "固定话术",
          "hasFallback": "有兜底",
          "defaultPolicy": "按默认服务策略执行",
          "useSessionContext": "使用会话上下文",
          "pendingOrchestration": "尚未编排",
          "pendingActions": "待处理动作",
          "stageIntent": {
            "receive": "确认客户进入、设备绑定与会话上下文。",
            "understand": "评估问题类型、风险级别与下一步服务路径。",
            "knowledge": "在产品范围内检索知识，避免混产品回答。",
            "answer": "生成或复用客户可读的排障建议。",
            "service_action": "将确认后的诉求转为工单、人工或视频协作。",
            "respond": "发送客户回复，结束本轮服务。"
          },
          "knowledgeStageIntent": {
            "receive": "确认客户进入与会话上下文。",
            "knowledge": "检索企业通用知识库，确保回答有明确依据。"
          },
          "stageAction": {
            "receive": "调整进入",
            "understand": "调整评估",
            "knowledge": "替换知识方案",
            "answer": "调整回复",
            "service_action": "启用动作",
            "respond": "调整收尾"
          }
        },
        "capabilitySettings": {
          "title": "服务能力配置",
          "adjustByPlan": "由服务方案调整",
          "readOnlyConfig": "只读配置",
          "notConfigured": "未配置",
          "switchAriaOn": "{title}已开启",
          "switchAriaOff": "{title}未开启",
          "hint": "各能力由上方服务方案决定，切换方案后保存草稿并发布，产品启用后生效。",
          "rows": {
            "knowledge": {
              "title": "产品知识检索",
              "description": "客户提问后优先检索产品知识，回答有文档依据。"
            },
            "tenantKnowledge": {
              "title": "企业知识检索",
              "description": "客户提问后检索企业通用知识库，并展示回答所依据的来源。"
            },
            "ticket": {
              "title": "工单闭环",
              "description": "客户确认后生成售后工单，用于处理与跟踪。"
            },
            "handoff": {
              "title": "转接工程师",
              "description": "复杂问题可转给工程师继续跟进。"
            },
            "video": {
              "title": "远程视频协作",
              "description": "现场需要查看时，可发起远程视频协作。"
            },
            "learning": {
              "title": "知识沉淀",
              "description": "未被覆盖的问题可沉淀为后续知识候选。"
            }
          }
        }
      },
      "productUtils": {
        "credentialChain": {
          "product": "产品专属账号",
          "tenantDefault": "企业默认账号"
        },
        "runErrors": {
          "incompatibleVector": "知识索引与当前向量模型不兼容，请重新构建并发布知识索引。",
          "searchFailed": "产品知识暂时不可用，请检查知识索引服务或稍后重试。",
          "embeddingFailed": "知识检索所需的向量模型暂时不可用，请检查模型配置或稍后重试。",
          "modelTimeout": "模型响应超时，请稍后重试或检查模型服务状态。",
          "noCredential": "模型费用账号不可用，请检查产品专属账号和企业默认账号的开通状态。",
          "credentialConflict": "平台模型路由与当前费用归属策略冲突，请联系平台管理员检查模型接入设置。"
        },
        "journeyStages": {
          "receive": "接收客户问题",
          "understand": "理解并判断",
          "knowledge": "检索产品知识",
          "tenantKnowledge": "检索企业知识",
          "answer": "诊断与回答",
          "serviceAction": "执行服务动作",
          "respond": "回复并结束"
        },
        "scenarios": {
          "actual": "按实际条件运行",
          "quickAiQuestion": {
            "title": "未绑定设备的快速问答",
            "message": "这个产品应当如何进行日常维护？"
          },
          "boundDeviceDiagnosis": {
            "title": "绑定设备诊断",
            "message": "设备出现故障码 E01，请根据设备信息给出诊断步骤。"
          },
          "aiUnanswerableHandoff": {
            "title": "无法回答后转人工",
            "message": "设备故障仍未解决，请继续协助处理。"
          },
          "customerRequestsHandoff": {
            "title": "客户主动要求人工",
            "message": "请帮我转人工工程师处理。"
          },
          "confirmedTicket": {
            "title": "客户确认创建工单",
            "message": "设备复位后仍然异常，请帮我创建售后工单。"
          }
        },
        "capabilities": {
          "knowledge": {
            "title": "产品知识问答"
          },
          "tenantKnowledge": {
            "title": "企业知识问答"
          },
          "ticket": {
            "title": "创建售后工单"
          },
          "handoff": {
            "title": "转人工工程师"
          },
          "video": {
            "title": "远程视频协作"
          },
          "learning": {
            "title": "知识入库"
          },
          "stateEnabled": "已启用",
          "stateDisabled": "未编排",
          "stateProhibited": "明确禁止"
        },
        "serviceModes": {
          "dispatchOnly": "功能派发",
          "aiHuman": "自动诊断与人工协同",
          "knowledgeHuman": "知识问答与人工协同",
          "deviceDiagnosis": "诊断服务",
          "lightProductQa": "轻量产品问答"
        },
        "checks": {
          "structure": "客户路径完整",
          "dispatchCapability": "功能派发能力",
          "replyCapability": "生成回复能力",
          "dispatchScript": "派发话术",
          "knowledgeBasis": "产品知识依据",
          "tenantKnowledgeBasis": "企业知识依据",
          "version": "稳定版本",
          "adoption": "产品使用范围",
          "serviceConfig": "接待配置"
        }
      }
    }
  },
  "en-US": {
    "workflowExtract": {
      "enterpriseWorkflowList": {
        "kind": {
          "platform": {
            "label": "Platform built-in",
            "detail": "Selectable by enterprises; read-only template"
          },
          "tenant": {
            "label": "Enterprise custom",
            "detail": "Editable and reusable within the enterprise"
          }
        },
        "templateName": {
          "enterprisePrefix": "Enterprise ",
          "enterpriseStandard": "Enterprise standard"
        },
        "capability": {
          "aiHuman": "Automated reception + human collaboration",
          "dispatchOnly": "Action dispatch",
          "deviceAi": "Diagnostic service",
          "basicAi": "Basic Q&A"
        },
        "profile": {
          "knowledgeSupport": {
            "subtitle": "Designed for enterprise knowledge questions with direct engineer handoff when needed.",
            "bestFor": "FAQs, technical documents, cross-document questions, human collaboration",
            "impact": "Covers enterprise knowledge retrieval, source display, and human reception"
          },
          "aiHuman": {
            "subtitle": "Best for complex equipment repair: diagnose first, then hand off to engineers for closure.",
            "bestFor": "Complex faults, cross-time-zone service, engineer takeover",
            "impact": "Covers diagnosis, tickets, human reception, and video collaboration"
          },
          "dispatchOnly": {
            "subtitle": "Best when AI triage is not needed and customers directly choose human support or ticket creation.",
            "bestFor": "Assignment, ticket closure, human access",
            "impact": "Customers see an action menu, and the system routes to ticket confirmation or human reception"
          },
          "deviceAi": {
            "subtitle": "Best for standard device issues, giving guidance after the customer binds a device.",
            "bestFor": "Standard faults, strong product knowledge, no human fallback",
            "impact": "Covers device recognition, knowledge retrieval, and diagnostic answers"
          },
          "basicAi": {
            "subtitle": "Best for common questions and document lookup while staying lightweight, stable, and low cost.",
            "bestFor": "FAQ, maintenance instructions, document lookup",
            "impact": "Covers product knowledge Q&A without proactively creating tickets"
          },
          "path": {
            "scan": "Scan to enter",
            "identifyDevice": "Identify device",
            "troubleshoot": "Troubleshoot",
            "handoff": "Handoff",
            "ticketVideo": "Ticket/video",
            "customerEntry": "Customer enters",
            "actionIdentify": "Identify action",
            "serviceMenu": "Service menu",
            "ticketHuman": "Ticket/human",
            "end": "End",
            "searchKnowledge": "Search knowledge",
            "diagnosis": "Give diagnosis",
            "customerQuestion": "Customer asks",
            "generateAnswer": "Generate answer",
            "followUp": "Follow-up"
          }
        },
        "launch": {
          "selectTemplate": "Select template",
          "adjustFlow": "Adjust flow",
          "testRun": "Test run",
          "publishVersion": "Publish version",
          "enableProducts": "Enable products",
          "enableReception": "Enable reception"
        },
        "governance": {
          "pendingPublish": {
            "label": "Pending publish",
            "detail": "Not applied to products"
          },
          "sourceUpdated": {
            "label": "Source updated",
            "detail": "The platform starting plan has a new stable version"
          },
          "upgradePending": {
            "label": "Upgrade pending",
            "detail": "{count} product configurations still use an old version"
          },
          "pendingPilot": {
            "label": "Pending pilot",
            "detail": "No reception configuration has adopted it yet"
          },
          "pendingReview": {
            "label": "Pending review",
            "detail": "No draft references yet"
          },
          "aligned": {
            "label": "Aligned",
            "detail": "All reception configurations use the current stable version"
          }
        },
        "createIntro": {
          "title": "Customer service path from scan to ticket",
          "description": "Customer scan, troubleshooting, knowledge retrieval, human handoff, and ticket closure are kept in one path; after publishing, choose products to enable.",
          "knowledgeTitle": "Customer service path from knowledge Q&A to human collaboration",
          "knowledgeDescription": "Questions, enterprise knowledge retrieval, source display, and human handoff stay in one path used directly by the reception configuration.",
          "customerEntry": "Customer entry",
          "productKnowledge": "Product knowledge",
          "tenantKnowledge": "Enterprise knowledge",
          "ticketClosure": "Ticket closure",
          "video": "Video collaboration"
        },
        "tabs": {
          "templates": "Workflow list",
          "runs": "Run records",
          "ariaLabel": "Workflow list sections"
        },
        "errors": {
          "summaryLoadFailed": "Failed to load workflow template summary",
          "adoptionLoadFailed": "Failed to load workflow adoption status",
          "noStableTemplate": "No stable plan is available",
          "createFailed": "Failed to create enterprise workflow"
        },
        "messages": {
          "created": "Enterprise workflow draft created"
        },
        "columns": {
          "workflow": "Workflow",
          "source": "Source",
          "capability": "Capability",
          "status": "Status",
          "adoption": "Adoption",
          "updatedAt": "Updated at",
          "actions": "Actions"
        },
        "status": {
          "published": "Published",
          "draft": "Draft"
        },
        "adoption": {
          "products": "{count} products",
          "notApplied": "Not applied",
          "pendingShort": "{count} pending",
          "pending": "{count} pending updates"
        },
        "actions": {
          "viewPlan": "View plan",
          "editFlow": "Edit flow",
          "createFromTemplate": "Create from template",
          "cancel": "Cancel",
          "startConfigure": "Start configuration"
        },
        "search": {
          "ariaLabel": "Search workflows",
          "placeholder": "Search workflows"
        },
        "listLabels": {
          "refresh": "Refresh",
          "query": "Search",
          "loading": "Loading workflows",
          "empty": "No workflows",
          "loadFailed": "Failed to load workflows"
        },
        "pageTitle": "Customer Service Workflows",
        "createModal": {
          "title": "New Enterprise Workflow",
          "recommendedStart": "Recommended starting point",
          "canPilot": "Ready to pilot",
          "bestFor": "Best for: {value}",
          "workflowName": "Flow name"
        }
      },
      "enterpriseWorkflowDetail": {
        "test": {
          "defaultMessage": "The device shows a fault code after startup. Please provide troubleshooting steps."
        },
        "errors": {
          "versionLoadFailed": "Failed to load workflow versions",
          "testConfigLoadFailed": "Failed to load test configuration",
          "productAdoptionLoadFailed": "Failed to load product adoption",
          "productAdoptionStatsLoadFailed": "Failed to load product adoption statistics",
          "blueprintLoadFailed": "Failed to load service plans",
          "templateLoadFailed": "Failed to load workflow template",
          "draftSaveFailed": "Failed to save workflow draft",
          "publishFailed": "Failed to publish workflow version",
          "selectTestAgent": "Select a test reception configuration",
          "enterTestMessage": "Enter a test question",
          "testFailed": "Test run failed",
          "workflowTestFailed": "Workflow test run failed",
          "scenarioFailed": "Scenario execution failed",
          "prepareUpgradeFailed": "Failed to prepare workflow upgrade",
          "rollbackFailed": "Failed to roll back workflow version",
          "archiveFailed": "Failed to archive workflow template",
          "copyFailed": "Failed to copy enterprise template"
        },
        "messages": {
          "draftSaved": "Workflow draft saved",
          "publishNoChange": "Current content matches the stable version; no duplicate version was created",
          "published": "Stable V{version} published. Existing product configurations will not upgrade automatically",
          "testCompleted": "Test run completed; the service path passed",
          "testInterrupted": "Test run stopped at a confirmation node",
          "batchFailed": "Batch regression completed; {failed} scenarios failed",
          "batchPassed": "Batch regression completed; all {count} scenarios passed",
          "blueprintApplied": "Applied “{title}”; not saved yet",
          "upgradePrepared": "Selected V{version}; production version remains unchanged",
          "rollbackCreated": "Created stable V{version} from V{fromVersion}",
          "archived": "Workflow template archived",
          "copied": "Enterprise configuration draft created; continue orchestration and publishing",
          "copiedName": "{name} - Enterprise config"
        },
        "confirm": {
          "applyBlueprintTitle": "Switch to “{title}”",
          "applyToDraft": "Apply to draft",
          "prepareUpgradeTitle": "Prepare V{version} for “{name}”",
          "prepareUpgrade": "Prepare upgrade",
          "rollbackTitle": "Restore V{version} as new V{nextVersion}",
          "rollbackAndPublish": "Confirm rollback and publish",
          "archiveTitle": "Archive workflow template “{name}”",
          "archive": "Confirm archive"
        },
        "empty": {
          "notFound": "Workflow template does not exist or access is denied"
        },
        "actions": {
          "backToList": "Back to workflow list",
          "testRun": "Test run",
          "refresh": "Refresh",
          "copyAndConfigure": "Copy and configure",
          "archive": "Archive",
          "archiveDisabledTitle": "Products still use this flow, so it cannot be archived",
          "archiveTitle": "Archive enterprise template",
          "inUse": "In use",
          "adjust": "Adjust",
          "adjusting": "Adjusting",
          "enable": "Enable",
          "replace": "Replace"
        },
        "scope": {
          "platform": "Platform standard",
          "tenant": "Enterprise custom"
        },
        "status": {
          "pendingReview": "Pending review submission",
          "protected": "Protected",
          "notPublished": "Not published yet",
          "readOnly": "Read-only"
        },
        "version": {
          "stable": "Stable V{version}"
        },
        "tabs": {
          "compose": "Service flow",
          "runs": "Service records",
          "bindings": "Knowledge config",
          "versions": "Published versions",
          "adoption": "Enabled products",
          "reception": "Reception config",
          "graph": "Path view",
          "list": "List view",
          "ariaLabel": "Workflow template sections"
        },
        "loading": {
          "page": "Loading workflow template",
          "status": "Loading flow status",
          "compose": "Loading service flow",
          "runs": "Loading run records",
          "bindings": "Loading knowledge config",
          "versions": "Loading version records",
          "adoption": "Loading product applications"
        },
        "graph": {
          "noNode": "No node",
          "layer": "Layer {layer}",
          "nodeResponsibility": "Node responsibility",
          "upstreamConnections": "Upstream connections",
          "downstreamPaths": "Next paths",
          "from": "From",
          "next": "Next",
          "servicePath": "Service path",
          "nodeCount": "{count} nodes",
          "branchPointCount": "{count} branch points",
          "controlAria": "Path view controls",
          "focusEntry": "Focus entry",
          "fitAll": "View all steps",
          "zoomOut": "Zoom out graph",
          "zoomIn": "Zoom in graph",
          "zoomLabel": "Current zoom {percent}%",
          "previewAria": "Customer service path preview",
          "previewControlAria": "Flow preview controls",
          "miniMap": "Flow overview",
          "overview": "Flow overview",
          "overviewStats": "{nodes} nodes · {edges} connections",
          "openOverview": "Open flow overview",
          "edgeCount": "{count} connections",
          "viewAria": "Flow overview view sections",
          "columns": {
            "order": "Order",
            "node": "Node",
            "nodeType": "Node type",
            "inputSource": "Input source",
            "risk": "Risk"
          },
          "source": {
            "upstream": "Upstream node",
            "context": "Conversation context"
          },
          "risk": {
            "high": "High",
            "medium": "Medium",
            "low": "Low"
          }
        },
        "runtime": {
          "knowledgeTitle": "Product knowledge takes effect with published config",
          "tenantKnowledgeTitle": "Enterprise knowledge takes effect with published config",
          "knowledgeStage": "Knowledge retrieval stage",
          "knowledgeCalls": "knowledge calls",
          "isolation": "Resource isolation",
          "byProduct": "By product",
          "byTenant": "By tenant",
          "noKnowledgeBaseId": "Knowledge base IDs are not stored",
          "knowledgeVersion": "Knowledge version",
          "fixedOnPublish": "Fixed at publish",
          "traceableAnswers": "Online answers are traceable",
          "callPosition": "Knowledge call positions",
          "knowledgeNode": "Knowledge retrieval {index}",
          "emptyKnowledge": "No product knowledge retrieval is configured for this flow.",
          "emptyTenantKnowledge": "No enterprise knowledge retrieval is configured for this flow.",
          "advancedConfig": "Advanced runtime config",
          "required": "Required"
        },
        "versionsPanel": {
          "currentStable": "Current stable version",
          "pendingPublish": "Pending publish",
          "stable": "Stable",
          "notOnline": "Not online",
          "steps": "Flow steps",
          "relativeTo": "Compared with {version}",
          "initialVersion": "Initial version",
          "addedCapability": "Added capabilities",
          "removedCapability": "Removed capabilities",
          "noAddedCapability": "No business capabilities added",
          "noRemovedCapability": "No business capabilities removed",
          "diff": "Version diff",
          "compareAria": "Compare historical version",
          "compareVersion": "Compare V{version}",
          "selectHistory": "Select historical version",
          "stepChanges": "Flow step changes",
          "added": "Added",
          "changed": "Changed",
          "removed": "Removed",
          "addedPath": "Added path",
          "removedPath": "Removed path",
          "noPathChange": "Flow path did not change",
          "serviceCapability": "Service capability",
          "noCapabilityChange": "Customer service capability did not change",
          "runtimeConfig": "Runtime config",
          "credential": "Cost attribution",
          "addedConfig": "Added config",
          "removedConfig": "Removed config",
          "noRuntimeChange": "Cost and runtime config did not change",
          "sameBehavior": "The two versions have the same business behavior and runtime config.",
          "currentStableBadge": "Current stable",
          "historyVersion": "Historical version",
          "versionHash": "Version hash",
          "compare": "Compare",
          "rollback": "Roll back",
          "rollbackAsNew": "Roll back as new version",
          "empty": "No published versions",
          "deltaNone": "No change",
          "deltaAdd": "Added {count}",
          "deltaRemove": "Removed {count}"
        },
        "adoptionPanel": {
          "upgradeTitle": "Version upgrade",
          "reviewing": "In review",
          "pendingReviewDeploy": "Pending review deployment",
          "pendingDeploy": "New config pending deployment",
          "newVersion": "New version available",
          "productionRunning": "Running in production",
          "unboundProduct": "Product not bound",
          "configVersion": "Config version",
          "pendingSelect": "Pending selection",
          "canUpdateTo": "Can update to V{version}",
          "prepareUpgrade": "Prepare upgrade",
          "configureAgent": "Configure reception",
          "pendingDeployHint": "Pending review deployment.",
          "pendingFirstDeployHint": "Pending first online version submission.",
          "empty": "No products use this flow",
          "emptyReception": "No reception configuration uses this flow"
        },
        "testPanel": {
          "title": "Test run: {name}",
          "batchTitle": "Batch regression set",
          "batchRunning": "Batch running",
          "runScenarios": "Run {count} scenarios",
          "totalItems": "{count} total",
          "passedItems": "{count} passed",
          "failedItems": "{count} failed",
          "running": "Running",
          "passed": "Passed",
          "failed": "Failed",
          "durationNodes": "{duration} ms · {nodes} nodes",
          "stoppedAt": "Stopped at: {node}",
          "hasHandoff": "This flow includes a human path",
          "noHandoff": "This flow does not include a human node",
          "chooseOther": "Choose another flow",
          "testAgent": "Test reception config",
          "noAgent": "No reception configs available.",
          "pageStatus": "Page {page} / {total}",
          "prevPage": "Previous",
          "nextPage": "Next",
          "scenario": "Validation scenario",
          "customScenario": "Custom branch combination",
          "deviceBound": "Simulate bound device",
          "customerInput": "Customer input",
          "messagePlaceholder": "Enter a real customer question",
          "advancedBranch": "Advanced branch control",
          "overrideCount": "{count} specified",
          "actualCondition": "Use actual conditions",
          "onlyThisRun": "Only overrides this run",
          "branchOverrideAria": "{node} branch override",
          "actualConditionDecision": "Evaluate actual conditions",
          "defaultBranch": " (default branch)",
          "autoConfirm": "Auto-pass confirmation nodes",
          "isolation": "Test isolation",
          "start": "Start test run",
          "runningDraft": "Running current draft",
          "result": "Result",
          "technicalDetails": "View technical details",
          "duration": "Total duration",
          "visitedNodes": "Visited nodes",
          "modelAccount": "Model account",
          "productKey": "Product dedicated key",
          "tenantKey": "Tenant default key",
          "customModel": "Custom model config",
          "noUsageKey": "No platform usage key linked",
          "fallbackTenantKey": "Fell back to tenant default key",
          "executionChain": "Execution chain",
          "errorDetails": "View error details",
          "inputOutput": "View input and output",
          "finalReply": "Final reply",
          "emptyResult": "No run result",
          "statusCompleted": "All passed",
          "statusInterrupted": "Waiting for confirmation",
          "statusFailed": "Execution failed",
          "nodeRecovered": "Recovered by fallback",
          "nodeNotVisited": "Not visited this run"
        },
        "compose": {
          "unsaved": "Unsaved changes",
          "draftSaved": "Draft saved, pending publish",
          "stableAligned": "Aligned with stable version",
          "saveDraft": "Save draft",
          "publishStable": "Publish stable version",
          "staleNotice": "V{version} is published, but {count} products still choose an old config",
          "handleUpgrade": "Handle version upgrade",
          "readonlyPlatform": "Platform standard flows are read-only; copy before configuring."
        },
        "runsPanel": {
          "title": "Online service records",
          "scope": "Validation scope",
          "currentAdoption": "Current flow product adoption"
        },
        "nodeLabels": {
          "types": {
            "start": "Customer entry",
            "entry_context": "Entry recognition",
            "conversation_understanding": "Question understanding",
            "service_access_policy": "Service access policy",
            "reply_policy": "Reply policy",
            "condition": "Branch decision",
            "knowledge_retrieve": "Product knowledge",
            "tenant_knowledge_retrieve": "Enterprise knowledge",
            "knowledge_merge": "Knowledge merge",
            "answerability_gate": "Diagnosis decision",
            "llm_reply": "Service reply",
            "human_confirm": "Customer confirmation",
            "prepare_ticket_draft": "Ticket draft",
            "create_ticket": "Create ticket",
            "create_video_meeting": "Video collaboration",
            "create_knowledge_candidate": "Knowledge candidate",
            "handoff_to_human": "Handoff to human",
            "send_reply": "Send reply",
            "subflow": "Subflow",
            "loop": "Loop troubleshooting",
            "end": "End",
            "unknown": "Service step"
          },
          "legend": {
            "reception": "Reception and reply",
            "routing": "Branch decision",
            "knowledge": "Product knowledge",
            "tenantKnowledge": "Enterprise knowledge",
            "actions": "Service actions"
          },
          "subtitle": {
            "entry": "Customer message and service context",
            "understand": "Intent, risk, and answer scope",
            "entryIdentify": "Service code and device recognition",
            "accessPolicy": "Entry access and service policy",
            "replyPolicy": "Reply, follow-up, and ticket policy",
            "condition": "Branch by variables",
            "knowledgeRetrieve": "Retrieve product knowledge",
            "tenantKnowledgeRetrieve": "Retrieve enterprise knowledge",
            "knowledgeMerge": "Merge knowledge results",
            "answerGate": "Check whether an answer is available",
            "staticReply": "Fixed reply copy",
            "generateReply": "Generate customer reply",
            "humanConfirm": "Customer confirmation",
            "ticketDraft": "Ticket draft",
            "createTicket": "Create ticket and write back to conversation",
            "videoMeeting": "Create or reuse video collaboration",
            "knowledgeCandidate": "Generate knowledge candidate",
            "handoff": "Repair team and duty policy",
            "sendReply": "Send customer reply",
            "subflow": "Immutable subflow",
            "loop": "Loop troubleshooting",
            "end": "End flow"
          },
          "chip": {
            "entry": "Entry",
            "understand": "Understand",
            "identify": "Identify",
            "access": "Access",
            "policy": "Policy",
            "condition": "Route",
            "knowledge": "Knowledge",
            "merge": "Merge",
            "gate": "Gate",
            "staticReply": "Copy",
            "reply": "Reply",
            "confirm": "Confirm",
            "draft": "Draft",
            "ticket": "Ticket",
            "video": "Video",
            "candidate": "Collect",
            "handoff": "Human",
            "output": "Output",
            "subflow": "Subflow",
            "loop": "Loop",
            "end": "End",
            "unknown": "Node"
          },
          "risk": {
            "high": "High risk",
            "medium": "Medium risk",
            "low": "Low risk"
          },
          "edge": {
            "onFailure": "On failure",
            "defaultBranch": "Default"
          },
          "branchName": "Branch {index}",
          "sanitize": {
            "rag": "Knowledge",
            "llm": "Reply",
            "intelligentDiagnosis": "Device diagnosis",
            "intelligentReception": "Reception"
          }
        },
        "serviceModes": {
          "autoHuman": "Auto reception + human",
          "dispatchOnly": "Action dispatch",
          "auto": "Auto reception"
        },
        "blueprint": {
          "sectionTitle": "Service plan configuration",
          "current": "Current: {title}",
          "loading": "Loading service plans",
          "noneAvailable": "No service plan available",
          "noOptions": "No plans available",
          "labels": {
            "basic_ai": "Light product Q&A",
            "dispatch_only": "Action dispatch",
            "device_ai": "Device diagnostic service",
            "ai_human": "Diagnosis with human collaboration"
          },
          "facts": {
            "basic_ai": [
              "Standard Q&A",
              "No device context",
              "No handoff"
            ],
            "dispatch_only": [
              "Action menu",
              "No AI diagnosis",
              "Ticket confirmation"
            ],
            "device_ai": [
              "Device diagnosis",
              "No device Q&A",
              "No handoff"
            ],
            "ai_human": [
              "Device recognition",
              "Auto-first",
              "Human handoff possible"
            ]
          },
          "currentStandard": "Current standard",
          "customizedCurrentType": "Current type · customized",
          "restoreStandard": "Restore standard plan",
          "applyThis": "Apply this plan"
        },
        "hero": {
          "cannotLaunch": "Not ready to launch",
          "productsToUpgrade": "Products pending upgrade",
          "configurationsToUpgrade": "Reception configs pending upgrade",
          "productionRunning": "Running in production",
          "pilotReady": "Ready to pilot",
          "stageCount": "{count} service stages",
          "actionCount": "{count} service actions",
          "currentVersion": "Current version",
          "launchStatus": "Launch status"
        },
        "readiness": {
          "title": "Launch readiness"
        },
        "releaseSettings": {
          "title": "Publish and enable",
          "stableVersion": "Stable version",
          "stableHint": "Customer service always uses the published stable version.",
          "enabledProducts": "Enabled products",
          "enabledProductsHint": "After publishing, choose which products use this flow in the product apps.",
          "productCount": "{count} products",
          "enabledReception": "Reception config",
          "enabledReceptionHint": "After publishing, the enterprise reception configuration uses this flow.",
          "receptionCount": "{count} reception configs",
          "notEnabled": "Not enabled",
          "currentCoverage": "Current version coverage",
          "coverageHint": "Enabled products can gradually switch to the latest stable version.",
          "receptionCoverageHint": "Reception configurations can switch to the latest stable version.",
          "flowSource": "Flow source",
          "sourceHint": "Platform standard flows can be copied into your own enterprise configuration."
        },
        "credentialPolicy": {
          "noModelAccount": "This flow does not depend on a model account; fixed replies and rule-based dispatch run directly.",
          "notNeeded": "No model account needed",
          "chargeOrder": "Charge order",
          "productThenTenant": "Product account first, enterprise account as fallback",
          "tenantThenProduct": "Enterprise account first, product account as fallback",
          "productOnly": "Product-dedicated accounts only",
          "tenantOnly": "Enterprise default accounts only",
          "custom": "Custom charge attribution",
          "save": "Save charge policy",
          "platformPreset": "Platform preset"
        },
        "basicSettings": {
          "name": "Flow name",
          "description": "Flow description"
        },
        "orchestration": {
          "aria": "Flow orchestration",
          "title": "Flow orchestration",
          "hint": "Select a stage on the left, adjust steps and branches in the middle, and enable service actions or replace plans on the right.",
          "stagesAria": "Service stages",
          "totalStages": "{count} stages",
          "totalSteps": "{count} steps",
          "totalEdges": "{count} connections",
          "editable": "Editable",
          "stageStepCount": "{count} steps",
          "stageActionCount": "{count} actions",
          "stageBranchCount": "{count} branches",
          "stageHintFallback": "Configure the customer service path by business steps.",
          "adjustPlan": "Adjust plan",
          "stepConfigTitle": "Step config",
          "stepName": "Step name",
          "inputSource": "Input source",
          "customerPhrase": "Customer wording",
          "branchTargets": "Branch targets",
          "branchCountUnit": "{count} items",
          "defaultPath": "Default path",
          "conditionPath": "Conditional path",
          "branchAria": "{name} destination",
          "unsetTarget": "No target set",
          "emptyStage": "This stage has no steps yet; add them from the service plans on the right.",
          "emptyFlow": "This flow has no steps to orchestrate.",
          "serviceActions": "Service actions",
          "servicePlans": "Service plans",
          "plansLoading": "Loading plans",
          "noReplacePlans": "No replaceable plans",
          "summaryBranches": "{count} branches",
          "summaryInputs": "{count} inputs",
          "staticScript": "Fixed wording",
          "hasFallback": "Has fallback",
          "defaultPolicy": "Runs per the default service policy",
          "useSessionContext": "Uses conversation context",
          "pendingOrchestration": "Not orchestrated",
          "pendingActions": "Actions pending",
          "stageIntent": {
            "receive": "Confirm customer entry, device binding, and conversation context.",
            "understand": "Assess the problem type, risk level, and the next service path.",
            "knowledge": "Retrieve knowledge within the product scope to avoid mixing products.",
            "answer": "Generate or reuse troubleshooting suggestions customers can read.",
            "service_action": "Turn the confirmed request into a ticket, human, or video collaboration.",
            "respond": "Send the customer reply and finish this round of service."
          },
          "knowledgeStageIntent": {
            "receive": "Confirm customer entry and conversation context.",
            "knowledge": "Retrieve the enterprise knowledge base so every answer has a clear source."
          },
          "stageAction": {
            "receive": "Adjust entry",
            "understand": "Adjust assessment",
            "knowledge": "Replace knowledge plan",
            "answer": "Adjust reply",
            "service_action": "Enable actions",
            "respond": "Adjust completion"
          }
        },
        "capabilitySettings": {
          "title": "Service capability config",
          "adjustByPlan": "Adjusted by the service plan",
          "readOnlyConfig": "Read-only config",
          "notConfigured": "Not configured",
          "switchAriaOn": "{title} enabled",
          "switchAriaOff": "{title} disabled",
          "hint": "Each capability is decided by the service plan above; after switching plans, save the draft and publish, and it takes effect when products enable it.",
          "rows": {
            "knowledge": {
              "title": "Product knowledge retrieval",
              "description": "After the customer asks, product knowledge is searched first so answers are backed by documentation."
            },
            "tenantKnowledge": {
              "title": "Enterprise knowledge retrieval",
              "description": "Retrieve the enterprise knowledge base and show the sources used by the answer."
            },
            "ticket": {
              "title": "Ticket closure",
              "description": "After customer confirmation, an after-sales ticket is created for handling and tracking."
            },
            "handoff": {
              "title": "Handoff to engineer",
              "description": "Complex issues can be handed to engineers to continue."
            },
            "video": {
              "title": "Remote video collaboration",
              "description": "Remote video collaboration can start when the site needs inspection."
            },
            "learning": {
              "title": "Knowledge candidate",
              "description": "Questions not covered can accumulate as future knowledge candidates."
            }
          }
        }
      },
      "productUtils": {
        "credentialChain": {
          "product": "Product account",
          "tenantDefault": "Enterprise default account"
        },
        "runErrors": {
          "incompatibleVector": "The knowledge index is incompatible with the current vector model. Rebuild and publish the knowledge index.",
          "searchFailed": "Product knowledge is temporarily unavailable. Check the knowledge index service or try again later.",
          "embeddingFailed": "The vector model used for knowledge retrieval is temporarily unavailable. Check the model configuration or try again later.",
          "modelTimeout": "Model response timed out. Try again later or check the model service status.",
          "noCredential": "No model billing account is available. Check the provisioning of the product account and the enterprise default account.",
          "credentialConflict": "Platform model routing conflicts with the current billing attribution policy. Contact the platform administrator to check the model access settings."
        },
        "journeyStages": {
          "receive": "Receive customer question",
          "understand": "Understand and evaluate",
          "knowledge": "Retrieve product knowledge",
          "tenantKnowledge": "Retrieve enterprise knowledge",
          "answer": "Diagnose and answer",
          "serviceAction": "Perform service actions",
          "respond": "Reply and close"
        },
        "scenarios": {
          "actual": "Run with actual conditions",
          "quickAiQuestion": {
            "title": "Quick Q&A without a bound device",
            "message": "How should this product be maintained on a daily basis?"
          },
          "boundDeviceDiagnosis": {
            "title": "Diagnose a bound device",
            "message": "The device shows fault code E01. Please provide diagnostic steps based on the device information."
          },
          "aiUnanswerableHandoff": {
            "title": "Handoff after the AI cannot answer",
            "message": "The device fault is still unresolved. Please continue to assist."
          },
          "customerRequestsHandoff": {
            "title": "Customer requests human support",
            "message": "Please transfer me to a human engineer."
          },
          "confirmedTicket": {
            "title": "Customer confirms ticket creation",
            "message": "The device is still abnormal after a reset. Please create an after-sales ticket."
          }
        },
        "capabilities": {
          "knowledge": {
            "title": "Product knowledge Q&A"
          },
          "tenantKnowledge": {
            "title": "Enterprise knowledge Q&A"
          },
          "ticket": {
            "title": "Create after-sales tickets"
          },
          "handoff": {
            "title": "Handoff to human engineer"
          },
          "video": {
            "title": "Remote video collaboration"
          },
          "learning": {
            "title": "Knowledge accumulation"
          },
          "stateEnabled": "Enabled",
          "stateDisabled": "Not orchestrated",
          "stateProhibited": "Explicitly prohibited"
        },
        "serviceModes": {
          "dispatchOnly": "Action dispatch",
          "aiHuman": "Auto diagnosis with human collaboration",
          "knowledgeHuman": "Knowledge Q&A with human collaboration",
          "deviceDiagnosis": "Diagnostic service",
          "lightProductQa": "Light product Q&A"
        },
        "checks": {
          "structure": "Complete customer path",
          "dispatchCapability": "Action dispatch capability",
          "replyCapability": "Reply generation capability",
          "dispatchScript": "Dispatch wording",
          "knowledgeBasis": "Product knowledge basis",
          "tenantKnowledgeBasis": "Enterprise knowledge basis",
          "version": "Stable version",
          "adoption": "Product adoption scope",
          "serviceConfig": "Reception configuration"
        }
      }
    }
  },
  "es-ES": {
    "workflowExtract": {
      "enterpriseWorkflowList": {
        "kind": {
          "platform": {
            "label": "Integrado en la plataforma",
            "detail": "Seleccionable por empresas; plantilla de solo lectura"
          },
          "tenant": {
            "label": "Personalizado de empresa",
            "detail": "Editable y reutilizable dentro de la empresa"
          }
        },
        "templateName": {
          "enterprisePrefix": "Empresa ",
          "enterpriseStandard": "Estandar de empresa"
        },
        "capability": {
          "aiHuman": "Recepcion automatica + colaboracion humana",
          "dispatchOnly": "Despacho de acciones",
          "deviceAi": "Servicio de diagnostico",
          "basicAi": "Preguntas y respuestas basicas"
        },
        "profile": {
          "knowledgeSupport": {
            "subtitle": "Pensado para consultas de conocimiento empresarial con transferencia directa a ingenieria cuando sea necesario.",
            "bestFor": "FAQ, documentos tecnicos, preguntas entre documentos y colaboracion humana",
            "impact": "Cubre busqueda de conocimiento empresarial, fuentes y atencion humana"
          },
          "aiHuman": {
            "subtitle": "Adecuado para reparaciones complejas: primero diagnostica y luego pasa a ingenieria para cerrar el caso.",
            "bestFor": "Fallas complejas, servicio entre zonas horarias, toma por ingenieria",
            "impact": "Cubre diagnostico, tickets, recepcion humana y colaboracion por video"
          },
          "dispatchOnly": {
            "subtitle": "Adecuado cuando no se necesita triaje con IA y el cliente elige soporte humano o creacion de ticket.",
            "bestFor": "Asignacion, cierre de tickets, acceso humano",
            "impact": "El cliente ve un menu de acciones y el sistema enruta a confirmacion de ticket o recepcion humana"
          },
          "deviceAi": {
            "subtitle": "Adecuado para problemas estandar de equipos, con guia automatica tras vincular el dispositivo.",
            "bestFor": "Fallas estandar, conocimiento de producto suficiente, sin respaldo humano",
            "impact": "Cubre identificacion del dispositivo, busqueda de conocimiento y respuestas de diagnostico"
          },
          "basicAi": {
            "subtitle": "Adecuado para preguntas frecuentes y consulta de documentos, manteniendose ligero, estable y de bajo costo.",
            "bestFor": "FAQ, instrucciones de mantenimiento, consulta de documentos",
            "impact": "Cubre preguntas de conocimiento de producto sin crear tickets de forma proactiva"
          },
          "path": {
            "scan": "Escanear para entrar",
            "identifyDevice": "Identificar equipo",
            "troubleshoot": "Diagnosticar",
            "handoff": "Transferir",
            "ticketVideo": "Ticket/video",
            "customerEntry": "Entrada del cliente",
            "actionIdentify": "Identificar accion",
            "serviceMenu": "Menu de servicio",
            "ticketHuman": "Ticket/humano",
            "end": "Fin",
            "searchKnowledge": "Buscar conocimiento",
            "diagnosis": "Dar diagnostico",
            "customerQuestion": "Pregunta del cliente",
            "generateAnswer": "Generar respuesta",
            "followUp": "Seguimiento"
          }
        },
        "launch": {
          "selectTemplate": "Seleccionar plantilla",
          "adjustFlow": "Ajustar flujo",
          "testRun": "Prueba",
          "publishVersion": "Publicar version",
          "enableProducts": "Activar productos",
          "enableReception": "Activar recepcion"
        },
        "governance": {
          "pendingPublish": {
            "label": "Pendiente de publicar",
            "detail": "No aplicado a productos"
          },
          "sourceUpdated": {
            "label": "Origen actualizado",
            "detail": "El plan inicial de plataforma tiene una nueva version estable"
          },
          "upgradePending": {
            "label": "Actualizacion pendiente",
            "detail": "{count} configuraciones de producto aun usan una version anterior"
          },
          "pendingPilot": {
            "label": "Piloto pendiente",
            "detail": "Ninguna configuracion de recepcion lo ha adoptado"
          },
          "pendingReview": {
            "label": "Pendiente de evaluar",
            "detail": "Aun no hay referencias de borrador"
          },
          "aligned": {
            "label": "Alineado",
            "detail": "Todas las configuraciones usan la version estable actual"
          }
        },
        "createIntro": {
          "title": "Ruta de servicio del escaneo al ticket",
          "description": "Escaneo del cliente, diagnostico, busqueda de conocimiento, transferencia humana y cierre de ticket se concentran en una ruta; despues de publicar, elige los productos a activar.",
          "knowledgeTitle": "Ruta de servicio de preguntas a colaboracion humana",
          "knowledgeDescription": "Las preguntas, la busqueda de conocimiento empresarial, las fuentes y la transferencia humana se mantienen en una ruta usada por la configuracion de recepcion.",
          "customerEntry": "Entrada del cliente",
          "productKnowledge": "Conocimiento de producto",
          "tenantKnowledge": "Conocimiento empresarial",
          "ticketClosure": "Cierre de ticket",
          "video": "Colaboracion por video"
        },
        "tabs": {
          "templates": "Lista de flujos",
          "runs": "Registros de ejecucion",
          "ariaLabel": "Secciones de lista de flujos"
        },
        "errors": {
          "summaryLoadFailed": "Error al cargar el resumen de plantillas",
          "adoptionLoadFailed": "Error al cargar el estado de adopcion",
          "noStableTemplate": "No hay ningun plan estable disponible",
          "createFailed": "Error al crear el flujo de empresa"
        },
        "messages": {
          "created": "Borrador de flujo de empresa creado"
        },
        "columns": {
          "workflow": "Flujo",
          "source": "Origen",
          "capability": "Capacidad",
          "status": "Estado",
          "adoption": "Uso",
          "updatedAt": "Actualizado",
          "actions": "Acciones"
        },
        "status": {
          "published": "Publicado",
          "draft": "Borrador"
        },
        "adoption": {
          "products": "{count} productos",
          "notApplied": "No aplicado",
          "pendingShort": "{count} pendiente",
          "pending": "{count} actualizaciones pendientes"
        },
        "actions": {
          "viewPlan": "Ver plan",
          "editFlow": "Editar flujo",
          "createFromTemplate": "Crear desde plantilla",
          "cancel": "Cancelar",
          "startConfigure": "Iniciar configuracion"
        },
        "search": {
          "ariaLabel": "Buscar flujos",
          "placeholder": "Buscar flujos"
        },
        "listLabels": {
          "refresh": "Actualizar",
          "query": "Buscar",
          "loading": "Cargando flujos",
          "empty": "No hay flujos",
          "loadFailed": "Error al cargar flujos"
        },
        "pageTitle": "Flujos de Servicio al Cliente",
        "createModal": {
          "title": "Nuevo Flujo de Empresa",
          "recommendedStart": "Punto de partida recomendado",
          "canPilot": "Listo para piloto",
          "bestFor": "Adecuado para: {value}",
          "workflowName": "Nombre del flujo"
        }
      },
      "enterpriseWorkflowDetail": {
        "test": {
          "defaultMessage": "El equipo muestra un codigo de falla despues de iniciar. Indica los pasos de diagnostico."
        },
        "errors": {
          "versionLoadFailed": "Error al cargar versiones del flujo",
          "testConfigLoadFailed": "Error al cargar la configuracion de prueba",
          "productAdoptionLoadFailed": "Error al cargar el uso por productos",
          "productAdoptionStatsLoadFailed": "Error al cargar estadisticas de uso por productos",
          "blueprintLoadFailed": "Error al cargar planes de servicio",
          "templateLoadFailed": "Error al cargar la plantilla de flujo",
          "draftSaveFailed": "Error al guardar el borrador del flujo",
          "publishFailed": "Error al publicar la version del flujo",
          "selectTestAgent": "Selecciona una configuracion de recepcion para la prueba",
          "enterTestMessage": "Introduce una pregunta de prueba",
          "testFailed": "La prueba fallo",
          "workflowTestFailed": "Error al ejecutar la prueba del flujo",
          "scenarioFailed": "Error al ejecutar el escenario",
          "prepareUpgradeFailed": "Error al preparar la actualizacion del flujo",
          "rollbackFailed": "Error al revertir la version del flujo",
          "archiveFailed": "Error al archivar la plantilla de flujo",
          "copyFailed": "Error al copiar la plantilla de empresa"
        },
        "messages": {
          "draftSaved": "Borrador del flujo guardado",
          "publishNoChange": "El contenido actual coincide con la version estable; no se creo una version duplicada",
          "published": "Version estable V{version} publicada. Las configuraciones existentes no se actualizaran automaticamente",
          "testCompleted": "Prueba completada; la ruta de servicio paso",
          "testInterrupted": "La prueba se detuvo en un nodo de confirmacion",
          "batchFailed": "Regresion por lotes completada; fallaron {failed} escenarios",
          "batchPassed": "Regresion por lotes completada; pasaron los {count} escenarios",
          "blueprintApplied": "Se aplico “{title}”; aun no se ha guardado",
          "upgradePrepared": "V{version} seleccionada; la version de produccion queda igual",
          "rollbackCreated": "Creada la version estable V{version} desde V{fromVersion}",
          "archived": "Plantilla de flujo archivada",
          "copied": "Borrador de configuracion de empresa creado; continua con la orquestacion y publicacion",
          "copiedName": "{name} - Config. de empresa"
        },
        "confirm": {
          "applyBlueprintTitle": "Cambiar a “{title}”",
          "applyToDraft": "Aplicar al borrador",
          "prepareUpgradeTitle": "Preparar V{version} para “{name}”",
          "prepareUpgrade": "Preparar actualizacion",
          "rollbackTitle": "Restaurar V{version} como nueva V{nextVersion}",
          "rollbackAndPublish": "Confirmar reversion y publicar",
          "archiveTitle": "Archivar plantilla de flujo “{name}”",
          "archive": "Confirmar archivo"
        },
        "empty": {
          "notFound": "La plantilla de flujo no existe o no tienes acceso"
        },
        "actions": {
          "backToList": "Volver a la lista de flujos",
          "testRun": "Prueba",
          "refresh": "Actualizar",
          "copyAndConfigure": "Copiar y configurar",
          "archive": "Archivar",
          "archiveDisabledTitle": "Aun hay productos usando este flujo, no se puede archivar",
          "archiveTitle": "Archivar plantilla de empresa",
          "inUse": "En uso",
          "adjust": "Ajustar",
          "adjusting": "Ajustando",
          "enable": "Activar",
          "replace": "Reemplazar"
        },
        "scope": {
          "platform": "Estandar de plataforma",
          "tenant": "Personalizado de empresa"
        },
        "status": {
          "pendingReview": "Pendiente de enviar a revision",
          "protected": "Protegido",
          "notPublished": "Aun no publicado",
          "readOnly": "Solo lectura"
        },
        "version": {
          "stable": "Estable V{version}"
        },
        "tabs": {
          "compose": "Flujo de servicio",
          "runs": "Registros de servicio",
          "bindings": "Config. de conocimiento",
          "versions": "Versiones publicadas",
          "adoption": "Productos activados",
          "reception": "Config. de recepcion",
          "graph": "Vista de ruta",
          "list": "Vista de lista",
          "ariaLabel": "Secciones de plantilla de flujo"
        },
        "loading": {
          "page": "Cargando plantilla de flujo",
          "status": "Cargando estado del flujo",
          "compose": "Cargando flujo de servicio",
          "runs": "Cargando registros de ejecucion",
          "bindings": "Cargando configuracion de conocimiento",
          "versions": "Cargando registros de version",
          "adoption": "Cargando aplicaciones de producto"
        },
        "graph": {
          "noNode": "No hay nodo",
          "layer": "Capa {layer}",
          "nodeResponsibility": "Responsabilidad del nodo",
          "upstreamConnections": "Conexiones anteriores",
          "downstreamPaths": "Rutas siguientes",
          "from": "Desde",
          "next": "Siguiente",
          "servicePath": "Ruta de servicio",
          "nodeCount": "{count} nodos",
          "branchPointCount": "{count} puntos de rama",
          "controlAria": "Controles de vista de ruta",
          "focusEntry": "Ir a la entrada",
          "fitAll": "Ver todos los pasos",
          "zoomOut": "Alejar grafo",
          "zoomIn": "Acercar grafo",
          "zoomLabel": "Zoom actual {percent}%",
          "previewAria": "Vista previa de ruta de servicio al cliente",
          "previewControlAria": "Controles de vista previa del flujo",
          "miniMap": "Resumen del flujo",
          "overview": "Resumen del flujo",
          "overviewStats": "{nodes} nodos · {edges} conexiones",
          "openOverview": "Abrir resumen del flujo",
          "edgeCount": "{count} conexiones",
          "viewAria": "Secciones de vista de resumen del flujo",
          "columns": {
            "order": "Orden",
            "node": "Nodo",
            "nodeType": "Tipo de nodo",
            "inputSource": "Origen de entrada",
            "risk": "Riesgo"
          },
          "source": {
            "upstream": "Nodo anterior",
            "context": "Contexto de conversacion"
          },
          "risk": {
            "high": "Alto",
            "medium": "Medio",
            "low": "Bajo"
          }
        },
        "runtime": {
          "knowledgeTitle": "El conocimiento de producto entra en vigor con la configuracion publicada",
          "tenantKnowledgeTitle": "El conocimiento empresarial entra en vigor con la configuracion publicada",
          "knowledgeStage": "Etapa de busqueda de conocimiento",
          "knowledgeCalls": "llamadas de conocimiento",
          "isolation": "Aislamiento de recursos",
          "byProduct": "Por producto",
          "byTenant": "Por inquilino",
          "noKnowledgeBaseId": "No guarda ID de base de conocimiento",
          "knowledgeVersion": "Version de conocimiento",
          "fixedOnPublish": "Fijada al publicar",
          "traceableAnswers": "Respuestas online trazables",
          "callPosition": "Ubicaciones de llamada de conocimiento",
          "knowledgeNode": "Busqueda de conocimiento {index}",
          "emptyKnowledge": "Este flujo no tiene busqueda de conocimiento de producto configurada.",
          "emptyTenantKnowledge": "Este flujo no tiene busqueda de conocimiento empresarial configurada.",
          "advancedConfig": "Configuracion avanzada de ejecucion",
          "required": "Obligatorio"
        },
        "versionsPanel": {
          "currentStable": "Version estable actual",
          "pendingPublish": "Pendiente de publicar",
          "stable": "Estable",
          "notOnline": "No online",
          "steps": "Pasos del flujo",
          "relativeTo": "Comparado con {version}",
          "initialVersion": "Version inicial",
          "addedCapability": "Capacidades anadidas",
          "removedCapability": "Capacidades eliminadas",
          "noAddedCapability": "No se anadieron capacidades de negocio",
          "noRemovedCapability": "No se eliminaron capacidades de negocio",
          "diff": "Diferencia de versiones",
          "compareAria": "Comparar version historica",
          "compareVersion": "Comparar V{version}",
          "selectHistory": "Seleccionar version historica",
          "stepChanges": "Cambios de pasos del flujo",
          "added": "Anadido",
          "changed": "Ajustado",
          "removed": "Eliminado",
          "addedPath": "Ruta anadida",
          "removedPath": "Ruta eliminada",
          "noPathChange": "La ruta del flujo no cambio",
          "serviceCapability": "Capacidad de servicio",
          "noCapabilityChange": "La capacidad de servicio al cliente no cambio",
          "runtimeConfig": "Configuracion de ejecucion",
          "credential": "Asignacion de costos",
          "addedConfig": "Config. anadida",
          "removedConfig": "Config. eliminada",
          "noRuntimeChange": "La asignacion de costos y la configuracion de ejecucion no cambiaron",
          "sameBehavior": "Las dos versiones tienen el mismo comportamiento de negocio y configuracion de ejecucion.",
          "currentStableBadge": "Estable actual",
          "historyVersion": "Version historica",
          "versionHash": "Identificador de version",
          "compare": "Comparar",
          "rollback": "Revertir",
          "rollbackAsNew": "Revertir como nueva version",
          "empty": "No hay versiones publicadas",
          "deltaNone": "Sin cambios",
          "deltaAdd": "Aumenta {count}",
          "deltaRemove": "Reduce {count}"
        },
        "adoptionPanel": {
          "upgradeTitle": "Actualizacion de version",
          "reviewing": "En revision",
          "pendingReviewDeploy": "Despliegue pendiente de revision",
          "pendingDeploy": "Nueva config. pendiente de despliegue",
          "newVersion": "Nueva version disponible",
          "productionRunning": "En produccion",
          "unboundProduct": "Producto pendiente de vincular",
          "configVersion": "Version de configuracion",
          "pendingSelect": "Pendiente de seleccionar",
          "canUpdateTo": "Se puede actualizar a V{version}",
          "prepareUpgrade": "Preparar actualizacion",
          "configureAgent": "Configurar recepcion",
          "pendingDeployHint": "Despliegue pendiente de revision.",
          "pendingFirstDeployHint": "Pendiente de enviar la primera version online.",
          "empty": "Ningun producto usa este flujo",
          "emptyReception": "Ninguna configuracion de recepcion usa este flujo"
        },
        "testPanel": {
          "title": "Prueba: {name}",
          "batchTitle": "Conjunto de regresion por lotes",
          "batchRunning": "Lote en ejecucion",
          "runScenarios": "Ejecutar {count} escenarios",
          "totalItems": "{count} en total",
          "passedItems": "{count} aprobados",
          "failedItems": "{count} fallidos",
          "running": "Ejecutando",
          "passed": "Aprobado",
          "failed": "Fallido",
          "durationNodes": "{duration} ms · {nodes} nodos",
          "stoppedAt": "Detenido en: {node}",
          "hasHandoff": "Este flujo incluye una ruta humana",
          "noHandoff": "Este flujo no incluye nodo humano",
          "chooseOther": "Elegir otro flujo",
          "testAgent": "Configuracion de recepcion de prueba",
          "noAgent": "No hay configuraciones de recepcion disponibles.",
          "pageStatus": "Pagina {page} / {total}",
          "prevPage": "Anterior",
          "nextPage": "Siguiente",
          "scenario": "Escenario de validacion",
          "customScenario": "Combinacion de ramas personalizada",
          "deviceBound": "Simular equipo vinculado",
          "customerInput": "Entrada del cliente",
          "messagePlaceholder": "Introduce una pregunta real del cliente",
          "advancedBranch": "Control avanzado de ramas",
          "overrideCount": "{count} especificados",
          "actualCondition": "Usar condiciones reales",
          "onlyThisRun": "Solo anula esta ejecucion",
          "branchOverrideAria": "Anulacion de rama de {node}",
          "actualConditionDecision": "Evaluar condiciones reales",
          "defaultBranch": " (rama predeterminada)",
          "autoConfirm": "Aprobar automaticamente nodos de confirmacion",
          "isolation": "Aislamiento de prueba",
          "start": "Iniciar prueba",
          "runningDraft": "Ejecutando borrador actual",
          "result": "Resultado",
          "technicalDetails": "Ver detalles tecnicos",
          "duration": "Duracion total",
          "visitedNodes": "Nodos recorridos",
          "modelAccount": "Cuenta de modelo",
          "productKey": "Key dedicada de producto",
          "tenantKey": "Key predeterminada del tenant",
          "customModel": "Config. de modelo personalizada",
          "noUsageKey": "Sin Key de uso de plataforma vinculada",
          "fallbackTenantKey": "Volvio a la Key predeterminada del tenant",
          "executionChain": "Cadena de ejecucion",
          "errorDetails": "Ver detalles del error",
          "inputOutput": "Ver entrada y salida",
          "finalReply": "Respuesta final",
          "emptyResult": "No hay resultado de ejecucion",
          "statusCompleted": "Todo aprobado",
          "statusInterrupted": "Esperando confirmacion",
          "statusFailed": "Ejecucion fallida",
          "nodeRecovered": "Recuperado por respaldo",
          "nodeNotVisited": "No recorrido en esta ejecucion"
        },
        "compose": {
          "unsaved": "Cambios sin guardar",
          "draftSaved": "Borrador guardado, pendiente de publicar",
          "stableAligned": "Alineado con la version estable",
          "saveDraft": "Guardar borrador",
          "publishStable": "Publicar version estable",
          "staleNotice": "V{version} esta publicada, pero {count} productos aun eligen una configuracion anterior",
          "handleUpgrade": "Gestionar actualizacion",
          "readonlyPlatform": "Los flujos estandar de plataforma son de solo lectura; copia antes de configurar."
        },
        "runsPanel": {
          "title": "Registros de servicio online",
          "scope": "Alcance de validacion",
          "currentAdoption": "Uso de productos del flujo actual"
        },
        "nodeLabels": {
          "types": {
            "start": "Entrada del cliente",
            "entry_context": "Reconocimiento de entrada",
            "conversation_understanding": "Comprension de la pregunta",
            "service_access_policy": "Acceso al servicio",
            "reply_policy": "Politica de respuesta",
            "condition": "Decision de rama",
            "knowledge_retrieve": "Conocimiento de producto",
            "tenant_knowledge_retrieve": "Conocimiento empresarial",
            "knowledge_merge": "Fusion de conocimiento",
            "answerability_gate": "Decision de diagnostico",
            "llm_reply": "Respuesta de servicio",
            "human_confirm": "Confirmacion del cliente",
            "prepare_ticket_draft": "Borrador de ticket",
            "create_ticket": "Crear ticket",
            "create_video_meeting": "Colaboracion por video",
            "create_knowledge_candidate": "Candidato de conocimiento",
            "handoff_to_human": "Transferir a humano",
            "send_reply": "Enviar respuesta",
            "subflow": "Subflujo",
            "loop": "Diagnostico en bucle",
            "end": "Fin",
            "unknown": "Paso de servicio"
          },
          "legend": {
            "reception": "Recepcion y respuesta",
            "routing": "Decision de rama",
            "knowledge": "Conocimiento de producto",
            "tenantKnowledge": "Conocimiento empresarial",
            "actions": "Acciones de servicio"
          },
          "subtitle": {
            "entry": "Mensajes del cliente y contexto del servicio",
            "understand": "Intencion, riesgo y alcance de respuesta",
            "entryIdentify": "Reconocimiento de codigo de servicio y equipo",
            "accessPolicy": "Politicas de acceso y servicio de entrada",
            "replyPolicy": "Politicas de respuesta, seguimiento y creacion de ticket",
            "condition": "Rama de variables",
            "knowledgeRetrieve": "Buscar conocimiento de producto",
            "tenantKnowledgeRetrieve": "Buscar conocimiento empresarial",
            "knowledgeMerge": "Fusionar resultados de conocimiento",
            "answerGate": "Evaluar si hay respuesta disponible",
            "staticReply": "Texto fijo de respuesta",
            "generateReply": "Generar respuesta al cliente",
            "humanConfirm": "Confirmacion del cliente",
            "ticketDraft": "Borrador de ticket",
            "createTicket": "Crear ticket y escribir en la conversacion",
            "videoMeeting": "Crear o reutilizar colaboracion por video",
            "knowledgeCandidate": "Generar candidato de conocimiento",
            "handoff": "Estrategia de equipo y guardia",
            "sendReply": "Enviar respuesta al cliente",
            "subflow": "Subflujo inmutable",
            "loop": "Diagnostico en bucle",
            "end": "Finalizar flujo"
          },
          "chip": {
            "entry": "Entrada",
            "understand": "Comprension",
            "identify": "Reconocimiento",
            "access": "Acceso",
            "policy": "Politica",
            "condition": "Desvio",
            "knowledge": "Conocimiento",
            "merge": "Fusion",
            "gate": "Control",
            "staticReply": "Texto",
            "reply": "Respuesta",
            "confirm": "Confirmacion",
            "draft": "Borrador",
            "ticket": "Ticket",
            "video": "Video",
            "candidate": "Recoleccion",
            "handoff": "Humano",
            "output": "Salida",
            "subflow": "Subflujo",
            "loop": "Bucle",
            "end": "Fin",
            "unknown": "Nodo"
          },
          "risk": {
            "high": "Riesgo alto",
            "medium": "Riesgo medio",
            "low": "Riesgo bajo"
          },
          "edge": {
            "onFailure": "Al fallar",
            "defaultBranch": "Predeterminado"
          },
          "branchName": "Rama {index}",
          "sanitize": {
            "rag": "Conocimiento",
            "llm": "Respuesta",
            "intelligentDiagnosis": "Diagnostico de equipo",
            "intelligentReception": "Recepcion"
          }
        },
        "serviceModes": {
          "autoHuman": "Recepcion automatica + humano",
          "dispatchOnly": "Despacho de acciones",
          "auto": "Recepcion automatica"
        },
        "blueprint": {
          "sectionTitle": "Configuracion del plan de servicio",
          "current": "Actual: {title}",
          "loading": "Cargando planes de servicio",
          "noneAvailable": "No hay ningun plan de servicio disponible",
          "noOptions": "No hay planes",
          "labels": {
            "basic_ai": "Preguntas ligeras de producto",
            "dispatch_only": "Despacho de acciones",
            "device_ai": "Servicio de diagnostico de equipos",
            "ai_human": "Diagnostico con colaboracion humana"
          },
          "facts": {
            "basic_ai": [
              "Preguntas y respuestas estandar",
              "Sin contexto de equipo",
              "Sin transferencia"
            ],
            "dispatch_only": [
              "Menu de acciones",
              "Sin diagnostico IA",
              "Confirmacion de ticket"
            ],
            "device_ai": [
              "Diagnostico de equipo",
              "Sin preguntas sobre equipo",
              "Sin transferencia"
            ],
            "ai_human": [
              "Reconocimiento de equipo",
              "Automatico primero",
              "Transferencia posible"
            ]
          },
          "currentStandard": "Estandar actual",
          "customizedCurrentType": "Tipo actual · personalizado",
          "restoreStandard": "Restaurar plan estandar",
          "applyThis": "Aplicar este plan"
        },
        "hero": {
          "cannotLaunch": "Aun no se puede lanzar",
          "productsToUpgrade": "Productos pendientes de actualizar",
          "configurationsToUpgrade": "Configuraciones de recepcion pendientes de actualizar",
          "productionRunning": "En produccion",
          "pilotReady": "Listo para piloto",
          "stageCount": "{count} etapas de servicio",
          "actionCount": "{count} acciones de servicio",
          "currentVersion": "Version actual",
          "launchStatus": "Estado de lanzamiento"
        },
        "readiness": {
          "title": "Preparacion para lanzar"
        },
        "releaseSettings": {
          "title": "Publicar y activar",
          "stableVersion": "Version estable",
          "stableHint": "El servicio al cliente solo usa la version estable publicada.",
          "enabledProducts": "Productos activados",
          "enabledProductsHint": "Despues de publicar, elige en las aplicaciones de producto cuales usan este flujo.",
          "productCount": "{count} productos",
          "enabledReception": "Config. de recepcion",
          "enabledReceptionHint": "Despues de publicar, la configuracion de recepcion de la empresa usa este flujo.",
          "receptionCount": "{count} configuraciones de recepcion",
          "notEnabled": "No activado",
          "currentCoverage": "Cobertura de version actual",
          "coverageHint": "Los productos activados pueden cambiar gradualmente a la ultima version estable.",
          "receptionCoverageHint": "Las configuraciones de recepcion pueden cambiar a la ultima version estable.",
          "flowSource": "Origen del flujo",
          "sourceHint": "Los flujos estandar de plataforma se pueden copiar para formar tu propia configuracion de empresa."
        },
        "credentialPolicy": {
          "noModelAccount": "Este flujo no depende de una cuenta de modelo; las respuestas fijas y el despacho por reglas se ejecutan directamente.",
          "notNeeded": "Sin cuenta de modelo",
          "chargeOrder": "Orden de cobro",
          "productThenTenant": "Cuenta de producto primero, cuenta de empresa como respaldo",
          "tenantThenProduct": "Cuenta de empresa primero, cuenta de producto como respaldo",
          "productOnly": "Solo cuentas exclusivas de producto",
          "tenantOnly": "Solo cuentas predeterminadas de empresa",
          "custom": "Asignacion de costos personalizada",
          "save": "Guardar politica de cobro",
          "platformPreset": "Preajuste de plataforma"
        },
        "basicSettings": {
          "name": "Nombre del flujo",
          "description": "Descripcion del flujo"
        },
        "orchestration": {
          "aria": "Orquestacion del flujo",
          "title": "Orquestacion del flujo",
          "hint": "Selecciona una etapa a la izquierda, ajusta pasos y ramas en el centro, y activa acciones de servicio o reemplaza planes a la derecha.",
          "stagesAria": "Etapas de servicio",
          "totalStages": "{count} etapas",
          "totalSteps": "{count} pasos",
          "totalEdges": "{count} conexiones",
          "editable": "Editable",
          "stageStepCount": "{count} pasos",
          "stageActionCount": "{count} acciones",
          "stageBranchCount": "{count} ramas",
          "stageHintFallback": "Configura la ruta de servicio al cliente por pasos de negocio.",
          "adjustPlan": "Ajustar plan",
          "stepConfigTitle": "Config. de paso",
          "stepName": "Nombre del paso",
          "inputSource": "Origen de entrada",
          "customerPhrase": "Texto del cliente",
          "branchTargets": "Destinos de rama",
          "branchCountUnit": "{count} elementos",
          "defaultPath": "Ruta predeterminada",
          "conditionPath": "Ruta condicional",
          "branchAria": "Destino de {name}",
          "unsetTarget": "Sin destino configurado",
          "emptyStage": "Esta etapa aun no tiene pasos; anadelos desde los planes de servicio a la derecha.",
          "emptyFlow": "Este flujo no tiene pasos que orquestar.",
          "serviceActions": "Acciones de servicio",
          "servicePlans": "Planes de servicio",
          "plansLoading": "Cargando planes",
          "noReplacePlans": "No hay planes reemplazables",
          "summaryBranches": "{count} ramas",
          "summaryInputs": "{count} entradas",
          "staticScript": "Texto fijo",
          "hasFallback": "Con respaldo",
          "defaultPolicy": "Ejecuta segun la politica de servicio predeterminada",
          "useSessionContext": "Usa contexto de conversacion",
          "pendingOrchestration": "Pendiente de orquestar",
          "pendingActions": "Acciones pendientes",
          "stageIntent": {
            "receive": "Confirma la entrada del cliente, la vinculacion del equipo y el contexto de la conversacion.",
            "understand": "Evalua el tipo de problema, el nivel de riesgo y la siguiente ruta de servicio.",
            "knowledge": "Busca conocimiento segun el alcance del producto para evitar mezclar productos.",
            "answer": "Genera o reutiliza sugerencias de diagnostico legibles para el cliente.",
            "service_action": "Convierte la solicitud confirmada en ticket, humano o video.",
            "respond": "Envia la respuesta al cliente y finaliza esta ronda de servicio."
          },
          "knowledgeStageIntent": {
            "receive": "Confirma la entrada del cliente y el contexto de la conversacion.",
            "knowledge": "Busca en la base de conocimiento empresarial para que cada respuesta tenga una fuente clara."
          },
          "stageAction": {
            "receive": "Ajustar entrada",
            "understand": "Ajustar evaluacion",
            "knowledge": "Reemplazar plan de conocimiento",
            "answer": "Ajustar respuesta",
            "service_action": "Activar acciones",
            "respond": "Ajustar finalizacion"
          }
        },
        "capabilitySettings": {
          "title": "Config. de capacidades de servicio",
          "adjustByPlan": "Ajustado por el plan de servicio",
          "readOnlyConfig": "Config. de solo lectura",
          "notConfigured": "No configurado",
          "switchAriaOn": "{title} activado",
          "switchAriaOff": "{title} desactivado",
          "hint": "Cada capacidad la decide el plan de servicio superior; despues de cambiar de plan, guarda el borrador y publica, y surte efecto cuando los productos lo activen.",
          "rows": {
            "knowledge": {
              "title": "Busqueda de conocimiento de producto",
              "description": "Tras la pregunta del cliente, se busca conocimiento de producto primero para que las respuestas tengan respaldo documental."
            },
            "tenantKnowledge": {
              "title": "Busqueda de conocimiento empresarial",
              "description": "Busca en la base de conocimiento empresarial y muestra las fuentes usadas por la respuesta."
            },
            "ticket": {
              "title": "Cierre de ticket",
              "description": "Tras la confirmacion del cliente, se crea un ticket de posventa para gestion y seguimiento."
            },
            "handoff": {
              "title": "Transferir a ingeniero",
              "description": "Los problemas complejos pueden pasar a ingenieros para continuar."
            },
            "video": {
              "title": "Colaboracion remota por video",
              "description": "Se puede iniciar colaboracion remota por video cuando se necesita ver el sitio."
            },
            "learning": {
              "title": "Recoleccion de conocimiento",
              "description": "Las preguntas no cubiertas se pueden acumular como futuros candidatos de conocimiento."
            }
          }
        }
      },
      "productUtils": {
        "credentialChain": {
          "product": "Cuenta de producto",
          "tenantDefault": "Cuenta predeterminada de empresa"
        },
        "runErrors": {
          "incompatibleVector": "El indice de conocimiento es incompatible con el modelo vectorial actual. Reconstruye y publica el indice de conocimiento.",
          "searchFailed": "El conocimiento de producto no esta disponible temporalmente. Comprueba el servicio de indice de conocimiento o intenta de nuevo mas tarde.",
          "embeddingFailed": "El modelo vectorial usado para la busqueda de conocimiento no esta disponible temporalmente. Comprueba la configuracion del modelo o intenta de nuevo mas tarde.",
          "modelTimeout": "La respuesta del modelo agoto el tiempo de espera. Intenta de nuevo mas tarde o comprueba el estado del servicio del modelo.",
          "noCredential": "No hay ninguna cuenta de cobro de modelo disponible. Comprueba el estado de activacion de la cuenta de producto y de la cuenta predeterminada de empresa.",
          "credentialConflict": "El enrutamiento de modelo de plataforma entra en conflicto con la politica de asignacion de costos actual. Contacta con el administrador de plataforma para revisar la configuracion de acceso al modelo."
        },
        "journeyStages": {
          "receive": "Recibir pregunta del cliente",
          "understand": "Entender y evaluar",
          "knowledge": "Buscar conocimiento de producto",
          "tenantKnowledge": "Buscar conocimiento empresarial",
          "answer": "Diagnosticar y responder",
          "serviceAction": "Ejecutar acciones de servicio",
          "respond": "Responder y finalizar"
        },
        "scenarios": {
          "actual": "Ejecutar con condiciones reales",
          "quickAiQuestion": {
            "title": "Preguntas rapidas sin equipo vinculado",
            "message": "Como se debe realizar el mantenimiento diario de este producto?"
          },
          "boundDeviceDiagnosis": {
            "title": "Diagnosticar equipo vinculado",
            "message": "El equipo muestra el codigo de falla E01. Proporciona pasos de diagnostico segun la informacion del equipo."
          },
          "aiUnanswerableHandoff": {
            "title": "Transferir cuando la IA no puede responder",
            "message": "La falla del equipo sigue sin resolverse. Continua ayudando por favor."
          },
          "customerRequestsHandoff": {
            "title": "El cliente solicita soporte humano",
            "message": "Por favor transfereme a un ingeniero humano."
          },
          "confirmedTicket": {
            "title": "El cliente confirma la creacion del ticket",
            "message": "El equipo sigue con fallas despues del reinicio. Por favor crea un ticket de posventa."
          }
        },
        "capabilities": {
          "knowledge": {
            "title": "Preguntas y respuestas de conocimiento de producto"
          },
          "tenantKnowledge": {
            "title": "Preguntas y respuestas de conocimiento empresarial"
          },
          "ticket": {
            "title": "Crear tickets de posventa"
          },
          "handoff": {
            "title": "Transferir a ingeniero humano"
          },
          "video": {
            "title": "Colaboracion remota por video"
          },
          "learning": {
            "title": "Acumulacion de conocimiento"
          },
          "stateEnabled": "Activado",
          "stateDisabled": "No orquestado",
          "stateProhibited": "Prohibido explicitamente"
        },
        "serviceModes": {
          "dispatchOnly": "Despacho de acciones",
          "aiHuman": "Diagnostico automatico con colaboracion humana",
          "knowledgeHuman": "Preguntas de conocimiento con colaboracion humana",
          "deviceDiagnosis": "Servicio de diagnostico",
          "lightProductQa": "Preguntas ligeras de producto"
        },
        "checks": {
          "structure": "Ruta de cliente completa",
          "dispatchCapability": "Capacidad de despacho de acciones",
          "replyCapability": "Capacidad de generacion de respuestas",
          "dispatchScript": "Texto de despacho",
          "knowledgeBasis": "Base de conocimiento de producto",
          "tenantKnowledgeBasis": "Base de conocimiento empresarial",
          "version": "Version estable",
          "adoption": "Alcance de uso del producto",
          "serviceConfig": "Configuracion de recepcion"
        }
      }
    }
  },
} satisfies ExtractedMessages
