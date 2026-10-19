.PHONY: help build graph clean test e2e-elevator e2e-engine

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

test: ## Run tests
	go test ./...
	@echo "✓ Tests finished"

e2e-elevator: ## Drive the elevator in a real Chrome (serial, ~1.5 min)
	cd e2e && go test -count=1 -v -run Elevator .

e2e-engine: ## Drive the combustion engine in a real Chrome (serial, ~1 min)
	cd e2e && go test -count=1 -v -run Engine .