# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000,-ST1003 ./...
	go test -race -vet=off ./...
	go mod verify

## charttesting: Run Helm chart unit tests
.PHONY: charttesting
charttesting:
	for dir in charts/steadybit-extension-*; do \
    echo "Unit Testing $$dir"; \
    helm unittest $$dir; \
  done

# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build: build the extension
.PHONY: build
build:
	go mod verify
	go build -o=./extension

## run: run the extension
.PHONY: run
run: tidy build
	./extension

## container: build the container image
.PHONY: container
container:
	docker build -t extension-scaffold:latest .

# ==================================================================================== #
# EJECT
# ==================================================================================== #

## eject: remove / clear up files associated with the scaffold repository
.PHONY: eject
eject:
	rm CHANGELOG.md
	mv CHANGELOG.SCAFFOLD.md CHANGELOG.md
	rm CONTRIBUTING.md
	mv CONTRIBUTING.SCAFFOLD.md CONTRIBUTING.md
	rm README.md
	mv README.SCAFFOLD.md README.md
	rm LICENSE
	mv LICENSE.SCAFFOLD LICENSE
