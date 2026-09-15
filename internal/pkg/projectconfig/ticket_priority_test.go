package projectconfig

import (
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"testing"
)

func TestConfigurationTicketPrioritySchema(t *testing.T) {
	doc := testDocument()
	p := ticketpolicy.Default()
	doc.TicketPriority = &p
	if result := Validate(doc, 7, "staging", nil); !result.Valid {
		t.Fatalf("valid priority policy rejected: %+v", result)
	}
	for _, invalid := range []func(*ticketpolicy.Policy){
		func(p *ticketpolicy.Policy) { p.Defaults["incident"] = "p0" },
		func(p *ticketpolicy.Policy) { delete(p.Defaults, "user_case") },
		func(p *ticketpolicy.Policy) { p.Matrix[0] = []string{"p1"} },
		func(p *ticketpolicy.Policy) { p.Version = "" },
	} {
		p = ticketpolicy.Default()
		invalid(&p)
		if result := Validate(doc, 7, "staging", nil); result.Valid {
			t.Fatalf("invalid priority policy accepted: %+v", p)
		}
	}
}
