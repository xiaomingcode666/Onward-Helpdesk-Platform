// Package testutil contains fixtures only; application services never import it.
package testutil

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"remotehelpdesk/internal/pkg/config"
	"testing"
	"time"
)

// CleanClamAV supplies the TCP protocol for unrelated upload/authorization unit tests.
// Antivirus behavior is covered separately with injected failures and real clamd.
func CleanClamAV(t *testing.T) config.ClamAVSecurityConfig {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				r := bufio.NewReader(conn)
				command, err := r.ReadString(0)
				if err != nil {
					return
				}
				if command == "zVERSION\x00" {
					_, _ = io.WriteString(conn, "ClamAV 1.4.3/99999/"+time.Now().UTC().Format("Mon Jan _2 15:04:05 2006")+"\x00")
					return
				}
				if command != "zINSTREAM\x00" {
					return
				}
				for {
					var n uint32
					if binary.Read(r, binary.BigEndian, &n) != nil {
						return
					}
					if n == 0 {
						break
					}
					if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
						return
					}
				}
				_, _ = io.WriteString(conn, "stream: OK\x00")
			}()
		}
	}()
	return config.ClamAVSecurityConfig{Enabled: true, Address: l.Addr().String()}
}
