.PHONY: test fmt cov tidy run lint fix build upgrade-tools

COVFILE = coverage.out
COVHTML = cover.html
GITHUB_REPOSITORY = koh-sh/commd

test:
	go test ./... -json | tparse -all

fmt:
	gofumpt -l -w .

cov:
	go test -cover ./... -coverprofile=$(COVFILE)
	go tool cover -html=$(COVFILE) -o $(COVHTML)
	CI=1 GITHUB_REPOSITORY=$(GITHUB_REPOSITORY) octocov
	rm $(COVFILE)

tidy:
	go mod tidy -v

lint:
	golangci-lint run --fix

build:
	go build -o commd .

ci: fmt fix lint build cov

# Go Fix (modernize)
fix:
	go fix ./...

# Upgrade dev tools managed by mise to latest versions
upgrade-tools:
	mise up
