package runtime

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/logger"
)

// AfterResource runs every registered callback again on every round, not the
// ones registered since the last one. A configuration reload rebuilds the
// database, the cache and the queue, and the hooks that have to attach
// themselves to the new ones are exactly the hooks that already ran.
func TestReentrantPhaseRunsEveryCallbackOnEveryRound(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var first, second int
	app.SetPhase(AfterResource, func() { first++ })
	app.SetPhase(AfterResource, func() { second++ })

	for i := 0; i < 3; i++ {
		app.RunPhase(AfterResource)
	}

	if first != 3 || second != 3 {
		t.Errorf("callbacks ran %d and %d times over three rounds, want 3 and 3", first, second)
	}
}

// The phase never closes, so a module that registers late is not refused -
// and nothing is logged about it, because it is not a mistake.
func TestReentrantPhaseNeverSeals(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	app.RunPhase(AfterResource)
	if app.PhaseSealed(AfterResource) {
		t.Error("AfterResource reports sealed; it is re-entrant and never closes")
	}

	late := 0
	app.SetPhase(AfterResource, func() { late++ })
	if errors := rec.only(logger.ErrorLevel); len(errors) != 0 {
		t.Errorf("registering into a re-entrant phase after a round is not late, got %v", errors)
	}

	app.RunPhase(AfterResource)
	if late != 1 {
		t.Errorf("the late callback ran %d times, want 1", late)
	}
}

// Two rounds must never overlap. Without that, two reloads in quick
// succession run the same hook twice at once - which for the queue hook
// means two sets of consumers on one adapter.
//
// The overlap is forced rather than raced for: the first round is held
// inside the callback until the second trigger has been delivered.
func TestReentrantPhaseNeverRunsTwoRoundsAtOnce(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var inFlight, overlapped, calls int32
	entered := make(chan struct{}, 4)
	release := make(chan struct{})

	app.SetPhase(AfterResource, func() {
		if atomic.AddInt32(&inFlight, 1) != 1 {
			atomic.StoreInt32(&overlapped, 1)
		}
		atomic.AddInt32(&calls, 1)
		entered <- struct{}{}
		<-release
		atomic.AddInt32(&inFlight, -1)
	})

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		app.RunPhase(AfterResource)
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the first round never started")
	}

	// The second trigger must not wait for the round in flight: the caller
	// is the one goroutine that drives configuration reloads.
	mustReturn(t, "RunPhase(AfterResource) while a round is in flight", func() {
		app.RunPhase(AfterResource)
	})

	close(release)
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the rounds never finished")
	}

	if atomic.LoadInt32(&overlapped) != 0 {
		t.Error("two rounds ran the same callback at the same time")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("the callback ran %d times, want 2: the trigger that arrived during a round owes another round", got)
	}
}

// The trigger that arrives during a round is owed a round, not discarded.
// Discarding is the cheaper implementation and the dangerous one: the
// adapter built by the second reload may have been installed after the first
// round took its snapshot, so dropping the trigger leaves it with no
// consumers at all.
func TestATriggerDuringARoundIsNotDiscarded(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var rounds int32
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	var once sync.Once

	app.SetPhase(AfterResource, func() {
		atomic.AddInt32(&rounds, 1)
		entered <- struct{}{}
		// Only the first round blocks; the later ones must be free to run.
		once.Do(func() { <-release })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.RunPhase(AfterResource)
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the first round never started")
	}

	// Three triggers arrive while the first round is held. They collapse
	// into one owed round - the point is that at least one survives.
	for i := 0; i < 3; i++ {
		app.RunPhase(AfterResource)
	}
	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the owed round never ran: the trigger was discarded")
	}
	if got := atomic.LoadInt32(&rounds); got < 2 {
		t.Errorf("the callback ran %d times; triggers that arrived during the round were dropped", got)
	}
}

