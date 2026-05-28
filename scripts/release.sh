#!/bin/bash
# Slim release script: bump version, commit, tag, push.
#
# Usage:
#   ./scripts/release.sh patch
#   ./scripts/release.sh minor
#   ./scripts/release.sh major
#   ./scripts/release.sh 2.1.0
#   ./scripts/release.sh 2.1.0-rc.1

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

DRY_RUN=0             # 1 = replace destructive steps with logs
CHANGELOG_SECTION=""  # populated by load_changelog_section

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()   { echo -e "${GREEN}[RELEASE]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
step()  { echo -e "\n${CYAN}${BOLD}==> $1${NC}"; }

# --- Usage ---
usage() {
    cat <<EOF
Usage: $0 [--dry-run] [patch|minor|major|VERSION]

Version bump:
  patch  - 2.0.9 → 2.0.10   (bugfixes)
  minor  - 2.0.9 → 2.1.0    (new features)
  major  - 2.0.9 → 3.0.0    (breaking changes)

Explicit version:
  2.1.0       - set exact version
  2.1.0-rc.1  - pre-release (auto-detected for GitHub)

Flags:
  --dry-run   replace destructive steps with logs

Before running, ADD the '## [VERSION] - YYYY-MM-DD' block to CHANGELOG.md —
the script reads it from there for the release preview.

Examples:
  $0 patch
  $0 2.1.0
  $0 --dry-run minor
EOF
    exit 1
}

# --- Validation ---
validate_prerequisites() {
    step "Validating prerequisites"

    # Check gh CLI
    if ! command -v gh &>/dev/null; then
        error "GitHub CLI (gh) not found. Install: https://cli.github.com/"
    fi

    if ! gh auth status &>/dev/null; then
        error "Not authenticated with GitHub. Run: gh auth login"
    fi
    log "GitHub CLI: authenticated"

    # Check git remote
    if ! git -C "$PROJECT_ROOT" remote get-url origin &>/dev/null; then
        error "No git remote 'origin' configured. Run: git remote add origin git@github.com:hoaxisr/awg-manager.git"
    fi
    log "Git remote: $(git -C "$PROJECT_ROOT" remote get-url origin)"

    # Check clean working tree (allow untracked)
    if ! git -C "$PROJECT_ROOT" diff --quiet HEAD 2>/dev/null; then
        warn "Working tree has uncommitted changes"
        git -C "$PROJECT_ROOT" status --short
        echo ""
        read -rp "Continue anyway? [y/N] " answer
        [[ "$answer" =~ ^[Yy]$ ]] || exit 1
    fi
}

# --- Version handling ---
# Strip pre-release suffix: 2.0.9-rc.1 → 2.0.9
strip_prerelease() {
    echo "$1" | sed 's/-.*//'
}

bump_version() {
    local current="$1"
    local bump_type="$2"

    # Strip any pre-release suffix before bumping
    local base
    base=$(strip_prerelease "$current")

    IFS='.' read -r major minor patch <<< "$base"

    case "$bump_type" in
        patch) patch=$((patch + 1)) ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        major) major=$((major + 1)); minor=0; patch=0 ;;
    esac

    echo "${major}.${minor}.${patch}"
}

resolve_version() {
    local input="$1"
    local current
    current=$(cat "$PROJECT_ROOT/VERSION")

    log "Current version: $current"

    case "$input" in
        patch|minor|major)
            NEW_VERSION=$(bump_version "$current" "$input")
            ;;
        [0-9]*)
            # Explicit version — basic validation
            if ! [[ "$input" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
                error "Invalid version format: $input (expected: X.Y.Z or X.Y.Z-suffix)"
            fi
            NEW_VERSION="$input"
            ;;
        *)
            usage
            ;;
    esac

    log "New version: ${BOLD}$NEW_VERSION${NC}"

    # Check if tag already exists
    if git -C "$PROJECT_ROOT" tag -l "v${NEW_VERSION}" | grep -q .; then
        error "Tag v${NEW_VERSION} already exists"
    fi
}

