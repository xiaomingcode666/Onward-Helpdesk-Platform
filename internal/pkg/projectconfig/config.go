// Package projectconfig owns the versioned configuration contract shared by
// the API and deployment gate. It has no database or HTTP dependencies.
package projectconfig

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

//go:embed schema.json
var SchemaJSON []byte

type Rule struct {
	ProjectKey     string   `json:"project_key"`
	Channel        string   `json:"channel"`
	TicketType     string   `json:"ticket_type"`
	RequiredFields []string `json:"required_fields"`
}

type IntakePolicy struct {
	Rules []Rule `json:"rules"`
}
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}
type Document struct {
	SchemaVersion int          `json:"schema_version"`
	TenantID      int64        `json:"tenant_id"`
	Environment   string       `json:"environment"`
	Projects      []Project    `json:"projects"`
	Intake        IntakePolicy `json:"intake"`
	SecretRefs    []string     `json:"secret_refs"`
	Runtime       *Runtime     `json:"runtime,omitempty"`
}

type Issue struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Report struct {
	Valid  bool    `json:"valid"`
	Issues []Issue `json:"issues"`
}

var schemaOnce sync.Once
var resolved *jsonschema.Resolved
var schemaErr error

// Decode rejects unknown fields, trailing documents and oversized payloads.
func Decode(data []byte) (Document, error) {
	var doc Document
	if len(data) > 256*1024 {
		return doc, fmt.Errorf("配置不能超过 256 KiB")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&doc); err != nil {
		return doc, fmt.Errorf("配置格式不正确或包含不支持的字段")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return doc, fmt.Errorf("只能提交一份配置")
	}
	return doc, nil
}

func Digest(doc Document) string {
	b, _ := json.Marshal(doc)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func Environment() string {
	if value := strings.TrimSpace(os.Getenv("RHD_PROJECT_ENVIRONMENT")); value != "" {
		return value
	}
	return "development"
}

// DeploymentManaged configurations may only be activated by deployment tooling.
// Mounted bundles and their digest environment variables are immutable to the API.
func DeploymentManaged() bool {
	return os.Getenv("RHD_PROJECT_CONFIG_FILE") != "" || os.Getenv("RHD_PROJECT_CONFIG_REQUIRED") == "1" || Environment() == "staging" || Environment() == "production"
}

// ValidateDraftShape permits unfinished business rules, but not missing arrays.
// Historical drafts are left intact; clients must also handle older null fields.
func ValidateDraftShape(doc Document) error {
	if (doc.SchemaVersion != 1 && doc.SchemaVersion != 2) || doc.Projects == nil || doc.Intake.Rules == nil || doc.SecretRefs == nil {
		return fmt.Errorf("草稿必须包含 schema_version=1 或 2、projects、intake.rules 和 secret_refs；空列表请填写 []")
	}
	if doc.SchemaVersion == 2 && doc.Runtime == nil {
		return fmt.Errorf("版本 2 草稿必须包含运营设置 runtime")
	}
	if r := doc.Runtime; r != nil {
		refs := []string{r.Mail.PasswordRef}
		for _, i := range r.Integrations {
			refs = append(refs, i.SecretRef, i.KeyRef)
		}
		for _, ref := range refs {
			if ref != "" && !ValidSecretReference(ref) {
				return fmt.Errorf("密钥字段只能填写 secret://名称，不能保存密码")
			}
		}
		if r.Locales == nil || r.Calendars == nil || r.Targets == nil || r.Channels == nil || r.Integrations == nil {
			return fmt.Errorf("运营草稿必须包含语言、日历、目标、渠道和接入列表；空列表填写 []")
		}
		for _, c := range r.Calendars {
			if c.WorkDays == nil || c.Holidays == nil {
				return fmt.Errorf("每个日历必须包含工作日和休息日期列表")
			}
		}
	}
	if len(doc.Projects) > 200 || len(doc.Intake.Rules) > 200 || len(doc.SecretRefs) > 64 {
		return fmt.Errorf("草稿中的项目、规则或密钥引用数量超限")
	}
	for _, rule := range doc.Intake.Rules {
		if rule.RequiredFields == nil {
			return fmt.Errorf("每条规则必须包含 required_fields；没有额外必填项请填写 []")
		}
	}
	return nil
}

// SecretChecker never returns secret values. A reference names a file below
// a tenant/environment-specific directory controlled by the operator.
type SecretChecker func(tenantID int64, environment, ref string) error

var secretName = regexp.MustCompile(`^secret://[a-z][a-z0-9_-]{0,63}$`)

func ValidSecretReference(ref string) bool { return secretName.MatchString(ref) }

func CheckSecret(root string, tenantID int64, environment, ref string) error {
	_, err := ReadSecret(root, tenantID, environment, ref)
	return err
}

// ReadSecret resolves within an OS-enforced root and reads the validated handle.
// Callers must never put its result in configuration snapshots or error messages.
func ReadSecret(root string, tenantID int64, environment, ref string) ([]byte, error) {
	if !secretName.MatchString(ref) || !validEnvironment(environment) {
		return nil, fmt.Errorf("密钥引用格式不正确")
	}
	if root == "" {
		return nil, fmt.Errorf("此环境尚未配置密钥目录")
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("此环境的密钥目录不可读取")
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("此环境的密钥目录不可读取")
	}
	path := base
	for _, part := range []string{fmt.Sprint(tenantID), environment, strings.TrimPrefix(ref, "secret://")} {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("当前公司及环境下的密钥不存在或使用了不允许的链接")
		}
	}
	scope, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("当前公司及环境下的密钥不可读取")
	}
	defer scope.Close()
	f, err := scope.Open(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("当前公司及环境下的密钥不可读取")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64*1024 {
		return nil, fmt.Errorf("密钥必须是小于 64 KiB 的普通文件")
	}
	b, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil || len(b) > 64*1024 || len(bytes.TrimSpace(b)) == 0 {
		return nil, fmt.Errorf("密钥为空、超限或不可读取")
	}
	return bytes.TrimSpace(b), nil
}

