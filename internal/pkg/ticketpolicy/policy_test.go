package ticketpolicy

import "testing"

func TestClassificationDefaultsAndEvidence(t *testing.T) {
	p := Default()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for typ, want := range map[string]string{"incident": "p1", "major_incident": "p1", "user_case": "p2", "problem": "p3", "known_error": "p3", "service_request": "p4"} {
		if got := p.Assess(typ, Facts{}); got.Priority != want || !got.Incomplete {
			t.Fatalf("%s: %+v", typ, got)
		}
	}
	f := Facts{Impact: "high", Urgency: "high", Reach: "high", Safety: "none", Workaround: "none"}
	if p.Assess("service_request", f).Priority != "p1" {
		t.Fatal("service request hid widespread outage")
	}
	f = Facts{Safety: "critical", Evidence: "confirmed risk"}
	if a := p.Assess("known_error", f); a.Priority != "p1" || !a.Incomplete {
		t.Fatal(a)
	}
	if (Facts{Workaround: "verified"}).Validate() == nil {
		t.Fatal("unsubstantiated workaround accepted")
	}
	if (Facts{Impact: "invented"}).Validate() == nil {
		t.Fatal("invalid assessment accepted")
	}
	f = Facts{Impact: "medium", Reach: "medium", Urgency: "medium", Safety: "none", Workaround: "verified", Evidence: "tested"}
	if p.Assess("incident", f).Priority != "p2" {
		t.Fatal("workaround must not automatically subtract a priority level")
	}
}
