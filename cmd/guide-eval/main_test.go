package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluationReportsQualityAndRejectsInvalidRuns(t *testing.T) {
	dir := t.TempDir()
	guide, cases := filepath.Join(dir, "guide.md"), filepath.Join(dir, "cases.json")
	corpus := []byte("杭州雨天\n\n上海雨天\n")
	if err := os.WriteFile(guide, corpus, 0600); err != nil {
		t.Fatal(err)
	}
	valid := suite{GuideSHA: fmtHash(corpus), Cases: []queryCase{{ID: "hangzhou", Query: "杭州", Expected: []int{1}}}}
	for _, tc := range []struct {
		name   string
		modify func(*suite)
		code   int
	}{
		{"pass", func(s *suite) {}, 0},
		{"false_positive", func(s *suite) { s.Cases[0].Expected = []int{} }, 1},
		{"missing", func(s *suite) { s.Cases[0].Query = "下雨" }, 1},
		{"wrong_hash", func(s *suite) { s.GuideSHA = strings.Repeat("0", 64) }, 2},
		{"duplicate_ids", func(s *suite) { s.Cases = append(s.Cases, s.Cases[0]) }, 2},
		{"missing_expectations", func(s *suite) { s.Cases[0].Expected = nil }, 2},
		{"unordered_lines", func(s *suite) { s.Cases[0].Expected = []int{3, 1} }, 2},
		{"invalid_query", func(s *suite) { s.Cases[0].Query = "" }, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := valid
			s.Cases = append([]queryCase(nil), valid.Cases...)
			tc.modify(&s)
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cases, data, 0600); err != nil {
				t.Fatal(err)
			}
			var out, errors bytes.Buffer
			if got := run([]string{"-guide", guide, "-suite", cases}, &out, &errors); got != tc.code {
				t.Fatalf("exit=%d want=%d: %s", got, tc.code, errors.String())
			}
			if tc.code == 2 {
				if out.Len() != 0 || errors.Len() == 0 {
					t.Fatal("invalid run must not emit quality report")
				}
				return
			}
			var r report
			if err := json.Unmarshal(out.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			if r.Total != 1 || r.SuiteSHA != fmtHash(data) || r.GuideSHA != valid.GuideSHA {
				t.Fatalf("wrong provenance: %+v", r)
			}
			if tc.name == "false_positive" && (r.Exact != 0 || r.NoEvidence != 1 || r.FalsePositives != 1) {
				t.Fatalf("false positive hidden: %+v", r)
			}
			if tc.name == "pass" && r.Exact != 1 {
				t.Fatal("expected exact result")
			}
		})
	}
	for _, malformed := range []string{`{} {}`, `{"extra":1}`, strings.Repeat(" ", (256<<10)+1)} {
		if err := os.WriteFile(cases, []byte(malformed), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := evaluate(guide, cases); err == nil {
			t.Fatal("invalid suite accepted")
		}
	}
}

func fmtHash(data []byte) string { hash := sha256.Sum256(data); return fmt.Sprintf("%x", hash) }