// A callback registered from inside a callback waits for the next round.
// Sweeping it up at the end of the current one instead would never
// terminate for a hook that registers a hook on every run.
func TestACallbackRegisteredDuringARoundWaitsForTheNextOne(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var late int
	var once sync.Once
	app.SetPhase(AfterResource, func() {
		once.Do(func() {
			app.SetPhase(AfterResource, func() { late++ })
		})
	})

	mustReturn(t, "RunPhase(AfterResource) with a callback that registers one", func() {
		app.RunPhase(AfterResource)
	})
	if late != 0 {
		t.Errorf("the callback registered during the round ran %d times in that round, want 0", late)
	}

	app.RunPhase(AfterResource)
	if late != 1 {
		t.Errorf("the callback registered during the previous round ran %d times, want 1", late)
	}
}

// Growth across rounds is the only automatic signal that a hook is not
// idempotent: the registry never shrinks, and every entry runs every round,
// so a hook that registers a hook costs one more callback per reload for the
// life of the process and reports nothing on its own.
func TestGrowthAcrossRoundsIsWarnedAbout(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	app.SetPhase(AfterResource, func() {})
	app.RunPhase(AfterResource)
	if warns := rec.only(logger.WarnLevel); len(warns) != 0 {
		t.Errorf("the first round has nothing to compare against and must not warn, got %v", warns)
	}

	app.SetPhase(AfterResource, func() {})
	app.RunPhase(AfterResource)

	warns := rec.only(logger.WarnLevel)
	if len(warns) != 1 {
		t.Fatalf("want one warning about the registry growing, got %v", warns)
	}
	for _, want := range []string{"AfterResource", "2", "1"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("the warning must contain %q, got:\n%s", want, warns[0])
		}
	}

	app.RunPhase(AfterResource)
	if warns := rec.only(logger.WarnLevel); len(warns) != 1 {
		t.Errorf("a round that did not grow must not warn again, got %v", warns)
	}
}

// WithFatal on the re-entrant phase keeps its meaning - refusing to start
// when the database is unreachable is a fair thing to ask for, and silently
// downgrading it would be its own silent failure - but it is warned about at
// the registration site, because on the second round the process it exits is
// one that is already serving requests.
func TestWithFatalOnTheReentrantPhaseIsWarnedAboutAndKept(t *testing.T) {
	app := NewConfig()
	rec := captureLogs(t, app)

	previous := exit
	t.Cleanup(func() { exit = previous })
	var codes []int
	exit = func(code int) { codes = append(codes, code) }

	app.SetPhase(AfterResource, func() { panic("boom") }, WithFatal(), WithName("db-check"))

	warns := rec.only(logger.WarnLevel)
	if len(warns) != 1 {
		t.Fatalf("want one warning at the registration site, got %v", warns)
	}
	for _, want := range []string{"AfterResource", "db-check", "phase_reentrant_test.go", "WithFatal"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("the warning must contain %q, got:\n%s", want, warns[0])
		}
	}

	app.RunPhase(AfterResource)
	if len(codes) != 1 || codes[0] != 1 {
		t.Errorf("exit was called with %v, want exactly one call with 1: WithFatal must keep its meaning", codes)
	}
}

// The warning belongs to the re-entrant phase only. On a phase that happens
// once, WithFatal is the option working as documented and warning about it
// would train the reader to ignore the line.
func TestWithFatalOnAOnceOnlyPhaseIsNotWarnedAbout(t *testing.T) {
	for _, p := range onceOnlyPhases {
		app := NewConfig()
		rec := captureLogs(t, app)

		app.SetPhase(p, func() {}, WithFatal())

		if warns := rec.only(logger.WarnLevel); len(warns) != 0 {
			t.Errorf("%v warned about WithFatal, got %v", p, warns)
		}
	}
}

// The re-entrant path under -race: triggers from many goroutines while
// callbacks are being registered.
func TestReentrantPhaseIsRaceFree(t *testing.T) {
	app := NewConfig()
	captureLogs(t, app)

	var calls int64
	for i := 0; i < 3; i++ {
		app.SetPhase(AfterResource, func() { atomic.AddInt64(&calls, 1) })
	}

	const goroutines, iters = 12, 30
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				if (i+j)%4 == 0 {
					app.SetPhase(AfterResource, func() { atomic.AddInt64(&calls, 1) })
					continue
				}
				app.RunPhase(AfterResource)
			}
		}(i)
	}
	wg.Wait()

	if atomic.LoadInt64(&calls) == 0 {
		t.Error("no callback ran at all")
	}
}
