# Justfile for Huma Playground
# https://github.com/casey/just

set dotenv-load

PORT := env("PORT", "8080")
CONTAINER_RUNTIME := if `command -v podman 2>/dev/null || true` != "" { "podman" } else { "docker" }

@_:
    just --list

[group('build')]
build: build-app build-functions

[group('build')]
build-app:
    go build -v ./...

[group('build')]
build-functions:
    cd functions && GOWORK=off go build -v ./...

[group('build')]
clean:
    rm -f coverage.out coverage.html coverage-summary.txt \
        integration-coverage.out integration-coverage.html integration-coverage-summary.txt \
        firebase-debug.log firestore-debug.log ui-debug.log

[group('run')]
run:
    go run ./cmd/server

[group('run')]
run-port port=PORT:
    PORT={{ port }} go run ./cmd/server

[group('run')]
functions-run port="8080":
    cd functions && GOWORK=off FUNCTION_TARGET=Hello LOCAL_ONLY=true PORT={{ port }} go run ./cmd/server

[group('test')]
emulators:
    npx --yes firebase-tools@15.34.0 emulators:start --only auth,firestore --project demo-test-project

[group('test')]
test *args:
    just test-app {{ args }}
    just test-functions {{ args }}

[group('test')]
test-app *args:
    go test ./... {{ args }}

[group('test')]
test-functions *args:
    cd functions && GOWORK=off go test ./... {{ args }}

[group('test')]
test-verbose *args:
    just test-app -v {{ args }}
    just test-functions -v {{ args }}

[group('test')]
test-race: test-race-app test-race-functions

[group('test')]
test-race-app:
    go test -race ./...

[group('test')]
test-race-functions:
    cd functions && GOWORK=off go test -race ./...

[group('test')]
fuzz fuzz_time="5s":
    go test -run='^$' -fuzz='^FuzzDecodeCursor$' -fuzztime={{ fuzz_time }} ./internal/platform/pagination
    go test -run='^$' -fuzz='^FuzzParseLinkHeader$' -fuzztime={{ fuzz_time }} ./internal/service/github
    go test -run='^$' -fuzz='^FuzzParseLinkHeaderRoundTrip$' -fuzztime={{ fuzz_time }} ./internal/service/github

[group('test')]
test-integration-ci *args:
    REQUIRE_FIREBASE_EMULATORS=1 npx --yes firebase-tools@15.34.0 emulators:exec --only auth,firestore --project demo-test-project \
        "just test-app -count=1 -covermode=atomic -coverpkg=./... -coverprofile=integration-coverage.out {{ args }}"
    go tool cover -func=integration-coverage.out > integration-coverage-summary.txt
    go tool cover -html=integration-coverage.out -o integration-coverage.html

[group('test')]
functions-smoke port="18081":
    #!/usr/bin/env bash
    set -euo pipefail
    tmp="$(mktemp -d)"
    cleanup() {
      result=$?
      if [[ "$result" -ne 0 ]]; then
        [[ -f "$tmp/log" ]] && tail -n 200 "$tmp/log" >&2
        [[ -f "$tmp/response" ]] && tail -n 200 "$tmp/response" >&2
      fi
      if [[ -n "${pid:-}" ]]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
      rm -rf "$tmp"
    }
    trap cleanup EXIT
    (cd functions && GOWORK=off go build -o "$tmp/server" ./cmd/server)
    FUNCTION_TARGET=Hello LOCAL_ONLY=true PORT={{ port }} "$tmp/server" >"$tmp/log" 2>&1 &
    pid=$!
    for _ in {1..30}; do
      if curl --fail --silent "http://127.0.0.1:{{ port }}/?name=Smoke" >"$tmp/response"; then
        if grep -F '"message":"Hello, Smoke!"' "$tmp/response" >/dev/null; then exit 0; fi
      fi
      sleep 0.2
    done
    exit 1

[group('test')]
test-coverage *args:
    go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./... {{ args }}

[group('test')]
coverage: test-coverage
    go tool cover -func=coverage.out | tee coverage-summary.txt
    go tool cover -html=coverage.out -o coverage.html

[group('qa')]
lint: lint-app lint-functions

[group('qa')]
lint-app:
    golangci-lint run ./...

[group('qa')]
lint-functions:
    cd functions && GOWORK=off golangci-lint run ./...

[group('qa')]
fmt: fmt-app fmt-functions

[group('qa')]
fmt-app:
    golangci-lint fmt ./...

[group('qa')]
fmt-functions:
    cd functions && GOWORK=off golangci-lint fmt ./...

[group('qa')]
fmt-check: fmt-check-app fmt-check-functions

[group('qa')]
fmt-check-app:
    #!/usr/bin/env bash
    set -euo pipefail
    diff="$(golangci-lint fmt --diff ./...)"
    if [[ -n "$diff" ]]; then printf '%s\n' "$diff"; exit 1; fi

[group('qa')]
fmt-check-functions:
    #!/usr/bin/env bash
    set -euo pipefail
    diff="$(cd functions && GOWORK=off golangci-lint fmt --diff ./...)"
    if [[ -n "$diff" ]]; then printf '%s\n' "$diff"; exit 1; fi

[group('qa')]
fix: fix-app fix-functions

[group('qa')]
fix-app:
    golangci-lint run --fix ./...

