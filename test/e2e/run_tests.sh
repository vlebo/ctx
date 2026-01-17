#!/bin/bash
# SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
# SPDX-License-Identifier: MIT
#
# ctx End-to-End Test Suite
# Run on a sandboxed machine to test all ctx features
#
# Usage: ./run_tests.sh [OPTIONS]
#   -v, --verbose     Show command output
#   --keep-config     Don't clean up test contexts after running
#

# Don't exit on error - we handle errors ourselves
set +e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Config
CTX_CONFIG_DIR="${HOME}/.config/ctx"
CTX_CONTEXTS_DIR="${CTX_CONFIG_DIR}/contexts"
CTX_STATE_DIR="${CTX_CONFIG_DIR}/state"
KEEP_CONFIG=false
VERBOSE=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        --keep-config)
            KEEP_CONFIG=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
    esac
done

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
    ((TESTS_SKIPPED++))
}

log_section() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

assert_eq() {
    local actual="$1"
    local expected="$2"
    local message="$3"

    if [[ "$actual" == "$expected" ]]; then
        log_success "$message"
        return 0
    else
        log_fail "$message (expected: '$expected', got: '$actual')"
        return 1
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="$3"

    if [[ "$haystack" == *"$needle"* ]]; then
        log_success "$message"
        return 0
    else
        log_fail "$message (expected to contain: '$needle')"
        return 1
    fi
}

assert_not_empty() {
    local value="$1"
    local message="$2"

    if [[ -n "$value" ]]; then
        log_success "$message"
        return 0
    else
        log_fail "$message (value is empty)"
        return 1
    fi
}

assert_empty() {
    local value="$1"
    local message="$2"

    if [[ -z "$value" ]]; then
        log_success "$message"
        return 0
    else
        log_fail "$message (expected empty, got: '$value')"
        return 1
    fi
}

assert_file_exists() {
    local path="$1"
    local message="$2"

    if [[ -f "$path" ]]; then
        log_success "$message"
        return 0
    else
        log_fail "$message (file not found: $path)"
        return 1
    fi
}

assert_command_success() {
    local message="$1"
    shift

    if "$@" > /dev/null 2>&1; then
        log_success "$message"
        return 0
    else
        log_fail "$message (command failed: $*)"
        return 1
    fi
}

# Run a command, show output if verbose
run_cmd() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${YELLOW}  > $*${NC}" >&2
        "$@" 2>&1 | sed 's/^/    /' >&2
        return ${PIPESTATUS[0]}
    else
        "$@" > /dev/null 2>&1
    fi
}

# Run a command and capture output, show if verbose
# Verbose output goes to stderr, captured output to stdout
run_cmd_capture() {
    local output
    output=$("$@" 2>&1)
    local exit_code=$?
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${YELLOW}  > $*${NC}" >&2
        echo "$output" | sed 's/^/    /' >&2
    fi
    echo "$output"
    return $exit_code
}

# Cleanup function
cleanup() {
    log_section "Cleanup"

    if [[ "$KEEP_CONFIG" == "true" ]]; then
        log_info "Keeping test config (--keep-config specified)"
        return
    fi

    log_info "Removing test contexts..."
    rm -f "${CTX_CONTEXTS_DIR}/e2e-test-dev.yaml"
    rm -f "${CTX_CONTEXTS_DIR}/e2e-test-prod.yaml"
    rm -f "${CTX_CONTEXTS_DIR}/e2e-test-full.yaml"

    log_info "Clearing state..."
    rm -f "${CTX_STATE_DIR}/current.name"
    rm -f "${CTX_STATE_DIR}/current.env"

    log_info "Cleanup complete"
}

