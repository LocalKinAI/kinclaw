# KinClaw — build + sign helpers.
#
# macOS TCC (Screen Recording, Accessibility) keys permission off the
# binary's code-signing identifier. `go build` stamps a fresh linker-
# signed identity every time, which means a fresh TCC entry every
# rebuild. Giving the binary a stable adhoc identifier fixes that.

BINARY     := kinclaw
IDENTIFIER := com.localkinai.kinclaw

.PHONY: all build sign cli run test vet clean help

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

tcc-reset:                 ## reset macOS TCC grants for this binary (forces re-prompt)
	tccutil reset Accessibility   $(IDENTIFIER) || true
	tccutil reset ScreenCapture   $(IDENTIFIER) || true
	@echo "→ TCC reset for $(IDENTIFIER)"

clean:                     ## remove binary
	rm -f $(BINARY)

help:                      ## print this help
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?##"}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
