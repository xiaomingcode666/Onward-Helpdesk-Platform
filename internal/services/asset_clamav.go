package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

const (
	ScanInfected      = "infected"
	ScanLimitExceeded = "limit_exceeded"
	ScanArchiveError  = "archive_error"
	ScanFailed        = "scan_failed"
	ScanPolicyBlocked = "policy_blocked"
)

type assetScanVerdict struct {
	Reason          string
	Detail          string
	EngineVersion   string
	DatabaseVersion string
	Performed       bool
}

// Every permitted file must have a complete clamd reply and a usable signature database.
// Legacy Enabled/FailClosed configuration can never turn an error into a clean verdict.
func scanAttachmentWithClamAV(data []byte, cfg config.ClamAVSecurityConfig) assetScanVerdict {
	result := assetScanVerdict{Reason: ScanFailed}
	if !cfg.Enabled {
		result.Detail = "ClamAV is not enabled"
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.TimeoutOrDefault())
	defer cancel()
	version, err := clamAVCommand(ctx, cfg, "VERSION", nil)
	if err != nil {
		result.Detail = "ClamAV version check failed"
		return result
	}
	parts := strings.SplitN(strings.TrimPrefix(version, "ClamAV "), "/", 3)
	if !strings.HasPrefix(version, "ClamAV ") || len(parts) != 3 {
		result.Detail = "ClamAV signature database is not ready"
		return result
	}
	result.EngineVersion, result.DatabaseVersion = parts[0], parts[1]
	databaseTime, err := time.Parse("Mon Jan _2 15:04:05 2006", parts[2])
	if err != nil || time.Since(databaseTime) > cfg.DatabaseMaxAge() || time.Until(databaseTime) > 24*time.Hour {
		result.Detail = "ClamAV signature database is stale or has an invalid timestamp"
		return result
	}
	reply, err := clamAVCommand(ctx, cfg, "INSTREAM", data)
	if err != nil {
		result.Detail = "ClamAV scan failed, timed out or returned an incomplete reply"
		return result
	}
	result.Performed = true
	result.Detail = reply
	if reply == "stream: OK" {
		result.Reason = ""
		return result
	}
	lower := strings.ToLower(reply)
	switch {
	case strings.Contains(lower, "limit") || strings.Contains(lower, "exceeded") || strings.Contains(lower, "size limit"):
		result.Reason = ScanLimitExceeded
	case strings.Contains(lower, "encrypted") || strings.Contains(lower, "broken") || strings.Contains(lower, "unpack"):
		result.Reason = ScanArchiveError
	case strings.HasPrefix(reply, "stream: ") && strings.HasSuffix(reply, " FOUND"):
		result.Reason = ScanInfected
	}
	return result
}

func clamAVCommand(ctx context.Context, cfg config.ClamAVSecurityConfig, command string, data []byte) (string, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", cfg.AddressOrDefault())
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err = io.Copy(conn, strings.NewReader("z"+command+"\x00")); err != nil {
		return "", err
	}
	if command == "INSTREAM" {
		for offset := 0; offset < len(data); {
			end := min(offset+32*1024, len(data))
			if err = binary.Write(conn, binary.BigEndian, uint32(end-offset)); err != nil {
				return "", err
			}
			if _, err = io.Copy(conn, bytes.NewReader(data[offset:end])); err != nil {
				return "", err
			}
			offset = end
		}
		if err = binary.Write(conn, binary.BigEndian, uint32(0)); err != nil {
			return "", err
		}
	}
	// INSTREAM uses NUL framing. EOF, oversized and partial replies are failures, even if ending in OK.
	reply, err := bufio.NewReader(io.LimitReader(conn, 4096)).ReadString(0)
	if err != nil {
		return "", fmt.Errorf("clamd response: %w", err)
	}
	return strings.TrimSpace(strings.TrimSuffix(reply, "\x00")), nil
}
