package middleware

import (
	"strconv"

	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DataScopeMiddleware resolves the current user's data scope rules,
// 并将规则注入到 Gin Context 中。
//
// 执行顺序：必须在 TenantContextMiddleware 之后注册。
func DataScopeMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal := GetAuthPrincipal(ctx)
		if principal == nil {
			ctx.Next()
			return
		}

		// 平台用户拥有全量数据范围，无需额外过滤
		if principal.IsPlatform() {
			principal.DataScopes = []DataScope{
				{
					SubjectType: "user",
					SubjectID:   principal.UserID,
					ScopeType:   "all_tenant",
					ScopeValues: []string{"*"},
					Include:     true,
				},
			}
			ctx.Next()
			return
		}

		// 从角色推断数据范围
		// 当前为静态规则实现，后续可改为从 data_scope_policies 表查询
		principal.DataScopes = resolveDataScopes(principal)

		ctx.Next()
	}
}

// resolveDataScopes 根据用户角色解析数据范围。
// 当前为静态实现，后续可改为从 data_scope_policies 表动态加载。
func resolveDataScopes(principal *AuthPrincipal) []DataScope {
	if principal == nil {
		return nil
	}

	// 企业管理员：全企业范围
	if principal.HasRole("tenant_admin") {
		return []DataScope{
			{
				SubjectType: "role",
				SubjectID:   principal.UserID,
				ScopeType:   "all_tenant",
				ScopeValues: []string{strconv.FormatInt(principal.TenantID, 10)},
				Include:     true,
			},
		}
	}

	// 根据角色组合默认范围
	scopes := make([]DataScope, 0)

	// 工程师/坐席：指派工单
	if principal.HasRole("engineer") || principal.HasRole("agent") {
		scopes = append(scopes, DataScope{
			SubjectType: "role",
			SubjectID:   principal.UserID,
			ScopeType:   "assigned_ticket",
			Include:     true,
		})
		scopes = append(scopes, DataScope{
			SubjectType: "role",
			SubjectID:   principal.UserID,
			ScopeType:   "self",
			Include:     true,
		})
	}

	// 客户用户：仅自己的设备和工单
	if principal.IsCustomer() {
		scopes = append(scopes, DataScope{
			SubjectType: "user",
			SubjectID:   principal.UserID,
			ScopeType:   "self",
			ScopeValues: []string{"device", "ticket"},
			Include:     true,
		})
	}

	return scopes
}

// ApplyDataScope 根据 AuthPrincipal 的数据范围向 GORM 查询添加过滤条件。
//
// 用法示例：
//
//	var tickets []models.Ticket
//	principal := middleware.GetAuthPrincipal(c)
//	db := middleware.ApplyDataScope(sqls.DB(), principal)
//	db.Where("tenant_id = ?", principal.TenantID).Find(&tickets)
//
// 该函数自动添加：
//   - tenant_id 过滤（除非是 platform 用户且未指定 TenantID）
//   - 数据范围过滤（根据 DataScopes 生成对应的 SQL WHERE 条件）
func ApplyDataScope(db *gorm.DB, principal *AuthPrincipal) *gorm.DB {
	if db == nil || principal == nil {
		return db
	}

	// 1. 租户隔离：非 platform 用户必须按 tenant_id 过滤
	if !principal.IsPlatform() && principal.TenantID > 0 {
		db = db.Where("tenant_id = ?", principal.TenantID)
	}

	// 2. 数据范围过滤
	for _, scope := range principal.DataScopes {
		if !scope.Include {
			continue
		}

		switch scope.ScopeType {
		case "all_tenant":
			// 全租户范围，不需要额外过滤（tenant_id 已过滤）
			continue

		case "self":
			// 个人范围：user_id = 当前用户
			// 注意：不同表可能用不同字段名，需要调用方自行处理
			continue

		case "assigned_ticket":
			// 指派工单：current_assignee_id = 当前用户
			db = db.Where("(current_assignee_id = ? OR current_assignee_id = 0)", principal.UserID)
			continue

		case "product_line":
			// 产品线范围
			if len(scope.ScopeValues) > 0 {
				db = db.Where("product_id IN ?", scope.ScopeValues)
			}

		case "region":
			// 区域范围
			if len(scope.ScopeValues) > 0 {
				db = db.Where("region_code IN ?", scope.ScopeValues)
			}

		case "customer_org":
			// 客户组织范围
			if len(scope.ScopeValues) > 0 {
				db = db.Where("customer_org_id IN ?", scope.ScopeValues)
			}

		case "device":
			// 设备范围
			if len(scope.ScopeValues) > 0 {
				db = db.Where("device_id IN ?", scope.ScopeValues)
			}
		}
	}

	return db
}

// DataScopeFilter 是 GORM 的 scope 函数，用于在查询中注入数据范围。
// 可直接在 Repository 层的查询中使用。
//
// 用法示例：
//
//	sqls.DB().Scopes(middleware.DataScopeFilter(principal)).Find(&tickets)
func DataScopeFilter(principal *AuthPrincipal) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return ApplyDataScope(db, principal)
	}
}

// DataScopeMiddlewareV2 require data scope from context before proceeding
func RequireDataScope(ctx *gin.Context) error {
	principal := GetAuthPrincipal(ctx)
	if principal == nil {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if len(principal.DataScopes) == 0 {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	return nil
}