# Setup function
setup() {
    log_section "Setup"

    # Check if ctx is installed
    if ! command -v ctx &> /dev/null; then
        echo -e "${RED}Error: ctx is not installed or not in PATH${NC}"
        exit 1
    fi

    log_info "ctx version: $(ctx --version 2>/dev/null || echo 'unknown')"

    # Initialize ctx if needed
    if [[ ! -d "$CTX_CONTEXTS_DIR" ]]; then
        log_info "Initializing ctx..."
        ctx init
    fi

    # Create test contexts
    log_info "Creating test contexts..."

    # Development context
    cat > "${CTX_CONTEXTS_DIR}/e2e-test-dev.yaml" << 'EOF'
name: e2e-test-dev
description: "E2E Test Development Environment"
environment: development
env_color: green

aws:
  profile: e2e-test-dev
  region: us-west-2

git:
  user_name: "E2E Test Dev"
  user_email: "e2e-dev@test.local"

proxy:
  http: http://proxy.test:8080
  https: http://proxy.test:8080
  no_proxy: localhost,127.0.0.1,.local

env:
  E2E_TEST_VAR: "dev-value"
  E2E_ENVIRONMENT: "development"
  LOG_LEVEL: "debug"

urls:
  docs: https://docs.example.com
  api: https://api.example.com

deactivate:
  disconnect_vpn: true
  stop_tunnels: true
EOF

    # Production context
    cat > "${CTX_CONTEXTS_DIR}/e2e-test-prod.yaml" << 'EOF'
name: e2e-test-prod
description: "E2E Test Production Environment"
environment: production
env_color: red

aws:
  profile: e2e-test-prod
  region: us-east-1

git:
  user_name: "E2E Test Prod"
  user_email: "e2e-prod@test.local"

env:
  E2E_TEST_VAR: "prod-value"
  E2E_ENVIRONMENT: "production"
  LOG_LEVEL: "warn"

deactivate:
  disconnect_vpn: false
  stop_tunnels: true
EOF

    # Full-featured context (for comprehensive testing)
    cat > "${CTX_CONTEXTS_DIR}/e2e-test-full.yaml" << 'EOF'
name: e2e-test-full
description: "E2E Test Full Featured Environment"
environment: staging
env_color: yellow
tags: [e2e, test, full]

aws:
  profile: e2e-test-full
  region: eu-west-1

gcp:
  project: e2e-test-project
  region: europe-west1
  config_name: e2e-test

azure:
  subscription_id: 00000000-0000-0000-0000-000000000000
  tenant_id: 00000000-0000-0000-0000-000000000001

nomad:
  address: http://nomad.local:4646
  namespace: e2e-test

consul:
  address: http://consul.local:8500

vault:
  address: http://vault.local:8200
  namespace: e2e-test

git:
  user_name: "E2E Test Full"
  user_email: "e2e-full@test.local"

databases:
  - name: postgres
    type: postgres
    host: localhost
    port: 5432
    database: e2e_test
    username: e2e_user

env:
  E2E_TEST_VAR: "full-value"
  E2E_ENVIRONMENT: "staging"
  CUSTOM_FLAG: "enabled"
EOF

    log_info "Setup complete"
}

# Source shell hook for env var handling
# This simulates what the shell hook does
source_ctx_env() {
    local context_name="$1"
    eval "$(ctx use --export "$context_name" 2>/dev/null)"
}

clear_ctx_env() {
    eval "$(ctx deactivate --export 2>/dev/null)"
}

#
# TEST SUITES
#

test_basic_commands() {
    log_section "Test: Basic Commands"

    # ctx --version
    local version_output
    version_output=$(run_cmd_capture ctx --version)
    assert_not_empty "$version_output" "ctx --version returns output"

    # ctx (no args, no context)
    clear_ctx_env
    local status_output
    status_output=$(run_cmd_capture ctx)
    assert_contains "$status_output" "No context" "ctx shows no active context"

    # ctx list
    local list_output
    list_output=$(run_cmd_capture ctx list)
    assert_contains "$list_output" "e2e-test-dev" "ctx list shows e2e-test-dev"
    assert_contains "$list_output" "e2e-test-prod" "ctx list shows e2e-test-prod"

    # ctx show
    local show_output
    show_output=$(run_cmd_capture ctx show e2e-test-dev)
    assert_contains "$show_output" "e2e-test-dev" "ctx show displays context name"
    assert_contains "$show_output" "development" "ctx show displays environment"
}

