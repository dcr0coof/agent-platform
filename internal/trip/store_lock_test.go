package trip

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dcr0coof/agent-platform/internal/llm"
)

// Use a separate process: an in-process mutex would not protect server restarts.
func TestStoreExclusiveOwner(t *testing.T) {
	if path := os.Getenv("TRIP_TEST_OWNER_DB"); path != "" {
		s, err := OpenStore(path)
		if err != nil {
			t.Fatal(err)
		}
		ss, err := s.Create("owner", "trip", Constraints{})
		if err != nil {
			t.Fatal(err)
		}
		r, _, err := s.Start("owner", ss.ID, "first", "saved", 1)
		if err != nil {
			t.Fatal(err)
		}
		history := []llm.Message{{Role: llm.RoleUser, Content: "first"}, {Role: llm.RoleAssistant, Content: "reply"}}
		if err = s.Finish(r.ID, "completed", "done", history); err != nil {
			t.Fatal(err)
		}
		pending, _, err := s.Start("owner", ss.ID, "second", "pending", 2)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(pending.ID); err != nil {
			t.Fatal(err)
		}
		// Parent attempts a competing open before releasing us through EOF.
		if _, err = io.Copy(io.Discard, os.Stdin); err != nil {
			t.Fatal(err)
		}
		active, err := s.Run("owner", pending.ID)
		if err != nil || active.Status != "running" || len(active.Events) != 1 {
			t.Fatalf("competing open changed active run: %+v, %v", active, err)
		}
		if err = s.AppendEvent(pending.ID, "test.owner_alive", "still writable"); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("TRIP_TEST_OWNER_EXIT") == "close" {
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
		}
		// In crash mode deliberately bypass Close and all Go defers.
		os.Exit(0)
	}
	for _, mode := range []string{"close", "crash"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trips.db")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStoreExclusiveOwner$")
			cmd.Env = append(os.Environ(), "TRIP_TEST_OWNER_DB="+path, "TRIP_TEST_OWNER_EXIT="+mode)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				stdin.Close()
				if err := cmd.Wait(); err != nil {
					t.Errorf("owner process failed: %v: %s", err, stderr.String())
				}
			}()
			var runID string
			if err = json.NewDecoder(stdout).Decode(&runID); err != nil {
				t.Fatal(err)
			}
			other, openErr := OpenStore(path)
			if openErr == nil {
				other.Close()
				t.Fatal("second process opened the active owner's database")
			}
			if !strings.Contains(openErr.Error(), "SQLITE_BUSY") {
				t.Fatalf("expected database lock error, got %v", openErr)
			}
			stdin.Close()
			// EOF means the child has checked its run and exited. Drain its output
			// before reopening; Wait is still responsible for checking exit status.
			if _, err = io.Copy(io.Discard, stdout); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenStore(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			recovered, err := reopened.Run("owner", runID)
			if err != nil || recovered.Status != "failed" || len(recovered.Events) != 3 || recovered.Events[2].Type != "run.failed" {
				t.Fatalf("orphan recovery failed: %+v, %v", recovered, err)
			}
			ss, err := reopened.Get("owner", recovered.SessionID)
			if err != nil || len(ss.Messages) != 2 || ss.Messages[1].Content != "reply" || ss.ActiveRun != nil {
				t.Fatalf("saved history changed after %s: %+v, %v", mode, ss, err)
			}
		})
	}
}
