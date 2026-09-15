// Package ticketpolicy defines the versioned P1-P4 contract independently of storage.
package ticketpolicy

import (
	"fmt"
	"slices"
	"strings"
)

var Types = []string{"user_case", "incident", "major_incident", "problem", "known_error", "service_request"}

type Policy struct {
	Version  string            `json:"version"`
	Defaults map[string]string `json:"defaults"`
	Matrix   [][]string        `json:"matrix"`
}

func Default() Policy {
	return Policy{Version: "classification-20260914-v1", Defaults: map[string]string{"user_case": "p2", "incident": "p1", "major_incident": "p1", "problem": "p3", "known_error": "p3", "service_request": "p4"}, Matrix: [][]string{{"p1", "p2", "p2"}, {"p2", "p2", "p3"}, {"p3", "p3", "p4"}}}
}
func ValidPriority(v string) bool { return slices.Contains([]string{"p1", "p2", "p3", "p4"}, v) }
func (p Policy) Validate() error {
	if p.Version == "" || len(p.Version) > 80 || len(p.Defaults) != len(Types) || len(p.Matrix) != 3 {
		return fmt.Errorf("优先级规则必须包含版本、六类默认等级和 3×3 矩阵")
	}
	for _, t := range Types {
		if !ValidPriority(p.Defaults[t]) {
			return fmt.Errorf("工单分类 %s 缺少有效的 P1–P4 默认等级", t)
		}
	}
	for _, row := range p.Matrix {
		if len(row) != 3 {
			return fmt.Errorf("优先级矩阵必须为 3×3")
		}
		for _, v := range row {
			if !ValidPriority(v) {
				return fmt.Errorf("优先级矩阵只允许 P1–P4")
			}
		}
	}
	return nil
}

type Facts struct {
	Impact     string `json:"impact"`
	Urgency    string `json:"urgency"`
	Safety     string `json:"safety"`
	Reach      string `json:"reach"`
	Workaround string `json:"workaround"`
	Evidence   string `json:"evidence"`
	RootCause  string `json:"root_cause"`
}

func (f Facts) Validate() error {
	for k, v := range map[string]string{"impact": f.Impact, "urgency": f.Urgency, "reach": f.Reach} {
		if !slices.Contains([]string{"", "unknown", "high", "medium", "low"}, v) {
			return fmt.Errorf("%s 的评估值无效", k)
		}
	}
	if !slices.Contains([]string{"", "unknown", "critical", "none"}, f.Safety) || !slices.Contains([]string{"", "unknown", "none", "limited", "verified"}, f.Workaround) {
		return fmt.Errorf("安全风险或临时解决办法的评估值无效")
	}
	if len([]rune(f.Evidence)) > 2000 || len([]rune(f.RootCause)) > 2000 {
		return fmt.Errorf("评估依据或根因说明最多 2000 字")
	}
	if (f.Safety == "critical" || f.Workaround == "verified") && strings.TrimSpace(f.Evidence) == "" {
		return fmt.Errorf("请填写风险或临时解决办法的核对依据")
	}
	return nil
}

type Assessment struct {
	Priority    string `json:"priority"`
	Explanation string `json:"explanation"`
	Incomplete  bool   `json:"incomplete"`
}

func (p Policy) Assess(t string, f Facts) Assessment {
	a := Assessment{Priority: p.Defaults[t], Explanation: "按工单分类默认等级处理，待补充影响评估", Incomplete: true}
	complete := true
	for _, v := range []string{f.Impact, f.Urgency, f.Safety, f.Reach, f.Workaround} {
		if v == "" || v == "unknown" {
			complete = false
		}
	}
	if f.Safety == "critical" || (f.Impact == "high" && f.Reach == "high" && f.Workaround == "none") {
		return Assessment{"p1", "重大安全风险，或关键业务大范围中断且无有效替代办法", !complete}
	}
	if !complete {
		return a
	}
	levels := []string{"high", "medium", "low"}
	impact := min(slices.Index(levels, f.Impact), slices.Index(levels, f.Reach))
	// Verified workaround is evidence for the recorded residual impact, never an automatic downgrade.
	return Assessment{p.Matrix[impact][slices.Index(levels, f.Urgency)], "根据实际业务影响、服务覆盖范围及紧急程度矩阵计算；临时办法不自动降低等级", false}
}

// LegacyCode is the compatibility key for existing SLA profiles and dispatch sorting.
func LegacyCode(v string) string {
	return map[string]string{"p1": "p0", "p2": "p1", "p3": "p2", "p4": "p3"}[v]
}
func FromLegacy(v string) string {
	return map[string]string{"p0": "p1", "p1": "p2", "p2": "p3", "p3": "p4", "p4": "p4"}[v]
}