test_context_switching() {
    log_section "Test: Context Switching"

    # Clear any existing context
    clear_ctx_env

    # Switch to dev context
    log_info "Switching to e2e-test-dev..."
    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev

    assert_eq "$CTX_CURRENT" "e2e-test-dev" "CTX_CURRENT is set correctly"
    assert_eq "$CTX_ENVIRONMENT" "development" "CTX_ENVIRONMENT is set correctly"
    assert_eq "$AWS_PROFILE" "e2e-test-dev" "AWS_PROFILE is set correctly"
    assert_eq "$AWS_REGION" "us-west-2" "AWS_REGION is set correctly"
    assert_eq "$E2E_TEST_VAR" "dev-value" "Custom env var is set correctly"
    assert_eq "$GIT_AUTHOR_NAME" "E2E Test Dev" "GIT_AUTHOR_NAME is set correctly"
    assert_eq "$HTTP_PROXY" "http://proxy.test:8080" "HTTP_PROXY is set correctly"

    # ctx shows current context
    local status_output
    status_output=$(run_cmd_capture ctx)
    assert_contains "$status_output" "e2e-test-dev" "ctx shows current context"

    # State file exists
    assert_file_exists "${CTX_STATE_DIR}/current.name" "current.name state file exists"
    assert_file_exists "${CTX_STATE_DIR}/current.env" "current.env state file exists"
}

test_production_confirmation() {
    log_section "Test: Production Confirmation"

    clear_ctx_env

    # Try to switch to prod without --confirm (should fail with non-interactive)
    log_info "Testing production switch without --confirm..."
    if run_cmd ctx use e2e-test-prod < /dev/null; then
        log_fail "Production switch should require confirmation"
    else
        log_success "Production switch requires confirmation"
    fi

    # Switch with --confirm
    log_info "Testing production switch with --confirm..."
    if run_cmd ctx use e2e-test-prod --confirm; then
        source_ctx_env e2e-test-prod
        assert_eq "$CTX_CURRENT" "e2e-test-prod" "Production switch with --confirm works"
        assert_eq "$CTX_ENVIRONMENT" "production" "Production environment is set"
    else
        log_fail "Production switch with --confirm should work"
    fi
}

test_deactivate() {
    log_section "Test: Deactivate"

    # First activate a context
    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev

    assert_eq "$CTX_CURRENT" "e2e-test-dev" "Context is active before deactivate"

    # Deactivate
    log_info "Deactivating context..."
    run_cmd ctx deactivate
    clear_ctx_env

    assert_empty "$CTX_CURRENT" "CTX_CURRENT is cleared after deactivate"
    assert_empty "$AWS_PROFILE" "AWS_PROFILE is cleared after deactivate"
    assert_empty "$E2E_TEST_VAR" "Custom env var is cleared after deactivate"

    # State files should be cleared
    if [[ -f "${CTX_STATE_DIR}/current.name" ]]; then
        log_fail "current.name should be removed after deactivate"
    else
        log_success "current.name is removed after deactivate"
    fi
}

test_logout() {
    log_section "Test: Logout"

    # First activate a context
    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev

    assert_eq "$CTX_CURRENT" "e2e-test-dev" "Context is active before logout"

    # Logout
    log_info "Logging out..."
    run_cmd ctx logout
    clear_ctx_env

    assert_empty "$CTX_CURRENT" "CTX_CURRENT is cleared after logout"
    assert_empty "$AWS_PROFILE" "AWS_PROFILE is cleared after logout"
}

