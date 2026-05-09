# KinClaw — build + sign helpers.
#
# macOS TCC (Screen Recording, Accessibility) keys permission off the
# binary's code-signing identifier. `go build` stamps a fresh linker-
# signed identity every time, which means a fresh TCC entry every
# rebuild. Giving the binary a stable adhoc identifier fixes that.

BINARY     := kinclaw
IDENTIFIER := com.localkinai.kinclaw

.PHONY: all build sign cli run run-claude test vet smoke tier3 bench bench-record warmup verify tcc-reset clean help

all: cli

build:                     ## go build -o ./kinclaw ./cmd/kinclaw
	go build -o $(BINARY) ./cmd/kinclaw

sign: build                ## re-sign binary with stable identifier (keeps TCC grants)
	@codesign --force --sign - --identifier $(IDENTIFIER) $(BINARY)
	@echo "→ signed $(BINARY) as $(IDENTIFIER)"
	@codesign -dv $(BINARY) 2>&1 | grep -E "Identifier|Signature"

cli: sign                  ## build + sign (alias for sign)

run: cli                   ## build + sign + launch the Kimi pilot
	./$(BINARY) -soul souls/pilot_kimi.soul.md

run-claude: cli            ## build + sign + launch the Claude pilot
	./$(BINARY) -soul souls/pilot.soul.md

test:                      ## go test ./...
	go test -count=1 ./...

vet:                       ## go vet ./...
	go vet ./...

smoke:                     ## tests/smoke.sh — Tier 0+1, no permission, ~30s
	@./tests/smoke.sh

tier3:                     ## tests/tier3.sh — drive Safari/TextEdit via -exec (interactive)
	@./tests/tier3.sh

bench: cli                 ## Run macbench (369 macOS task slots) against this kinclaw build
	@if [ ! -d ../macbench ]; then \
	  echo "✗ macbench not found at ../macbench"; \
	  echo "  clone it: git clone https://github.com/LocalKinAI/macbench ../macbench"; \
	  exit 2; \
	fi
	@if [ -z "$$SKIP_WARMUP" ]; then \
	  echo "→ kinclaw warmup (TCC + brain + kits health) — SKIP_WARMUP=1 to bypass"; \
	  ./scripts/warmup.sh || { echo "✗ kinclaw warmup failed; abort"; exit 1; }; \
	  echo ""; \
	fi
	@cd ../macbench && $(MAKE) bench \
	  AGENT="$(CURDIR)/$(BINARY)" \
	  AGENT_ARGS='-soul $(CURDIR)/souls/macbench.soul.md -exec {prompt}' \
	  $(if $(TASKS),TASKS=$(TASKS)) \
	  $(if $(TIMEOUT),TIMEOUT=$(TIMEOUT))

bench-record: cli          ## Run macbench with mp4 recording (kinrec required)
	@if [ ! -d ../macbench ]; then \
	  echo "✗ macbench not found at ../macbench"; \
	  echo "  clone it: git clone https://github.com/LocalKinAI/macbench ../macbench"; \
	  exit 2; \
	fi
	@if [ -z "$$SKIP_WARMUP" ]; then ./scripts/warmup.sh || exit 1; fi
	@cd ../macbench && $(MAKE) bench-record \
	  AGENT="$(CURDIR)/$(BINARY)" \
	  AGENT_ARGS='-soul $(CURDIR)/souls/macbench.soul.md -exec {prompt}' \
	  $(if $(TASKS),TASKS=$(TASKS)) \
	  $(if $(TIMEOUT),TIMEOUT=$(TIMEOUT))

warmup:                    ## Pre-flight: build + sign + verify TCC + brain + kits
	@./scripts/warmup.sh

verify: cli smoke          ## build + smoke (fastest "did I break anything" gate)
	@echo "→ smoke green; for end-to-end: cd ../kinclaw-mac && make kill && make run"

tcc-reset:                 ## reset macOS TCC grants for this binary (forces re-prompt)
	tccutil reset Accessibility   $(IDENTIFIER) || true
	tccutil reset ScreenCapture   $(IDENTIFIER) || true
	@echo "→ TCC reset for $(IDENTIFIER)"

clean:                     ## remove binary
	rm -f $(BINARY)

help:                      ## print this help
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?##"}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
