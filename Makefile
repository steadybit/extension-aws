# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## licenses-report: generate a report of all licenses
.PHONY: licenses-report
licenses-report:
ifeq ($(SKIP_LICENSES_REPORT), true)
	@echo "Skipping licenses report"
	rm -rf ./licenses && mkdir -p ./licenses
else
	@echo "Generating licenses report"
	rm -rf ./licenses
	go run github.com/google/go-licenses@v1.6.0 save . --save_path ./licenses
	go run github.com/google/go-licenses@v1.6.0 report . > ./licenses/THIRD-PARTY.csv
	cp LICENSE ./licenses/LICENSE.txt
endif

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
	go test -race -vet=off -coverprofile=coverage.out ./...
	go mod verify

## charttesting: Run Helm chart unit tests
.PHONY: charttesting
charttesting:
	@set -e; \
	for dir in charts/steadybit-extension-*; do \
		echo "Unit Testing $$dir"; \
		helm unittest $$dir; \
	done

## chartlint: Lint charts
.PHONY: chartlint
chartlint:
	ct lint --config chartTesting.yaml

## chart-bump-version: Bump the patch version and optionally set the appVersion
.PHONY: chart-bump-version
chart-bump-version:
	@set -e; \
	for dir in charts/steadybit-extension-*; do \
		if [ ! -z "$(APP_VERSION)" ]; then \
					yq -i ".appVersion = strenv(APP_VERSION)" $$dir/Chart.yaml; \
		fi; \
		CHART_VERSION=$$(semver -i patch $$(yq '.version' $$dir/Chart.yaml)) \
		yq -i ".version = strenv(CHART_VERSION)" $$dir/Chart.yaml; \
		grep -e "^version:" -e "^appVersion:" $$dir/Chart.yaml; \
	done
# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build: build the extension
.PHONY: build
build:
	goreleaser build --clean --snapshot --single-target -o extension

## run: run the extension
.PHONY: run
run: tidy build
	./extension

## container: build the container image
.PHONY: container
container:
	docker build --build-arg ADDITIONAL_BUILD_PARAMS="-cover -covermode=atomic" --build-arg BUILD_WITH_COVERAGE="true" --build-arg SKIP_LICENSES_REPORT="true" -t extension-aws:latest .

## linuxpkg: build the linux packages
.PHONY: linuxpkg
linuxpkg:
	goreleaser release --clean --snapshot