func RuntimeSecretCheck(tenantID int64, environment, ref string) error {
	return CheckSecret(os.Getenv("RHD_PROJECT_SECRET_DIR"), tenantID, environment, ref)
}

func validEnvironment(s string) bool {
	return s == "development" || s == "staging" || s == "production" || s == "integration"
}

// Validate checks JSON Schema, policy and secret references in that order.
// Error messages deliberately never include submitted values or secret content.
func Validate(doc Document, tenantID int64, environment string, check SecretChecker) Report {
	r := Report{Valid: true, Issues: []Issue{}}
	add := func(kind, path, message string) {
		r.Valid = false
		r.Issues = append(r.Issues, Issue{kind, path, message})
	}
	schemaOnce.Do(func() {
		var s jsonschema.Schema
		if schemaErr = json.Unmarshal(SchemaJSON, &s); schemaErr == nil {
			resolved, schemaErr = s.Resolve(nil)
		}
	})
	if schemaErr != nil {
		add("schema", "", "配置校验器不可用")
		return r
	}
	b, _ := json.Marshal(doc)
	var value any
	_ = json.Unmarshal(b, &value)
	if err := resolved.Validate(value); err != nil {
		add("schema", "document", "配置结构不符合规范，请检查版本、项目、环境和受理规则字段")
		return r
	}
	if doc.TenantID != tenantID {
		add("policy", "tenant_id", "配置不属于当前公司")
	}
	if doc.Environment != environment || !validEnvironment(environment) {
		add("policy", "environment", "配置与目标运行环境不一致")
	}
	projects := map[string]bool{}
	for i, p := range doc.Projects {
		if p.Key != strings.TrimSpace(p.Key) || strings.TrimSpace(p.Name) == "" || projects[p.Key] {
			add("policy", fmt.Sprintf("projects[%d]", i), "项目标识不能重复或包含首尾空格，名称不能为空")
		}
		projects[p.Key] = true
	}
	seen := map[string]bool{}
	for i, rule := range doc.Intake.Rules {
		path := fmt.Sprintf("intake.rules[%d]", i)
		if !projects[rule.ProjectKey] {
			add("policy", path+".project_key", "请先登记这条规则所属的服务项目")
		}
		if rule.TicketType != strings.TrimSpace(rule.TicketType) {
			add("policy", path+".ticket_type", "工单类型不能包含首尾空格")
		}
		key, _ := json.Marshal([]string{rule.ProjectKey, rule.Channel, rule.TicketType})
		if seen[string(key)] {
			add("policy", path, "同一项目、渠道和工单类型只能配置一条规则")
		}
		seen[string(key)] = true
	}
	for i, ref := range doc.SecretRefs {
		if check == nil {
			add("secret", fmt.Sprintf("secret_refs[%d]", i), "未配置密钥校验器")
			continue
		}
		if doc.TenantID != tenantID || doc.Environment != environment {
			continue
		}
		if err := check(tenantID, environment, ref); err != nil {
			add("secret", fmt.Sprintf("secret_refs[%d]", i), err.Error())
		}
	}
	if doc.Runtime != nil {
		validateRuntime(doc, add)
	}
	return r
}
