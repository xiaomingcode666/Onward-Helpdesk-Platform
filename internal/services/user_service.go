package services

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"slices"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var UserService = newUserService()

func newUserService() *userService {
	return &userService{}
}

type userService struct {
}

func (s *userService) Get(id int64) *models.User {
	return repositories.UserRepository.Get(sqls.DB(), id)
}

func (s *userService) Take(where ...interface{}) *models.User {
	return repositories.UserRepository.Take(sqls.DB(), where...)
}

func (s *userService) Find(cnd *sqls.Cnd) []models.User {
	return repositories.UserRepository.Find(sqls.DB(), cnd)
}

func (s *userService) FindOne(cnd *sqls.Cnd) *models.User {
	return repositories.UserRepository.FindOne(sqls.DB(), cnd)
}

func (s *userService) FindPageByParams(params *params.QueryParams) (list []models.User, paging *sqls.Paging) {
	return repositories.UserRepository.FindPageByParams(sqls.DB(), params)
}

func (s *userService) FindPageByCnd(cnd *sqls.Cnd) (list []models.User, paging *sqls.Paging) {
	return repositories.UserRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *userService) Count(cnd *sqls.Cnd) int64 {
	return repositories.UserRepository.Count(sqls.DB(), cnd)
}

func (s *userService) FindByIds(ids []int64) []models.User {
	return repositories.UserRepository.FindByIds(sqls.DB(), ids)
}

func (s *userService) Create(t *models.User) error {
	return repositories.UserRepository.Create(sqls.DB(), t)
}

func (s *userService) Update(t *models.User) error {
	return repositories.UserRepository.Update(sqls.DB(), t)
}

func (s *userService) Updates(id int64, columns map[string]interface{}) error {
	return repositories.UserRepository.Updates(sqls.DB(), id, columns)
}

func (s *userService) UpdateColumn(id int64, name string, value interface{}) error {
	return repositories.UserRepository.UpdateColumn(sqls.DB(), id, name, value)
}

func (s *userService) GetByUsername(username string) *models.User {
	return repositories.UserRepository.GetByUsername(sqls.DB(), username)
}

func (s *userService) GetByMobile(mobile string) *models.User {
	return repositories.UserRepository.GetByMobile(sqls.DB(), mobile)
}

func (s *userService) GetByEmail(email string) *models.User {
	return repositories.UserRepository.GetByEmail(sqls.DB(), email)
}

func (s *userService) CreateUser(req request.CreateUserRequest, operator *dto.AuthPrincipal) (*models.User, string, error) {
	var user *models.User
	var plain string
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var err error
		user, plain, err = s.CreateUserDB(ctx.Tx, req, operator)
		return err
	})
	return user, plain, err
}

func (s *userService) CreateUserDB(db *gorm.DB, req request.CreateUserRequest, operator *dto.AuthPrincipal) (*models.User, string, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, "", errorsx.InvalidParamI18n("error.e0257")
	}
	if repositories.UserRepository.GetByUsername(db, username) != nil {
		return nil, "", errorsx.InvalidParamI18n("error.e0259")
	}

	mobile := utils.NormalizeNullableString(req.Mobile)
	email := utils.NormalizeNullableString(req.Email)
	if mobile != nil && repositories.UserRepository.GetByMobile(db, *mobile) != nil {
		return nil, "", errorsx.InvalidParamI18n("error.e0206")
	}
	if email != nil && repositories.UserRepository.GetByEmail(db, *email) != nil {
		return nil, "", errorsx.InvalidParamI18n("error.e0338")
	}

	plain := strings.TrimSpace(req.Password)
	if plain == "" {
		var err error
		plain, err = utils.GenerateRandomPassword(12)
		if err != nil {
			return nil, "", err
		}
	}
	if len(plain) < 8 {
		return nil, "", errorsx.InvalidParam("password must contain at least 8 characters")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	user := &models.User{
		Username:     username,
		Nickname:     strings.TrimSpace(req.Nickname),
		Password:     string(passwordHash),
		Avatar:       strings.TrimSpace(req.Avatar),
		Mobile:       mobile,
		Email:        email,
		Status:       enums.StatusOk,
		Remark:       strings.TrimSpace(req.Remark),
		PasswordSalt: "",
		AuditFields:  utils.BuildAuditFields(operator),
	}
	if user.Nickname == "" {
		user.Nickname = username
	}

	if err := repositories.UserRepository.Create(db, user); err != nil {
		return nil, "", err
	}
	if len(req.RoleIDs) > 0 {
		if err := s.replaceUserRolesDB(db, user.ID, req.RoleIDs, operator); err != nil {
			return nil, "", err
		}
	}
	return user, plain, nil
}