test_env_var_export() {
    log_section "Test: Environment Variable Export"

    # Test --export flag
    local export_output
    export_output=$(run_cmd_capture ctx use --export e2e-test-dev)

    assert_contains "$export_output" "export CTX_CURRENT=" "Export contains CTX_CURRENT"
    assert_contains "$export_output" "export AWS_PROFILE=" "Export contains AWS_PROFILE"
    assert_contains "$export_output" "export E2E_TEST_VAR=" "Export contains custom env var"

    # Test deactivate --export (need CTX_CURRENT set for deactivate to work)
    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev
    local unset_output
    unset_output=$(CTX_CURRENT=e2e-test-dev run_cmd_capture ctx deactivate --export)

    assert_contains "$unset_output" "unset CTX_CURRENT" "Deactivate export contains unset CTX_CURRENT"
    assert_contains "$unset_output" "unset AWS_PROFILE" "Deactivate export contains unset AWS_PROFILE"

    clear_ctx_env
}

test_full_context() {
    log_section "Test: Full Featured Context"

    clear_ctx_env

    # Switch to full context
    run_cmd ctx use e2e-test-full
    source_ctx_env e2e-test-full

    # AWS
    assert_eq "$AWS_PROFILE" "e2e-test-full" "AWS_PROFILE set"
    assert_eq "$AWS_REGION" "eu-west-1" "AWS_REGION set"

    # GCP
    assert_contains "$CLOUDSDK_CONFIG" "e2e-test-full" "CLOUDSDK_CONFIG has context isolation"
    assert_eq "$CLOUDSDK_CORE_PROJECT" "e2e-test-project" "CLOUDSDK_CORE_PROJECT set"
    assert_eq "$GOOGLE_CLOUD_PROJECT" "e2e-test-project" "GOOGLE_CLOUD_PROJECT set"

    # Azure
    assert_contains "$AZURE_CONFIG_DIR" "e2e-test-full" "AZURE_CONFIG_DIR has context isolation"
    assert_eq "$AZURE_SUBSCRIPTION_ID" "00000000-0000-0000-0000-000000000000" "AZURE_SUBSCRIPTION_ID set"

    # Nomad
    assert_eq "$NOMAD_ADDR" "http://nomad.local:4646" "NOMAD_ADDR set"
    assert_eq "$NOMAD_NAMESPACE" "e2e-test" "NOMAD_NAMESPACE set"

    # Consul
    assert_eq "$CONSUL_HTTP_ADDR" "http://consul.local:8500" "CONSUL_HTTP_ADDR set"

    # Vault
    assert_eq "$VAULT_ADDR" "http://vault.local:8200" "VAULT_ADDR set"
    assert_eq "$VAULT_NAMESPACE" "e2e-test" "VAULT_NAMESPACE set"

    # Git
    assert_eq "$GIT_AUTHOR_NAME" "E2E Test Full" "GIT_AUTHOR_NAME set"
    assert_eq "$GIT_AUTHOR_EMAIL" "e2e-full@test.local" "GIT_AUTHOR_EMAIL set"

    # Database (postgres)
    assert_eq "$PGHOST" "localhost" "PGHOST set"
    assert_eq "$PGPORT" "5432" "PGPORT set"
    assert_eq "$PGDATABASE" "e2e_test" "PGDATABASE set"
    assert_eq "$PGUSER" "e2e_user" "PGUSER set"

    # Custom env
    assert_eq "$E2E_TEST_VAR" "full-value" "Custom E2E_TEST_VAR set"
    assert_eq "$CUSTOM_FLAG" "enabled" "Custom CUSTOM_FLAG set"

    clear_ctx_env
}

test_context_isolation() {
    log_section "Test: Context Isolation"

    # Switch to dev, capture vars
    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev
    local dev_var="$E2E_TEST_VAR"
    local dev_profile="$AWS_PROFILE"

    # Switch to prod, verify vars changed
    run_cmd ctx use e2e-test-prod --confirm
    source_ctx_env e2e-test-prod
    local prod_var="$E2E_TEST_VAR"
    local prod_profile="$AWS_PROFILE"

    assert_eq "$dev_var" "dev-value" "Dev context has correct var"
    assert_eq "$prod_var" "prod-value" "Prod context has correct var"
    assert_eq "$dev_profile" "e2e-test-dev" "Dev context has correct profile"
    assert_eq "$prod_profile" "e2e-test-prod" "Prod context has correct profile"

    if [[ "$dev_var" != "$prod_var" ]]; then
        log_success "Contexts have isolated env vars"
    else
        log_fail "Contexts should have different env vars"
    fi

    clear_ctx_env
}