[group('qa')]
fix-functions:
    cd functions && GOWORK=off golangci-lint run --fix ./...

[group('qa')]
vuln: vuln-app vuln-functions

[group('qa')]
vuln-app:
    go tool govulncheck ./...

[group('qa')]
vuln-functions:
    cd functions && GOWORK=off go tool -modfile=../go.mod govulncheck ./...

[group('qa')]
workflow-check:
    actionlint
    zizmor --offline --persona=pedantic .

[group('qa')]
modernize-check:
    go fix -diff ./...
    cd functions && GOWORK=off go fix -diff ./...

[group('qa')]
qa: tidy fix check

[group('qa')]
check: workflow-check fmt-check lint tidy-check modernize-check build coverage test-functions vuln

alias install := download
[group('lifecycle')]
download: download-app download-functions

[group('lifecycle')]
download-app:
    go mod download

[group('lifecycle')]
download-functions:
    cd functions && GOWORK=off go mod download

[group('lifecycle')]
tidy: tidy-app tidy-functions

[group('lifecycle')]
tidy-app:
    go mod tidy

[group('lifecycle')]
tidy-functions:
    cd functions && GOWORK=off go mod tidy

[group('lifecycle')]
tidy-check:
    go mod tidy -diff
    cd functions && GOWORK=off go mod tidy -diff

[group('lifecycle')]
update: update-app update-functions

[group('lifecycle')]
update-app:
    go get -u -t ./... tool
    go mod tidy

[group('lifecycle')]
update-functions:
    cd functions && GOWORK=off go get -u -t ./...
    cd functions && GOWORK=off go mod tidy

[group('lifecycle')]
fresh: clean download build

[group('container')]
container-build image="huma-playground:latest" version="dev" runtime_img="":
    {{ CONTAINER_RUNTIME }} build \
        --build-arg VERSION={{ version }} \
        {{ if runtime_img != "" { "--build-arg RUNTIME_IMAGE=" + runtime_img } else { "" } }} \
        -t {{ image }} .

[group('container')]
container-up image="huma-playground:latest" name="huma-playground" host_port=PORT:
    {{ CONTAINER_RUNTIME }} run -d --rm --name {{ name }} \
        {{ if path_exists(".env") == "true" { "--env-file .env" } else { "" } }} \
        -e PORT=8080 -p {{ host_port }}:8080 {{ image }}

[group('container')]
container-smoke image="huma-playground:smoke" name="huma-playground-smoke" host_port="18080":
    #!/usr/bin/env bash
    set -euo pipefail
    runtime="{{ CONTAINER_RUNTIME }}"
    tmp="$(mktemp -d)"
    container_id=""
    cleanup() {
      result=$?
      if [[ -n "$container_id" ]]; then
        if [[ "$result" -ne 0 ]]; then "$runtime" logs "$container_id" 2>&1 | tail -n 200 >&2 || true; fi
        "$runtime" rm --force "$container_id" >/dev/null 2>&1 || true
      fi
      rm -rf "$tmp"
    }
    trap cleanup EXIT
    just container-build {{ image }} ci-smoke
    test "$("$runtime" image inspect --format '{{ "{{.Config.User}}" }}' {{ image }})" = "65532:65532"
    container_id="$("$runtime" run -d --name {{ name }} \
      -e APP_ENVIRONMENT=development \
      -e FIREBASE_MODE=offline \
      -e PORT=8080 \
      -p 127.0.0.1:{{ host_port }}:8080 {{ image }})"
    base="http://127.0.0.1:{{ host_port }}"
    for _ in {1..60}; do
      if curl --fail --silent --max-time 5 "$base/health" >"$tmp/health.json"; then break; fi
      sleep 0.25
    done
    jq -e '. == {"status":"healthy"}' "$tmp/health.json"
    curl --fail --silent --max-time 5 -D "$tmp/headers" -H 'X-Request-ID: container-smoke' "$base/v1/hello" \
      | jq -e '. == {"message":"Hello, World!"}'
    tr -d '\r' <"$tmp/headers" | grep -qi '^X-Request-Id: container-smoke$'
    if grep -qi '^Server:' "$tmp/headers"; then
      printf '%s\n' 'Unexpected Server header' >&2
      exit 1
    fi
    test "$(curl --fail --silent --max-time 5 -H 'Accept: application/cbor' -o "$tmp/hello.cbor" -w '%{content_type}' "$base/v1/hello")" = application/cbor
    test -s "$tmp/hello.cbor"
    curl --fail --silent --max-time 5 "$base/openapi.json" | jq -e '.openapi == "3.1.0"'
    "$runtime" logs "$container_id" >"$tmp/logs" 2>&1
    grep -F '"version":"ci-smoke"' "$tmp/logs"
    grep -F '"go_version":"go1.27.2"' "$tmp/logs"
    "$runtime" stop --time 10 "$container_id" >/dev/null
    test "$("$runtime" inspect --format '{{ "{{.State.ExitCode}}" }}' "$container_id")" = 0
    "$runtime" logs "$container_id" 2>&1 | grep -F '"message":"server exited"'

[group('container')]
container-logs name="huma-playground":
    {{ CONTAINER_RUNTIME }} logs -f {{ name }}

[group('container')]
container-down name="huma-playground":
    -{{ CONTAINER_RUNTIME }} stop {{ name }}
