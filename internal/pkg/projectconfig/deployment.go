package projectconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Deployment is a content-addressed export of an applied configuration version.
type Deployment struct {
	VersionID int64    `json:"version_id"`
	Digest    string   `json:"digest"`
	Document  Document `json:"document"`
}

func ReadDeployment(path string, tenantID int64, environment, expectedDigest, secretRoot string) (*Deployment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("部署配置文件不可读取")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 300*1024+1))
	if err != nil || len(b) > 300*1024 {
		return nil, fmt.Errorf("部署配置读取失败或超过 300 KiB")
	}
	var bundle Deployment
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&bundle); err != nil {
		return nil, fmt.Errorf("部署配置格式错误或包含不支持的字段")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("只能提供一份部署配置")
	}
	if bundle.VersionID <= 0 || len(expectedDigest) != 64 || bundle.Digest != expectedDigest || Digest(bundle.Document) != expectedDigest {
		return nil, fmt.Errorf("部署配置版本或内容摘要与指定版本不一致")
	}
	r := Validate(bundle.Document, tenantID, environment, func(t int64, e, ref string) error { return CheckSecret(secretRoot, t, e, ref) })
	if !r.Valid {
		return nil, fmt.Errorf("部署配置检查失败：%s：%s", r.Issues[0].Path, r.Issues[0].Message)
	}
	return &bundle, nil
}