test_credential_isolation() {
    log_section "Test: Credential Isolation Directories"

    # Check that isolated directories are set correctly
    run_cmd ctx use e2e-test-full
    source_ctx_env e2e-test-full

    # Azure isolation
    local expected_azure_dir="${CTX_STATE_DIR}/cloud/e2e-test-full/azure"
    assert_eq "$AZURE_CONFIG_DIR" "$expected_azure_dir" "Azure config dir is isolated"

    # GCP isolation
    local expected_gcp_dir="${CTX_STATE_DIR}/cloud/e2e-test-full/gcloud"
    assert_eq "$CLOUDSDK_CONFIG" "$expected_gcp_dir" "GCP config dir is isolated"

    clear_ctx_env
}

test_urls() {
    log_section "Test: Quick URLs"

    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev

    # List URLs (need CTX_CURRENT set)
    local urls_output
    urls_output=$(CTX_CURRENT=e2e-test-dev run_cmd_capture ctx open)

    assert_contains "$urls_output" "docs" "URLs list shows docs"
    assert_contains "$urls_output" "api" "URLs list shows api"

    clear_ctx_env
}

test_replace_flag() {
    log_section "Test: Replace Flag (--replace)"

    clear_ctx_env

    # Switch to dev
    run_cmd ctx use e2e-test-dev
    source_ctx_env e2e-test-dev

    assert_eq "$CTX_CURRENT" "e2e-test-dev" "Dev context active"

    # Switch with --replace
    log_info "Switching with --replace flag..."
    run_cmd ctx use e2e-test-prod --confirm --replace
    source_ctx_env e2e-test-prod

    assert_eq "$CTX_CURRENT" "e2e-test-prod" "Prod context active after --replace"

    clear_ctx_env
}

test_show_command() {
    log_section "Test: Show Command"

    local show_output
    show_output=$(run_cmd_capture ctx show e2e-test-full)

    assert_contains "$show_output" "e2e-test-full" "Show displays context name"
    assert_contains "$show_output" "staging" "Show displays environment"
    assert_contains "$show_output" "AWS" "Show displays AWS config"
    assert_contains "$show_output" "GCP" "Show displays GCP config"
}

test_invalid_context() {
    log_section "Test: Invalid Context Handling"

    # Try to use non-existent context
    local use_output
    use_output=$(run_cmd_capture ctx use non-existent-context)
    if echo "$use_output" | grep -qi "not found\|error"; then
        log_success "Invalid context returns error"
    else
        log_fail "Invalid context should return error"
    fi

    # Try to show non-existent context
    local show_output
    show_output=$(run_cmd_capture ctx show non-existent-context)
    if echo "$show_output" | grep -qi "not found\|error"; then
        log_success "Show invalid context returns error"
    else
        log_fail "Show invalid context should return error"
    fi
}

#
# MAIN
#

main() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║              ctx End-to-End Test Suite                       ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Setup
    setup

    # Run test suites
    test_basic_commands
    test_context_switching
    test_production_confirmation
    test_deactivate
    test_logout
    test_env_var_export
    test_full_context
    test_context_isolation
    test_credential_isolation
    test_urls
    test_replace_flag
    test_show_command
    test_invalid_context

    # Cleanup
    cleanup

    # Summary
    log_section "Test Summary"
    echo ""
    echo -e "  ${GREEN}Passed:${NC}  $TESTS_PASSED"
    echo -e "  ${RED}Failed:${NC}  $TESTS_FAILED"
    echo -e "  ${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
    echo ""

    local total=$((TESTS_PASSED + TESTS_FAILED))
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}All $total tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}$TESTS_FAILED of $total tests failed${NC}"
        exit 1
    fi
}

# Run main
main "$@"
