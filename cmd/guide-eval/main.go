// guide-eval measures the local lexical retriever, not model-generated answers.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/dcr0coof/agent-platform/internal/tool/builtin"
)

type queryCase struct {
	ID       string `json:"id"`
	Query    string `json:"query"`
	Expected []int  `json:"expected_start_lines"`
	More     bool   `json:"expected_more_matches"`
}
type suite struct {
	GuideSHA string      `json:"guide_sha256"`
	Cases    []queryCase `json:"cases"`
}
type caseResult struct {
	queryCase
	Actual []int `json:"actual_start_lines"`
	More   bool  `json:"actual_more_matches"`
	Exact  bool  `json:"exact"`
}
type report struct {
	Strategy       string       `json:"strategy"`
	GuideSHA       string       `json:"guide_sha256"`
	SuiteSHA       string       `json:"suite_sha256"`
	Total          int          `json:"total"`
	Exact          int          `json:"exact"`
	NoEvidence     int          `json:"no_evidence_cases"`
	FalsePositives int          `json:"no_evidence_false_positives"`
	Cases          []caseResult `json:"cases"`
}

func evaluate(guidePath, suitePath string) (report, error) {
	f, err := os.Open(suitePath)
	if err != nil {
		return report{}, fmt.Errorf("cannot open evaluation suite")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if err != nil || len(data) > 256<<10 {
		return report{}, fmt.Errorf("suite exceeds 256 KiB or cannot be read")
	}
	var s suite
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return report{}, fmt.Errorf("invalid suite JSON: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return report{}, fmt.Errorf("suite must be one JSON document")
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s.GuideSHA) || len(s.Cases) == 0 || len(s.Cases) > 200 {
		return report{}, fmt.Errorf("suite requires a lowercase guide SHA256 and 1–200 cases")
	}
	ids := map[string]bool{}
	for _, c := range s.Cases {
		if strings.TrimSpace(c.ID) == "" || ids[c.ID] || c.Expected == nil || len(c.Expected) > 3 || (c.More && len(c.Expected) != 3) {
			return report{}, fmt.Errorf("case IDs must be unique/nonblank; expected lines must be an array of up to three lines")
		}
		ids[c.ID] = true
		last := 0
		for _, line := range c.Expected {
			if line <= last {
				return report{}, fmt.Errorf("expected lines must be positive and strictly increasing")
			}
			last = line
		}
	}
	g, err := builtin.NewTravelGuide(guidePath)
	if err != nil {
		return report{}, err
	}
	r := report{Strategy: "literal-and-source-order-v1", GuideSHA: s.GuideSHA, SuiteSHA: fmt.Sprintf("%x", sha256.Sum256(data)), Total: len(s.Cases), Cases: make([]caseResult, 0, len(s.Cases))}
	for _, c := range s.Cases {
		raw, err := g.Execute(context.Background(), map[string]interface{}{"query": c.Query})
		if err != nil {
			return report{}, fmt.Errorf("case %s: %w", c.ID, err)
		}
		var result struct {
			SHA     string `json:"sha256"`
			Matches []struct {
				Start int `json:"start_line"`
			} `json:"matches"`
			More bool `json:"more_matches"`
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return report{}, err
		}
		if result.SHA != s.GuideSHA {
			return report{}, fmt.Errorf("guide SHA256 differs from suite; do not score against changed evidence")
		}
		actual := make([]int, 0, len(result.Matches))
		for _, m := range result.Matches {
			actual = append(actual, m.Start)
		}
		exact := slices.Equal(actual, c.Expected) && result.More == c.More
		if exact {
			r.Exact++
		}
		if len(c.Expected) == 0 {
			r.NoEvidence++
			if len(actual) > 0 {
				r.FalsePositives++
			}
		}
		r.Cases = append(r.Cases, caseResult{c, actual, result.More, exact})
	}
	return r, nil
}

// Exit 0: all expectations match; 1: quality mismatch with report; 2: invalid run.
func run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("guide-eval", flag.ContinueOnError)
	fs.SetOutput(errOut)
	guide := fs.String("guide", "", "explicit corpus file")
	suitePath := fs.String("suite", "", "fixed evaluation JSON")
	if fs.Parse(args) != nil {
		return 2
	}
	if *guide == "" || *suitePath == "" || fs.NArg() != 0 {
		fmt.Fprintln(errOut, "provide -guide and -suite only")
		return 2
	}
	r, err := evaluate(*guide, *suitePath)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	if err := e.Encode(r); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if r.Exact != r.Total {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
