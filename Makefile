# FMesh Examples - Minimal Makefile

.PHONY: help list build-all graph-all run-all check clean

# Find all examples (directories containing main.go)
EXAMPLES := $(shell find . -name "main.go" -type f | grep -v tools | sed 's|/main.go$$||' | sed 's|^\./||' | sort)

# Interactive examples and their input
INTERACTIVE_EXAMPLES := pipeline
PIPELINE_INPUT := "hello world this is a test input for the pipeline example"

help: ## Show help
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

list: ## List all examples
	@for ex in $(EXAMPLES); do echo "  $$ex"; done

build-all: ## Build all examples
	@for ex in $(EXAMPLES); do \
		echo "Building $$ex..."; \
		(cd $$ex && go build .) || exit 1; \
	done
	@echo "✓ All examples built successfully"

graph-all: ## Generate graphs for all examples
	@for ex in $(EXAMPLES); do \
		echo "Generating graph for $$ex..."; \
		(cd $$ex && go run . --graph 2>/dev/null) || echo "  ⊘ Skipped"; \
	done

run-all: ## Run all examples
	@for ex in $(EXAMPLES); do \
		skip=0; \
		for s in $(INTERACTIVE_EXAMPLES); do \
			if [ "$$ex" = "$$s" ]; then skip=1; break; fi; \
		done; \
		if [ $$skip -eq 1 ]; then \
			echo "Running $$ex (with input)..."; \
			echo $(PIPELINE_INPUT) | (cd $$ex && go run .) || echo "  ⚠ Failed"; \
		else \
			echo "Running $$ex..."; \
			(cd $$ex && go run .) || echo "  ⚠ Failed"; \
		fi; \
	done

check: build-all graph-all ## Build and generate graphs

clean: ## Clean build artifacts and .tmp/ directory
	@for ex in $(EXAMPLES); do (cd $$ex && go clean) || true; done
	@rm -rf .tmp 2>/dev/null || true
	@echo "✓ Clean complete"
