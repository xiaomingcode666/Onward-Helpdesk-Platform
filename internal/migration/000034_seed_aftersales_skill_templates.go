package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const builtinSkillTemplateRemarkPrefix = "builtin-aftersales-template:"

type builtinSkillTemplate struct {
	Code          string
	Name          string
	Description   string
	Instruction   string
	Examples      string
	ToolWhitelist string
}

func init() {
	register(34, "seed production after-sales skill templates", func() error {
		return seedAfterSalesSkillTemplates(sqls.DB())
	})
}

func seedAfterSalesSkillTemplates(db *gorm.DB) error {
	now := time.Now()
	for _, template := range afterSalesSkillTemplates() {
		remark := builtinSkillTemplateRemarkPrefix + template.Code
		var count int64
		if err := db.Model(&models.SkillDefinition{}).
			Where("tenant_id = ? AND remark = ?", 0, remark).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		item := &models.SkillDefinition{
			TenantID:      0,
			Name:          template.Name,
			Description:   template.Description,
			Instruction:   template.Instruction,
			Examples:      template.Examples,
			ToolWhitelist: template.ToolWhitelist,
			Status:        enums.StatusOk,
			Remark:        remark,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserName: "system",
				UpdatedAt:      now,
				UpdateUserName: "system",
			},
		}
		if err := db.Create(item).Error; err != nil {
			return err
		}
	}
	return nil
}

func afterSalesSkillTemplates() []builtinSkillTemplate {
	return []builtinSkillTemplate{
		{
			Code:          "fault-diagnosis",
			Name:          "设备故障诊断",
			Description:   "基于产品、设备、故障码和知识证据完成结构化问诊，不执行业务副作用。",
			Instruction:   "使用客户当前语言完成设备故障问诊。先确认产品、设备型号、故障现象、故障码、发生时间和已尝试操作，再检索产品知识并区分事实、推断与待确认信息。没有可靠依据时不得编造结论；涉及人身、设备或环境风险时立即停止给出继续操作建议，并切换到安全升级技能。",
			Examples:      `["设备启动后压力一直波动","控制器显示 E37 是什么问题","机器运行十分钟后自动停机"]`,
			ToolWhitelist: `[]`,
		},
		{
			Code:          "safe-stop-escalation",
			Name:          "安全停机与人工升级",
			Description:   "识别安全风险、重复失败和明确人工诉求，输出停机建议与人工升级信号。",
			Instruction:   "识别人身伤害、冒烟、异味、异常高温、漏电、泄漏、失控运动以及可能扩大设备损坏的风险。命中风险时先给出清晰的停机、断能和现场隔离建议，禁止继续远程试错。客户明确要求人工、连续多轮无效或知识依据不足时，清楚说明升级原因并输出人工升级信号；转人工动作由会话主流程幂等执行，不要自行声称已经转接。",
			Examples:      `["设备有焦糊味还可以继续运行吗","已经按你的方法试了三次还是不行","请马上转给维修工程师"]`,
			ToolWhitelist: `[]`,
		},
		{
			Code:          "operation-guidance",
			Name:          "操作与维修指导",
			Description:   "依据已发布产品知识提供可验证的操作步骤、前置条件和停止条件。",
			Instruction:   "仅依据当前产品已发布知识给出操作或维修指导。步骤必须包含适用型号、前置条件、安全措施、操作顺序、预期结果和停止条件。一次只推进一个可验证阶段，并在继续前确认客户反馈；知识冲突、版本不明或缺少关键安全条件时停止指导并建议升级人工。",
			Examples:      `["怎么复位这个故障码","如何更换滤芯","保养周期和操作步骤是什么"]`,
			ToolWhitelist: `[]`,
		},
		{
			Code:          "service-ticket",
			Name:          "售后工单建单",
			Description:   "整理故障上下文和客户诉求，输出结构化建单草稿。",
			Instruction:   "当客户明确要求报修、投诉或持续跟进时，整理产品、设备、故障现象、影响范围、紧急程度、已尝试操作和联系方式。信息不足时先追问；信息完整后输出工单草稿与建单意图。客户确认和幂等创建由会话主流程执行，禁止自行声称已经建单。",
			Examples:      `["帮我报修这台设备","这个问题一直没解决，请创建工单","我要提交一次设备故障投诉"]`,
			ToolWhitelist: `[]`,
		},
	}
}
