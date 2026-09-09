package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func RenderJSON(w io.Writer, report *Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func RenderText(w io.Writer, report *Report) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}
	if _, err := fmt.Fprintf(w, "Suite: %s v%d\n", report.SuiteName, report.SuiteVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Generated At: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Cases: total=%d enabled=%d passed=%d failed=%d\n", report.Summary.TotalCases, report.Summary.EnabledCases, report.Summary.PassedCases, report.Summary.FailedCases); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Retrieval: recall@5=%.3f recall@10=%.3f mrr=%.3f forbidden=%d citation=%.3f p50=%dms p95=%dms\n", report.Summary.RecallAt5, report.Summary.RecallAt10, report.Summary.MRR, report.Summary.ForbiddenHitCount, report.Summary.CitationCoverage, report.Summary.P50LatencyMs, report.Summary.P95LatencyMs); err != nil {
		return err
	}
	if report.Summary.AnswerEvaluated {
		if _, err := fmt.Fprintf(w, "Answer: no-answer precision=%.3f recall=%.3f handoff=%.3f\n", report.Summary.NoAnswerPrecision, report.Summary.NoAnswerRecall, report.Summary.HandoffAccuracy); err != nil {
			return err
		}
	} else if report.Summary.AnswerAssertionsSkipped > 0 {
		if _, err := fmt.Fprintf(w, "Answer: skipped assertions=%d (run with answer generation to evaluate)\n", report.Summary.AnswerAssertionsSkipped); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tPASS\tR@5\tR@10\tMRR\tFORBID\tMODE\tLAT(ms)\tQUERY"); err != nil {
		return err
	}
	for _, item := range report.Cases {
		query := strings.TrimSpace(item.Query)
		if len(query) > 60 {
			query = query[:57] + "..."
		}
		pass := "FAIL"
		if item.Passed {
			pass = "PASS"
		}
		if !item.Enabled {
			pass = "SKIP"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\t%d\t%s\t%d\t%s\n",
			item.ID,
			pass,
			item.Retrieval.RecallAt5,
			item.Retrieval.RecallAt10,
			item.Retrieval.MRR,
			item.Retrieval.ForbiddenHitCount,
			item.Retrieval.AppliedMode,
			item.Retrieval.LatencyMs,
			query,
		); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	failed := make([]CaseReport, 0)
	for _, item := range report.Cases {
		if item.Enabled && !item.Passed {
			failed = append(failed, item)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	if _, err := io.WriteString(w, "\nFailures:\n"); err != nil {
		return err
	}
	for _, item := range failed {
		if _, err := fmt.Fprintf(w, "- %s %s\n", item.ID, item.Name); err != nil {
			return err
		}
		for _, failure := range item.Failures {
			if _, err := fmt.Fprintf(w, "  - %s\n", failure); err != nil {
				return err
			}
		}
		if item.Error != "" {
			if _, err := fmt.Fprintf(w, "  - error: %s\n", item.Error); err != nil {
				return err
			}
		}
	}
	return nil
}