func (s *userService) UpdateUser(req request.UpdateUserRequest, operator *dto.AuthPrincipal) error {
	user := s.Get(req.ID)
	if user == nil || user.DeletedAt.Valid {
		return errorsx.InvalidParamI18n("error.e0255")
	}

	mobile := utils.NormalizeNullableString(req.Mobile)
	email := utils.NormalizeNullableString(req.Email)
	if mobile != nil {
		if existed := s.GetByMobile(*mobile); existed != nil && existed.ID != req.ID {
			return errorsx.InvalidParamI18n("error.e0206")
		}
	}
	if email != nil {
		if existed := s.GetByEmail(*email); existed != nil && existed.ID != req.ID {
			return errorsx.InvalidParamI18n("error.e0338")
		}
	}

	return s.Updates(req.ID, map[string]any{
		"nickname":         strings.TrimSpace(req.Nickname),
		"avatar":           strings.TrimSpace(req.Avatar),
		"mobile":           mobile,
		"email":            email,
		"remark":           strings.TrimSpace(req.Remark),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *userService) UpdateOwnProfile(req request.UpdateOwnProfileRequest, operator *dto.AuthPrincipal) (*models.User, error) {
	if operator == nil || operator.UserID <= 0 {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	user := s.Get(operator.UserID)
	if user == nil || user.DeletedAt.Valid {
		return nil, errorsx.InvalidParamI18n("error.e0255")
	}

	mobile := utils.NormalizeNullableString(req.Mobile)
	email := utils.NormalizeNullableString(req.Email)
	if mobile != nil {
		if existed := s.GetByMobile(*mobile); existed != nil && existed.ID != user.ID {
			return nil, errorsx.InvalidParamI18n("error.e0206")
		}
	}
	if email != nil {
		if existed := s.GetByEmail(*email); existed != nil && existed.ID != user.ID {
			return nil, errorsx.InvalidParamI18n("error.e0338")
		}
	}

	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = user.Locale
	}
	locale = i18nx.NormalizeLocale(locale)
	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = strings.TrimSpace(user.Timezone)
	}
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}

	if err := s.Updates(user.ID, map[string]any{
		"nickname":         strings.TrimSpace(req.Nickname),
		"avatar":           strings.TrimSpace(req.Avatar),
		"mobile":           mobile,
		"email":            email,
		"locale":           locale,
		"timezone":         timezone,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return nil, err
	}
	return s.Get(user.ID), nil
}

func (s *userService) DeleteUser(id int64, operator *dto.AuthPrincipal) error {
	user := s.Get(id)
	if user == nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}

	if err := s.Updates(id, map[string]any{
		"status":           enums.StatusDisabled,
		"deleted_at":       time.Now(),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	return LoginSessionService.RevokeByUser(id, operator.UserID, operator.Username)
}

func (s *userService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	user := s.Get(id)
	if user == nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}
	if !slices.Contains(enums.StatusValues, enums.Status(status)) {
		return errorsx.InvalidParamI18n("error.e0254")
	}
	if err := s.Updates(id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if status == int(enums.StatusDisabled) || status == int(enums.StatusDeleted) {
		return LoginSessionService.RevokeByUser(id, operator.UserID, operator.Username)
	}
	return nil
}

func (s *userService) ResetPassword(userID int64, operator *dto.AuthPrincipal) (string, error) {
	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return "", err
	}
	if err = s.changePassword(userID, password, operator); err != nil {
		return "", err
	}
	return password, nil
}

func (s *userService) ChangeOwnPassword(password string, operator *dto.AuthPrincipal) error {
	if operator == nil || operator.UserID <= 0 {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return s.changePassword(operator.UserID, password, operator)
}

func (s *userService) SetPassword(userID int64, password string, operator *dto.AuthPrincipal) error {
	return s.changePassword(userID, password, operator)
}

func (s *userService) AssignRoles(userID int64, roleIDs []int64, operator *dto.AuthPrincipal) error {
	user := s.Get(userID)
	if user == nil || user.DeletedAt.Valid {
		return errorsx.InvalidParamI18n("error.e0255")
	}
	if err := s.replaceUserRoles(userID, roleIDs, operator); err != nil {
		return err
	}
	return LoginSessionService.RevokeByUser(userID, operator.UserID, operator.Username)
}

func (s *userService) replaceUserRoles(userID int64, roleIDs []int64, operator *dto.AuthPrincipal) error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return s.replaceUserRolesDB(ctx.Tx, userID, roleIDs, operator)
	})
}

func (s *userService) replaceUserRolesDB(db *gorm.DB, userID int64, roleIDs []int64, operator *dto.AuthPrincipal) error {
	if err := db.Where("user_id = ?", userID).Delete(&models.UserRole{}).Error; err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		role := RoleService.Get(roleID)
		if role == nil {
			return errorsx.InvalidParamI18n("error.e0305")
		}
		if role.Status != enums.StatusOk {
			return errorsx.InvalidParamI18n("error.e0291")
		}
		relation := &models.UserRole{
			UserID:      userID,
			RoleID:      roleID,
			AuditFields: utils.BuildAuditFields(operator),
		}
		if err := db.Create(relation).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *userService) changePassword(userID int64, password string, operator *dto.AuthPrincipal) error {
	user := s.Get(userID)
	if user == nil || user.DeletedAt.Valid {
		return errorsx.InvalidParamI18n("error.e0255")
	}
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return errorsx.InvalidParam("password must contain at least 8 characters")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now()
	if err = s.Updates(userID, map[string]any{
		"password":         string(passwordHash),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}); err != nil {
		return err
	}
	return LoginSessionService.RevokeByUser(userID, operator.UserID, operator.Username)
}
