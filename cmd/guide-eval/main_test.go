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
	valid := suite{GuideSHA: fmtHash(corpus), Cases: []queryCase{{ID: "hangzhou", Query: "杭州", Expected: []int{1}, More: new(bool)}}}
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

func TestSuiteUTF8Validation(t *testing.T) {
	dir := t.TempDir()
	guide, cases := filepath.Join(dir, "guide.md"), filepath.Join(dir, "cases.json")
	corpus := []byte("杭州 �")
	if err := os.WriteFile(guide, corpus, 0600); err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(suite{GuideSHA: fmtHash(corpus), Cases: []queryCase{{ID: "unicode", Query: "�", Expected: []int{1}, More: new(bool)}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{{0xff}, {0xc0, 0xaf}, {0xed, 0xa0, 0x80}, {0xe4, 0xb8}, []byte("�")} {
		data := bytes.Replace(valid, []byte("�"), raw, 1)
		if err := os.WriteFile(cases, data, 0600); err != nil {
			t.Fatal(err)
		}
		var out, errors bytes.Buffer
		code := run([]string{"-guide", guide, "-suite", cases}, &out, &errors)
		if bytes.Equal(raw, []byte("�")) {
			if code != 0 || out.Len() == 0 {
				t.Fatalf("valid Unicode rejected: %d %s", code, &errors)
			}
		} else if code != 2 || out.Len() != 0 || !strings.Contains(errors.String(), "UTF-8") {
			t.Fatalf("damaged suite scored: %d %s %s", code, &out, &errors)
		}
	}
}

func TestExplicitExclusionTypes(t *testing.T) {
	dir := t.TempDir()
	guide, cases := filepath.Join(dir, "guide.md"), filepath.Join(dir, "cases.json")
	corpus := []byte("杭州 博物馆")
	if err := os.WriteFile(guide, corpus, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		field string
		code  int
	}{
		{``, 1}, {`,"exclude":""`, 1}, {`,"exclude":"博物馆"`, 0},
		{`,"exclude":null`, 2}, {`,"exclude":false`, 2}, {`,"exclude":[]`, 2},
	} {
		data := fmt.Sprintf(`{"guide_sha256":%q,"cases":[{"id":"x","query":"杭州","expected_start_lines":[],"expected_more_matches":false%s}]}`, fmtHash(corpus), tc.field)
		if err := os.WriteFile(cases, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errors bytes.Buffer
		if code := run([]string{"-guide", guide, "-suite", cases}, &out, &errors); code != tc.code {
			t.Fatalf("%s: exit %d: %s", tc.field, code, errors.String())
		}
		if tc.code == 2 && (out.Len() != 0 || errors.Len() == 0) {
			t.Fatal("invalid constraint emitted report")
		}
	}
}

func TestMoreExpectationMustBeExplicit(t *testing.T) {
	dir := t.TempDir()
	guide, cases := filepath.Join(dir, "guide.md"), filepath.Join(dir, "cases.json")
	corpus := []byte("杭州一\n\n杭州二\n\n杭州三\n\n杭州四")
	if err := os.WriteFile(guide, corpus, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		field string
		code  int
	}{
		{``, 2}, {`,"expected_more_matches":null`, 2}, {`,"expected_more_matches":"false"`, 2},
		{`,"expected_more_matches":false`, 1}, {`,"expected_more_matches":true`, 0},
	} {
		data := fmt.Sprintf(`{"guide_sha256":%q,"cases":[{"id":"x","query":"杭州","expected_start_lines":[1,3,5]%s}]}`, fmtHash(corpus), tc.field)
		if err := os.WriteFile(cases, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errors bytes.Buffer
		if code := run([]string{"-guide", guide, "-suite", cases}, &out, &errors); code != tc.code {
			t.Fatalf("%s: %d %s", tc.field, code, &errors)
		}
		if tc.code == 2 && (out.Len() != 0 || errors.Len() == 0) {
			t.Fatal("invalid expectation emitted score")
		}
	}
}

func TestPassageDiagnostics(t *testing.T) {
	dir := t.TempDir()
	guide, cases := filepath.Join(dir, "guide.md"), filepath.Join(dir, "cases.json")
	corpus := []byte("杭州\n\n杭州雨天\n\n上海\n")
	if err := os.WriteFile(guide, corpus, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, query                   string
		expected, missing, unexpected []int
	}{
		{"exact", "杭州", []int{1, 3}, []int{}, []int{}},
		{"mixed", "杭州", []int{3, 5}, []int{5}, []int{1}},
		{"no evidence", "北京", []int{1}, []int{1}, []int{}},
		{"false positive", "杭州", []int{}, []int{}, []int{1, 3}},
		{"empty", "北京", []int{}, []int{}, []int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(suite{GuideSHA: fmtHash(corpus), Cases: []queryCase{{ID: tc.name, Query: tc.query, Expected: tc.expected, More: new(bool)}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cases, data, 0600); err != nil {
				t.Fatal(err)
			}
			var out, errors bytes.Buffer
			code := run([]string{"-guide", guide, "-suite", cases}, &out, &errors)
			wantCode := 0
			if len(tc.missing)+len(tc.unexpected) > 0 {
				wantCode = 1
			}
			if code != wantCode {
				t.Fatalf("exit %d: %s", code, errors.String())
			}
			var result struct {
				Cases []struct {
					Missing    []int `json:"missing_start_lines"`
					Unexpected []int `json:"unexpected_start_lines"`
				}
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			got := result.Cases[0]
			m, _ := json.Marshal(got.Missing)
			u, _ := json.Marshal(got.Unexpected)
			wm, _ := json.Marshal(tc.missing)
			wu, _ := json.Marshal(tc.unexpected)
			if string(m) != string(wm) || string(u) != string(wu) {
				t.Fatalf("missing=%s unexpected=%s", m, u)
			}
		})
	}
}
