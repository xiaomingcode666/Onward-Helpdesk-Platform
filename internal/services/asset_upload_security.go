package services

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/services/storage"
)

var builtInBlockedUploadExtensions = map[string]struct{}{
	".apk": {}, ".app": {}, ".bat": {}, ".bash": {}, ".bin": {}, ".cmd": {},
	".com": {}, ".cpl": {}, ".deb": {}, ".dex": {}, ".dll": {}, ".dmg": {},
	".exe": {}, ".fish": {}, ".gadget": {}, ".hta": {}, ".htm": {}, ".html": {},
	".img": {}, ".iso": {}, ".jar": {}, ".js": {}, ".jse": {}, ".lnk": {},
	".mjs": {}, ".msi": {}, ".msp": {}, ".ocx": {}, ".php": {}, ".pkg": {},
	".pl": {}, ".ps1": {}, ".py": {}, ".reg": {}, ".rpm": {}, ".scr": {},
	".sh": {}, ".svg": {}, ".sys": {}, ".vb": {}, ".vbe": {}, ".vbs": {},
	".war": {}, ".wsf": {}, ".zsh": {},
}

var builtInBlockedUploadMIMETypes = map[string]struct{}{
	"application/hta":                               {},
	"application/java-archive":                      {},
	"application/javascript":                        {},
	"application/vnd.microsoft.portable-executable": {},
	"application/x-dosexec":                         {},
	"application/x-executable":                      {},
	"application/x-httpd-php":                       {},
	"application/x-msdownload":                      {},
	"application/x-sh":                              {},
	"application/x-sharedlib":                       {},
	"image/svg+xml":                                 {},
	"text/html":                                     {},
	"text/javascript":                               {},
}

var dangerousUploadMagic = [][]byte{
	{0x4d, 0x5a},             // PE / DOS executable
	{0x7f, 0x45, 0x4c, 0x46}, // ELF executable
	{0xca, 0xfe, 0xba, 0xbe}, // Java class or universal Mach-O
	{0xce, 0xfa, 0xed, 0xfe}, // Mach-O 32-bit
	{0xcf, 0xfa, 0xed, 0xfe}, // Mach-O 64-bit
	{0xfe, 0xed, 0xfa, 0xce}, // Mach-O 32-bit (big endian)
	{0xfe, 0xed, 0xfa, 0xcf}, // Mach-O 64-bit (big endian)
	[]byte("dex\n"),          // Android DEX
}

func inspectUpload(reader io.Reader, info storage.UploadInfo, cfg config.StorageConfig) ([]byte, storage.UploadInfo, error) {
	if reader == nil {
		return nil, info, errorsx.InvalidParamI18n("error.upload.empty")
	}

	maxSize := cfg.MaxUploadSizeBytes()
	data, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return nil, info, err
	}
	if int64(len(data)) > maxSize {
		return nil, info, errorsx.InvalidParamI18n("error.e0079")
	}
	if len(data) == 0 {
		return nil, info, errorsx.InvalidParamI18n("error.upload.empty")
	}

	filename, err := sanitizeUploadFilename(info.Filename)
	if err != nil {
		return nil, info, err
	}
	info.Filename = filename
	info.FileSize = int64(len(data))
	info.MimeType = detectedUploadMIME(data, filename, info.MimeType)

	security := cfg.UploadSecurity
	if security.EnabledOrDefault() {
		if uploadIsDangerous(data, info, security) {
			return nil, info, errorsx.InvalidParamI18n("error.upload.dangerous")
		}
		if err := validateUploadArchive(data, info, security); err != nil {
			return nil, info, err
		}
	}

	return data, info, nil
}

func validateUploadArchive(data []byte, info storage.UploadInfo, security config.UploadSecurityConfig) error {
	return checkArchiveContents(data, info.Filename, security)
}

func sanitizeUploadFilename(filename string) (string, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" {
		filename = ""
	}
	for _, value := range filename {
		if value == 0 || unicode.IsControl(value) {
			return "", errorsx.InvalidParamI18n("error.upload.invalidFilename")
		}
	}
	if filename == "" {
		return "upload", nil
	}
	return filename, nil
}

func detectedUploadMIME(data []byte, filename, declaredMIME string) string {
	detected := strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0])
	ext := strings.ToLower(filepath.Ext(filename))
	declaredMIME = strings.ToLower(strings.TrimSpace(strings.Split(declaredMIME, ";")[0]))
	if mimeType := detectedBrowserAudioMIME(data, ext, declaredMIME); mimeType != "" {
		return mimeType
	}
	if detected == "application/zip" {
		switch ext {
		case ".docx":
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		case ".xlsx":
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case ".pptx":
			return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		}
	}
	return detected
}

func detectedBrowserAudioMIME(data []byte, ext, declaredMIME string) string {
	switch {
	case ext == ".webm" && declaredMIME == "audio/webm" && bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3}):
		return "audio/webm"
	case ext == ".ogg" && declaredMIME == "audio/ogg" && bytes.HasPrefix(data, []byte("OggS")):
		return "audio/ogg"
	case (ext == ".m4a" || ext == ".mp4") && (declaredMIME == "audio/mp4" || declaredMIME == "audio/x-m4a") && len(data) >= 12 && string(data[4:8]) == "ftyp":
		return "audio/mp4"
	default:
		return ""
	}
}

func uploadIsDangerous(data []byte, info storage.UploadInfo, security config.UploadSecurityConfig) bool {
	ext := strings.ToLower(filepath.Ext(info.Filename))
	if _, blocked := builtInBlockedUploadExtensions[ext]; blocked {
		return true
	}
	for _, configured := range security.BlockedExtensions {
		configured = strings.ToLower(strings.TrimSpace(configured))
		if configured != "" && !strings.HasPrefix(configured, ".") {
			configured = "." + configured
		}
		if ext == configured {
			return true
		}
	}

	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(info.MimeType, ";")[0]))
	if _, blocked := builtInBlockedUploadMIMETypes[mimeType]; blocked {
		return true
	}
	for _, configured := range security.BlockedMIMETypes {
		if mimeType == strings.ToLower(strings.TrimSpace(strings.Split(configured, ";")[0])) {
			return true
		}
	}

	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("#!")) {
		return true
	}
	for _, magic := range dangerousUploadMagic {
		if bytes.HasPrefix(data, magic) {
			return true
		}
	}
	upper := bytes.ToUpper(data)
	return bytes.Contains(upper, []byte("EICAR-STANDARD-ANTIVIRUS-TEST-FILE"))
}
