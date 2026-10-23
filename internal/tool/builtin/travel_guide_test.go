package builtin

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTravelGuideEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.md")
	raw := "\ufeff# 自制攻略\r\n\r\n杭州 雨天 Museum\r\n先核对预约。\r\n\r\n杭州 公园\r\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	g, err := NewTravelGuide(path)
	if err != nil {
		t.Fatal(err)
	}
	// Replacing the file must not silently change the source/version during a run.
	if err := os.WriteFile(path, []byte("新版本"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query string
		count int
	}{{"杭州 museum", 1}, {"杭州", 2}, {"雨天 公园", 0}, {"不存在", 0}} {
		result, err := g.Execute(context.Background(), map[string]interface{}{"query": tc.query})
		var evidence struct {
			Source, SHA256 string
			Matches        []guidePassage
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(result), &evidence); err != nil {
			t.Fatal(err)
		}
		if evidence.Source != "guide.md" || evidence.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) || len(evidence.Matches) != tc.count || strings.Contains(result, filepath.Dir(path)) {
			t.Fatalf("bad evidence: %s", result)
		}
		if tc.count == 1 && (evidence.Matches[0].StartLine != 3 || evidence.Matches[0].EndLine != 4 || evidence.Matches[0].Text != "杭州 雨天 Museum\n先核对预约。") {
			t.Fatalf("bad citation: %s", result)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.Execute(ctx, map[string]interface{}{"query": "杭州"}); err != context.Canceled {
		t.Fatalf("cancel: %v", err)
	}
	for _, params := range []map[string]interface{}{{}, {"query": 1}, {"query": " "}, {"query": strings.Repeat("中", 201)}, {"query": "a b c d e f g"}, {"query": "杭州", "path": "other.md"}} {
		if _, err := g.Execute(context.Background(), params); err == nil {
			t.Fatalf("accepted %v", params)
		}
	}
}

func TestTravelGuideBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.txt")
	for _, content := range []string{"", " \n\n", strings.Repeat("x", 4097), strings.Repeat("x\n\n", 22000), "bad\x00text", "\xff"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewTravelGuide(path); err == nil {
			t.Fatalf("accepted invalid content, length %d", len(content))
		}
	}
	if _, err := NewTravelGuide(filepath.Dir(path)); err == nil {
		t.Fatal("accepted directory")
	}
	if _, err := NewTravelGuide(path + "missing"); err == nil {
		t.Fatal("accepted missing file")
	}
	if err := os.WriteFile(path, []byte("词 一\n\n词 二\n\n词 三\n\n词 四"), 0600); err != nil {
		t.Fatal(err)
	}
	g, err := NewTravelGuide(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.Execute(context.Background(), map[string]interface{}{"query": "词"})
	if err != nil || !strings.Contains(result, `"more_matches":true`) || strings.Contains(result, "词 四") || !strings.Contains(result, "词 三") {
		t.Fatalf("bound: %s %v", result, err)
	}
}