# --- Tag release on the current branch ---
tag_release() {
    step "Tagging release on current branch"

    # Tags the version-bump commit on whatever branch the release is run from.
    # Run the release from the stable branch (master) after merging develop into it.

    cd "$PROJECT_ROOT"

    local branch
    branch=$(git rev-parse --abbrev-ref HEAD)

    if [[ "$DRY_RUN" -eq 1 ]]; then
        log "[dry-run] Would: git tag v${NEW_VERSION} && git push origin ${branch} v${NEW_VERSION}"
        return 0
    fi

    git tag "v${NEW_VERSION}"
    log "Tagged: v${NEW_VERSION} on ${branch}"

    git push origin "${branch}" "v${NEW_VERSION}"
    log "Pushed ${branch} + tag v${NEW_VERSION}"
}

# --- Summary ---
print_summary() {
    step "Release v${NEW_VERSION} complete!"

    echo ""
    echo -e "${BOLD}GitHub Release:${NC}"
    echo "  https://github.com/hoaxisr/awg-manager/releases/tag/v${NEW_VERSION}"
    echo ""
}

# --- Changelog loader ---
# CHANGELOG.md is the single source of truth. This extracts the body
# of the "## [NEW_VERSION] - DATE" block (everything up to but excluding
# the next "## [" heading) for use in the release preview.
# If the block is missing, abort.
load_changelog_section() {
    step "Loading changelog section from CHANGELOG.md"

    local changelog="$PROJECT_ROOT/CHANGELOG.md"
    if [[ ! -f "$changelog" ]]; then
        error "CHANGELOG.md not found at $changelog"
    fi

    local header="## [${NEW_VERSION}] - "
    CHANGELOG_SECTION=$(awk -v hdr="$header" '
        index($0, hdr) == 1     { capture=1; next }
        capture && index($0, "## [") == 1 { exit }
        capture                  { print }
    ' "$changelog" | sed -E '1{/^$/d;}' | sed -E '${/^$/d;}')

    if [[ -z "$CHANGELOG_SECTION" ]]; then
        error "No '## [$NEW_VERSION] - YYYY-MM-DD' block in CHANGELOG.md. Add it before releasing."
    fi
}

# --- Pre-flight preview ---
confirm_release() {
    echo
    echo "============= PREVIEW ============="
    echo "## [$NEW_VERSION] - $(date +%Y-%m-%d)"
    echo
    printf '%s\n' "$CHANGELOG_SECTION"
    echo "==================================="
    echo "Actions:"
    echo "  - write VERSION → $NEW_VERSION"
    echo "  - git commit + tag v$NEW_VERSION"
    echo "  - git push origin <branch> v$NEW_VERSION"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  (DRY-RUN — no destructive step will execute)"
    fi
    echo

    read -r -p "Proceed? [y/N] " ans
    [[ "$ans" == "y" || "$ans" == "Y" ]] || error "Release cancelled."
}

# --- Write VERSION ---
write_version_file() {
    step "Writing VERSION file"
    if [[ "$DRY_RUN" -eq 1 ]]; then
        log "[dry-run] would write '$NEW_VERSION' to VERSION"
        return 0
    fi
    echo "$NEW_VERSION" > "$PROJECT_ROOT/VERSION"
}

# --- Commit VERSION + CHANGELOG ---
commit_version_and_changelog() {
    step "Committing release commit"
    cd "$PROJECT_ROOT"
    git add VERSION
    if ! git diff --quiet CHANGELOG.md 2>/dev/null; then
        git add CHANGELOG.md
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        log "[dry-run] would: git commit -m 'release: v$NEW_VERSION'"
        return 0
    fi
    # Если VERSION уже актуальный и CHANGELOG не менялся — фейлить релиз нельзя.
    git commit -m "release: v$NEW_VERSION" || log "Nothing to commit (VERSION/CHANGELOG already at target)"
}

# --- Main ---
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --dry-run) DRY_RUN=1; shift ;;
            --)        shift; break ;;
            -*)
                echo "unknown flag: $1" >&2
                exit 1 ;;
            *) break ;;
        esac
    done

    [[ $# -lt 1 ]] && usage

    BUMP_OR_VERSION="${1}"
}

parse_args "$@"
validate_prerequisites
resolve_version "$BUMP_OR_VERSION"
load_changelog_section
confirm_release
write_version_file
commit_version_and_changelog
tag_release
print_summary
