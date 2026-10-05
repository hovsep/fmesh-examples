.PHONY: help build graph clean

# Find all examples (directories containing main.go)
EXAMPLES := $(shell find . -name "main.go" -type f | grep -v tools | sed 's|/main.go$$||' | sed 's|^\./||' | sort)

help: ## Show help
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build all examples
	@for ex in $(EXAMPLES); do \
		echo "Building $$ex..."; \
		(cd $$ex && go build .) || exit 1; \
	done
	@echo "✓ All examples built successfully"

graph: ## Generate graphs for all examples
	@for ex in $(EXAMPLES); do \
		echo "Generating graph for $$ex..."; \
		(cd $$ex && FMESH_GRAPH=1 go run . 2>/dev/null) || echo "  ⊘ Skipped"; \
	done

clean: ## Clean build artifacts
	@for ex in $(EXAMPLES); do (cd $$ex && go clean) || true; done
	@echo "✓ Clean complete"

# The life suite simulates hours of physiology: ~15 minutes locally and over 30
# on a hosted CI runner, far past go test's 10-minute per-package default, so it
# gets the same limit as CI.
test: ## Run tests
	go test -timeout 60m ./...
	@echo "✓ Tests finished"