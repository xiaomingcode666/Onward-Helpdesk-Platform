package builders

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/i18nx"
)

func TestBuildPlatformAuthRoleLocalizesBuiltinRoles(t *testing.T) {
	builtin := &models.AuthRole{Code: "service_engineer", IsBuiltin: true, Name: "服务工程师", Description: "处理分配给自己的会话、工单、诊断和视频协作"}
	english := BuildPlatformAuthRole(builtin, nil, i18nx.LocaleEnUS)
	if english.Name != "Service engineer" || english.Description != "Handles assigned conversations, tickets, diagnostics, and video collaboration" {
		t.Fatalf("english builtin role = %q / %q", english.Name, english.Description)
	}
	if chinese := BuildPlatformAuthRole(builtin, nil, i18nx.LocaleZhCN); chinese.Name != "服务工程师" {
		t.Fatalf("chinese builtin role = %q", chinese.Name)
	}

	custom := &models.AuthRole{Code: "night_shift", Name: "夜班组长", Description: "自定义"}
	if got := BuildPlatformAuthRole(custom, nil, i18nx.LocaleEnUS); got.Name != "夜班组长" || got.Description != "自定义" {
		t.Fatalf("custom role = %q / %q", got.Name, got.Description)
	}
	legacy := &models.AuthRole{Code: "super_admin", IsBuiltin: true, Name: "Super Admin"}
	if got := BuildPlatformAuthRole(legacy, nil, i18nx.LocaleEnUS); got.Name != "Super Admin" {
		t.Fatalf("builtin role without resources = %q", got.Name)
	}
	renamed := &models.AuthRole{Code: "service_engineer", IsBuiltin: true, Name: "Field engineer"}
	if got := BuildPlatformAuthRole(renamed, nil, i18nx.LocaleZhCN); got.Name != "Field engineer" {
		t.Fatalf("renamed builtin role = %q", got.Name)
	}
}

func TestBuildPlatformAuthRoleSkipsRolesWithoutResources(t *testing.T) {
	platform := &models.AuthRole{Code: "platform_admin", IsBuiltin: true, Name: "平台管理员", Description: "平台"}
	if got := BuildPlatformAuthRole(platform, nil, i18nx.LocaleEnUS); got.Name != "平台管理员" || got.Description != "平台" {
		t.Fatalf("platform role = %q / %q", got.Name, got.Description)
	}
}
