package editorapp

import (
	"os/exec"
	"runtime"
	"testing"
	"time"

	gioapp "gioui.org/app"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// recordAttention swaps the platform hooks for counters.
func recordAttention(t *testing.T) (requests, clears *int) {
	t.Helper()
	requests, clears = new(int), new(int)
	oldRequest, oldClear := attentionRequest, attentionClear
	attentionRequest = func(gioapp.ViewEvent) { *requests++ }
	attentionClear = func(gioapp.ViewEvent) { *clears++ }
	t.Cleanup(func() { attentionRequest, attentionClear = oldRequest, oldClear })
	return requests, clears
}

func TestAttentionOnlyWhenEnabledAndUnfocused(t *testing.T) {
	for _, tc := range []struct {
		name             string
		enabled, focused bool
		want             int
	}{
		{"disabled", false, false, 0},
		{"focused", true, true, 0},
		{"unfocused", true, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, _ := recordAttention(t)
			a := attention{enabled: tc.enabled, valid: true}
			a.setFocused(tc.focused)
			a.bell()
			if *requests != tc.want {
				t.Fatalf("requests = %d, want %d", *requests, tc.want)
			}
		})
	}
}

func TestAttentionArmedOnlyWhileABellWouldRaise(t *testing.T) {
	recordAttention(t)
	if (&attention{valid: true}).armed() {
		t.Fatal("armed while disabled")
	}
	a := attention{enabled: true, valid: true}
	a.setFocused(true)
	if a.armed() {
		t.Fatal("armed while focused")
	}
	a.setFocused(false)
	if !a.armed() {
		t.Fatal("not armed while unfocused")
	}
	a.bell()
	if a.armed() {
		t.Fatal("armed while attention is already raised")
	}
	a.setFocused(true)
	a.setFocused(false)
	if !a.armed() {
		t.Fatal("not armed after focus cleared the request")
	}
}

func TestAttentionNeedsAValidView(t *testing.T) {
	requests, _ := recordAttention(t)
	a := attention{enabled: true}
	a.bell()
	if *requests != 0 {
		t.Fatalf("requests = %d without a view, want 0", *requests)
	}
}

func TestAttentionClearedOnceOnFocus(t *testing.T) {
	requests, clears := recordAttention(t)
	a := attention{enabled: true, valid: true}

	a.setFocused(true)
	if *clears != 0 {
		t.Fatalf("clears = %d with nothing raised, want 0", *clears)
	}
	a.setFocused(false)
	a.bell()
	a.bell()
	a.setFocused(true)
	a.setFocused(false)
	a.setFocused(true)
	if *requests != 2 || *clears != 1 {
		t.Fatalf("requests, clears = %d, %d; want 2, 1", *requests, *clears)
	}
}

// TestFlushBellsDeliversAHeldTerminalBell rings a bell from a :terminal job
// that prints nothing else. Nvim holds such a bell until its next screen
// update, so without flushBells it never reaches the state while idle.
func TestFlushBellsDeliversAHeldTerminalBell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the terminal job needs sh")
	}
	path, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim not found on PATH")
	}
	proc, err := nvimproc.Spawn(path, []string{"-u", "NONE", "-n", "--cmd", "set belloff="}, nil, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	recordAttention(t)
	a := &App{proc: proc, state: uistate.New(), attention: attention{enabled: true, valid: true}}
	done := make(chan struct{})
	go func() {
		a.flushBells()
		close(done)
	}()

	if err := proc.Nvim.Command(`enew | call termopen(['sh', '-c', 'sleep 0.5; printf "\a"; sleep 30'])`); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for rang := false; !rang; {
		select {
		case batch := <-proc.Redraw:
			a.state.Apply(batch)
			rang = a.state.TakeBell()
		case <-deadline:
			t.Fatal("held terminal bell never reached the UI")
		}
	}

	_ = proc.Nvim.Close()
	<-proc.Exited
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("flushBells kept running after Nvim exited")
	}
}
