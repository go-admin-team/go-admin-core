GO ?= go

# Integration tests that need a live MySQL are guarded by testing.Short(),
# so every target below runs with -short. Nothing is excluded from the race
# target; anything that has to be would belong in docs/known-issues.md.

.PHONY: all build vet test test-race bench bench-compare soak lint vuln tidy api-snapshot api-check ci

all: ci

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test -short ./...

test-race:
	$(GO) test -short -race -timeout 600s ./...

# Pinned, because a gate whose verdict can change without a code change is not
# a gate. Analysed for the host and for linux: a file behind a build tag is
# invisible to a run on another platform, which is how a finding in
# watcher_linux.go stayed hidden from every local run and failed in CI.
STATICCHECK ?= honnef.co/go/tools/cmd/staticcheck@v0.7.0

# bench runs every benchmark once, briefly. It is a smoke test that the
# benchmarks still compile and run - not a measurement. For numbers worth
# comparing, raise -benchtime and use benchstat over several runs.
#
# Redis benchmarks skip unless REDIS_URL points at a server.
bench:
	$(GO) test -run '^$$' -bench . -benchtime 10x -benchmem ./...

# bench-compare measures this tree against a base revision on one machine and
# runs the results through benchstat. Timings are not gated in CI - see the
# script's header - so this is the deliberate check to run when a change is
# meant to affect performance.
#
#	make bench-compare                          # against origin/main
#	make bench-compare BASE=HEAD~1 BENCH=Cache  # narrower
BASE ?= origin/main
BENCH ?= .
BENCH_COUNT ?= 6
bench-compare:
	scripts/bench-compare.sh $(BASE) $(BENCH) $(BENCH_COUNT)

# soak runs the tests that need time: sustained load, resource reclamation and
# goroutine lifecycles. They are skipped by -short, which is what test and
# test-race use, so nothing else in this Makefile covers them.
#
# GOADMIN_SOAK sets how long each one runs, e.g. GOADMIN_SOAK=2m make soak.
soak:
	$(GO) test -race -timeout 1800s -run 'Soak|Leak|Stall|Reclaim|Release' ./...

lint:
	@tmp=$$(mktemp -d); \
	GOBIN=$$tmp $(GO) install $(STATICCHECK) || { rm -rf $$tmp; exit 1; }; \
	targets="$$($(GO) env GOOS)"; \
	case "$$targets" in linux) ;; *) targets="$$targets linux";; esac; \
	status=0; \
	for os in $$targets; do \
		echo "staticcheck GOOS=$$os"; \
		GOOS=$$os $$tmp/staticcheck ./... || status=1; \
	done; \
	rm -rf $$tmp; \
	exit $$status

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

tidy:
	$(GO) mod tidy
	git diff --exit-code go.mod go.sum

api-snapshot:
	scripts/api-snapshot.sh

# Fails when the committed snapshot no longer matches the code, forcing API
# changes to surface in the pull request diff.
api-check: api-snapshot
	git diff --exit-code api/

ci: tidy build vet lint test-race api-check
