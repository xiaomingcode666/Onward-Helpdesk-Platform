package services

import (
	"log/slog"
	"mime/multipart"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services/storage"

	"github.com/mlogclub/simple/sqls"
)

const (
	SystemIntroDocStatusDraft     = "draft"
	SystemIntroDocStatusPublished = "published"
)

var SystemIntroService = newSystemIntroService()

func newSystemIntroService() *systemIntroService {
	return &systemIntroService{}
}

type systemIntroService struct{}

// ListPage 管理端分页列表（不过滤上下架状态），关键字匹配标题/文件名。
func (s *systemIntroService) ListPage(keyword string, page, limit int) ([]response.PlatformSystemIntroDocResponse, int64, error) {
	cnd := sqls.NewCnd()
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		cnd.Like("title", keyword)
	}
	cnd.Asc("sort_no").Desc("id").Page(page, limit)
	items, paging := repositories.SystemIntroRepository.FindPageByCnd(sqls.DB(), cnd)
	total := int64(0)
	if paging != nil {
		total = paging.Total
	}
	result := make([]response.PlatformSystemIntroDocResponse, 0, len(items))
	for _, item := range items {
		result = append(result, buildSystemIntroDocResponse(&item, AssetService.Get(item.AssetID)))
	}
	return result, total, nil
}

// UploadDoc 上传系统介绍文档：先落 assets 再建业务行，失败回滚删除 asset。默认草稿态。
func (s *systemIntroService) UploadDoc(file *multipart.FileHeader, title string, sortNo int, operator *dto.AuthPrincipal) (response.PlatformSystemIntroDocResponse, error) {
	if operator == nil {
		return response.PlatformSystemIntroDocResponse{}, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	asset, err := AssetService.UploadFile(file, "system-intro", operator)
	if err != nil {
		return response.PlatformSystemIntroDocResponse{}, err
	}
	item := &models.SystemIntroDoc{
		AssetID:     asset.ID,
		Title:       firstNonBlank(strings.TrimSpace(title), strings.TrimSpace(file.Filename)),
		FileName:    asset.Filename,
		FileSize:    asset.FileSize,
		MimeType:    asset.MimeType,
		SortNo:      sortNo,
		Status:      SystemIntroDocStatusDraft,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := repositories.SystemIntroRepository.Create(sqls.DB(), item); err != nil {
		_ = AssetService.DeleteAsset(asset.ID, operator)
		return response.PlatformSystemIntroDocResponse{}, err
	}
	return buildSystemIntroDocResponse(item, asset), nil
}

// UpdateDoc 编辑系统介绍文档：标题/排序/上下架局部更新。上架写入 PublishedAt，下架清空。
func (s *systemIntroService) UpdateDoc(req request.PlatformSystemIntroUpdateRequest, operator *dto.AuthPrincipal) (response.PlatformSystemIntroDocResponse, error) {
	if operator == nil {
		return response.PlatformSystemIntroDocResponse{}, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := repositories.SystemIntroRepository.Get(sqls.DB(), req.ID)
	if item == nil {
		return response.PlatformSystemIntroDocResponse{}, errorsx.InvalidParam("system intro doc not found")
	}
	updates := map[string]any{
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}
	if title := strings.TrimSpace(req.Title); title != "" {
		updates["title"] = title
	}
	if req.SortNo != nil {
		updates["sort_no"] = *req.SortNo
	}
	status := strings.TrimSpace(req.Status)
	if status == SystemIntroDocStatusDraft || status == SystemIntroDocStatusPublished {
		if status != item.Status {
			updates["status"] = status
			if status == SystemIntroDocStatusPublished {
				now := time.Now()
				updates["published_at"] = now
			} else {
				updates["published_at"] = nil
			}
		}
	}
	if err := repositories.SystemIntroRepository.Updates(sqls.DB(), item.ID, updates); err != nil {
		return response.PlatformSystemIntroDocResponse{}, err
	}
	updated := repositories.SystemIntroRepository.Get(sqls.DB(), item.ID)
	return buildSystemIntroDocResponse(updated, AssetService.Get(updated.AssetID)), nil
}

// DeleteDoc 删除系统介绍文档：先删除关联 asset 再硬删业务行。
func (s *systemIntroService) DeleteDoc(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := repositories.SystemIntroRepository.Get(sqls.DB(), id)
	if item == nil {
		return errorsx.InvalidParam("system intro doc not found")
	}
	if err := AssetService.DeleteAsset(item.AssetID, operator); err != nil {
		return err
	}
	return repositories.SystemIntroRepository.Delete(sqls.DB(), id)
}

// ListPublishedDocs 客户门户访客可见列表（仅已上架，按排序号升序）。
// 注意：访客 scope 为空，此方法绝不调用 resolveScope；asset 缺失/未成功/签名 URL 失败时跳过该条。
func (s *systemIntroService) ListPublishedDocs() ([]dto.CustomerPortalSystemIntroDocDTO, error) {
	cnd := sqls.NewCnd().Eq("status", SystemIntroDocStatusPublished).Asc("sort_no").Asc("id")
	items := repositories.SystemIntroRepository.Find(sqls.DB(), cnd)
	result := make([]dto.CustomerPortalSystemIntroDocDTO, 0, len(items))
	for _, item := range items {
		asset := AssetService.Get(item.AssetID)
		if asset == nil || asset.Status != enums.AssetStatusSuccess {
			continue
		}
		url, err := AssetService.GetSignedURL(asset.ID)
		if err != nil || url == "" {
			continue
		}
		result = append(result, dto.CustomerPortalSystemIntroDocDTO{
			ID:          item.ID,
			Title:       firstNonBlank(item.Title, asset.Filename),
			Filename:    asset.Filename,
			FileSize:    asset.FileSize,
			MimeType:    asset.MimeType,
			URL:         url,
			PublishedAt: formatCustomerTimePtr(item.PublishedAt),
		})
	}
	return result, nil
}

// buildSystemIntroDocResponse 组装管理端响应。文件名/大小/MIME/签名 URL 取自 asset。
func buildSystemIntroDocResponse(item *models.SystemIntroDoc, asset *models.Asset) response.PlatformSystemIntroDocResponse {
	result := response.PlatformSystemIntroDocResponse{
		ID:             item.ID,
		AssetID:        item.AssetID,
		Title:          item.Title,
		SortNo:         item.SortNo,
		Status:         item.Status,
		PublishedAt:    formatSystemIntroTime(item.PublishedAt),
		CreatedAt:      item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      item.UpdatedAt.Format("2006-01-02 15:04:05"),
		CreateUserName: item.CreateUserName,
	}
	if asset != nil {
		result.Filename = asset.Filename
		result.FileSize = asset.FileSize
		result.MimeType = asset.MimeType
		if result.Title == "" {
			result.Title = asset.Filename
		}
		if provider, err := storage.GetProvider(asset.Provider); err == nil {
			result.URL = provider.GetSignedURL(asset.StorageKey)
		} else {
			slog.Error("get storage provider failed for system intro doc", "asset_id", asset.ID, "error", err)
		}
	}
	return result
}

func formatSystemIntroTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}
