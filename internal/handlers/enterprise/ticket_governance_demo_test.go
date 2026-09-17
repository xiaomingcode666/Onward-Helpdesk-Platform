package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"remotehelpdesk/internal/services"
)

// Opt-in, loopback-only walkthrough. Authentication and identities are synthetic;
// all ticket mutations use the real handlers/services and an isolated memory DB.
// No business API request is forwarded to the user's running backend.
func TestTicketGovernanceInteractiveDemo(t *testing.T) {
	if os.Getenv("HELPDESK_GOVERNANCE_DEMO") != "1" {
		t.Skip("interactive demo only")
	}
	f := newCaseHTTPFixture(t)
	if err := f.db.AutoMigrate(&models.TicketGovernanceOperation{}, &models.TicketRelation{}, &models.TicketPriorityProposal{}, &models.TicketNoSequence{}, &models.TicketContextSnapshot{}, &models.Tag{}, &models.TicketTag{}, &models.LoginSession{}); err != nil {
		t.Fatal(err)
	}
	conn, _ := f.db.DB()
	conn.SetMaxOpenConns(8)
	manager := f.member(t, "manager", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketCreate.Code, constants.PermissionTicketUpdate.Code, constants.PermissionTicketChangeStatus.Code})
	engineer := f.owner
	manager.Roles = []string{"service_manager"}
	customer := models.Customer{Name: "演示客户 A", PrimaryEmail: "customer-a@example.invalid"}
	if err := f.db.Create(&customer).Error; err != nil {
		t.Fatal(err)
	}
	f.db.Delete(&f.ticket)
	seed := func(title, typ string) *models.Ticket {
		item, err := services.TicketService.CreateTicket(request.CreateTicketRequest{Title: title, Description: "隔离演示数据：支付服务异常，需要跟进恢复。", Source: "manual", Channel: "enterprise", CustomerID: customer.ID, TicketClassificationInput: dto.TicketClassificationInput{CaseType: typ}}, manager)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	parent := seed("【演示父单】支付平台中断", "major_incident")
	child := seed("【演示子单 B】B 站点支付失败", "incident")
	f.db.Model(child).Update("current_assignee_id", engineer.UserID)
	for _, level := range []string{"p1", "p2", "p3", "p4"} {
		policy := services.SLAPolicy{ID: "demo-" + level, TenantID: "9401", Priority: ticketpolicy.LegacyCode(level), ResolutionMinutes: map[string]int{"p1": 60, "p2": 240, "p3": 480, "p4": 1440}[level], Status: "active"}
		if err := f.db.Create(&policy).Error; err != nil {
			t.Fatal(err)
		}
	}
	f.router.GET("/api/enterprise/v1/tickets/summary", TicketSummary)
	f.router.POST("/api/enterprise/v1/tickets", TicketCreate)
	f.router.GET("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	f.router.POST("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	f.router.GET("/api/enterprise/v1/ticket-settings/classification", TicketClassificationPolicy)
	profile := func(actor string) map[string]any {
		op := manager
		name := "管理负责人（隔离演示）"
		role := "service_manager"
		if actor == "engineer" {
			op = engineer
			name = "处理工程师（隔离演示）"
			role = "engineer"
		}
		return map[string]any{"accessToken": "demo-" + actor, "domain": "enterprise", "domainType": "enterprise", "tenantId": 9401, "tenant_id": 9401, "locale": "zh-CN", "timezone": "Asia/Shanghai", "roles": []string{role}, "permissions": op.Permissions, "user": map[string]any{"id": op.UserID, "username": op.Username, "nickname": name, "roles": []string{role}, "status": 0}}
	}
	target, _ := url.Parse("http://localhost:3000")
	proxy := httputil.NewSingleHostReverseProxy(target)
	jsonResponse := func(w http.ResponseWriter, data any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		actor := r.URL.Query().Get("actor")
		if actor != "engineer" {
			actor = "manager"
		}
		p, _ := json.Marshal(profile(actor))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>工单隔离演示</title><body style="font:18px system-ui;max-width:760px;margin:70px auto;line-height:1.9"><h1>分类、优先级与父子工单演示</h1><p>这里使用真实工单页面和后端业务逻辑。账号和数据均为临时演示数据，不连接业务数据库，不发送外部消息。</p><p>父单：%s · 支付平台中断（重大事件 P1）<br>待关联子单：%s · B 站点支付失败（故障事件 P1）</p><button style="padding:14px 28px" onclick='const p=%s;localStorage.setItem("remote-helpdesk-session:enterprise",JSON.stringify(p));localStorage.setItem("remote-helpdesk-session",JSON.stringify(p));location.href="/enterprise/tickets"'>以%s进入演示</button><p><a href="/demo?actor=manager">切换管理负责人</a>　<a href="/demo?actor=engineer">切换工程师</a></p><p>演示服务仅监听本机，30 分钟后自动停止。</p></body></html>`, parent.TicketNo, child.TicketNo, p, map[string]string{"manager": "管理负责人", "engineer": "工程师"}[actor])
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		actor := "manager"
		if r.Header.Get("Authorization") == "Bearer demo-engineer" {
			actor = "engineer"
		}
		path := r.URL.Path
		switch {
		case path == "/api/auth/profile":
			jsonResponse(w, profile(actor))
		case strings.HasSuffix(path, "/tickets/customer-options"):
			jsonResponse(w, []any{map[string]any{"customer_id": customer.ID, "display_name": customer.Name}})
		case strings.Contains(path, "capabilities"):
			jsonResponse(w, map[string]any{"service_scene": "knowledge_support", "features": map[string]any{}})
		case strings.Contains(path, "unread"):
			jsonResponse(w, map[string]any{"count": 0, "unread_count": 0})
		case strings.HasPrefix(path, "/api/enterprise/v1/tickets") || strings.HasPrefix(path, "/api/enterprise/v1/ticket-settings"):
			key := "manager"
			if actor == "engineer" {
				key = "owner"
			}
			r.Header.Set("X-Case-Fixture-Actor", key)
			f.router.ServeHTTP(w, r)
		case strings.Contains(path, "/ws/"):
			http.Error(w, "not enabled in demo", http.StatusNotImplemented)
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "演示未启用该辅助功能"})
		}
	})
	mux.Handle("/", proxy)
	ln, err := net.Listen("tcp", "127.0.0.1:3112")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(ln) }()
	t.Logf("DEMO_READY http://127.0.0.1:3112/demo parent=%d child=%d", parent.ID, child.ID)
	<-time.After(30 * time.Minute)
	_ = server.Shutdown(context.Background())
}
