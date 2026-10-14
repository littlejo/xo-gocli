#!/usr/bin/env bash
#
# bump-version.sh
#
# Computes the next semver tag from git history and prints the decision on
# the LAST line of stdout, so the caller can parse it:
#
#   vMAJOR.MINOR.PATCH   -> a new tag should be created with this value
#   SKIP                 -> HEAD is already tagged, do nothing
#
# Rules (conventional commits):
#   - a "feat" commit (or a merge from a *feat* branch) since the last tag
#     bumps the MINOR version and resets PATCH (0.1.0 -> 0.2.0)
#   - any other change bumps PATCH (0.1.0 -> 0.1.1)
#   - a change that only touches documentation (docs/, README.md, AGENTS.md,
#     CHANGELOG.md, CONTRIBUTING.md, SECURITY.md)
#     does NOT ship a new tag: documentation is not part of the release
#   - with no tag at all, the first feature starts the project at 0.1.0
#
# Human-readable detail is printed on the other lines.

set -euo pipefail

# --- find the most recent version tag that is an ancestor of HEAD ----------
LAST_TAG="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1 || true)"

# Only accept strict "vMAJOR.MINOR.PATCH" tags, so the version arithmetic
# below can never operate on an unexpected (e.g. crafted) tag name.
if ! echo "${LAST_TAG:-}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  LAST_TAG=""
fi

# Idempotency: if HEAD itself is already tagged, there is nothing to do.
if [ -n "$LAST_TAG" ] \
  && [ "$(git rev-parse "${LAST_TAG}^{commit}")" = "$(git rev-parse HEAD)" ]; then
  echo "HEAD $(git rev-parse --short HEAD) is already tagged ${LAST_TAG}"
  echo "SKIP"
  exit 0
fi

# --- base version + range of commits to inspect ----------------------------
if [ -n "$LAST_TAG" ]; then
  BASE="${LAST_TAG#v}"
  RANGE="${LAST_TAG}..HEAD"
else
  BASE="0.0.0"
  RANGE="HEAD"
fi

# --- documentation-only change: no new tag ----------------------------------
# A release ships a binary plus go.mod; docs/ and the top-level markdown
# files are documentation (the archives carry a copy of README.md and
# CHANGELOG.md, but changing them alone is not worth a release). When the
# whole range only touches documentation files, skip the tag so doc fixes do
# not churn the version.
DOC_FILES="^(docs/|(README|AGENTS|CHANGELOG|CONTRIBUTING|SECURITY)\.md$)"
if [ -n "$LAST_TAG" ]; then
  CHANGED="$(git diff --name-only "$LAST_TAG" HEAD)"
else
  CHANGED="$(git ls-tree -r --name-only HEAD)"
fi
if [ -z "$CHANGED" ]; then
  echo "range $RANGE has no file changes"
  echo "SKIP"
  exit 0
fi
if ! printf '%s\n' "$CHANGED" | grep -qvE "$DOC_FILES"; then
  echo "range $RANGE only touches documentation"
  echo "SKIP"
  exit 0
fi

MAJOR="${BASE%%.*}"
REST="${BASE#*.}"
MINOR="${REST%%.*}"
PATCH="${REST#*.}"
PATCH="${PATCH%%-*}"   # strip any pre-release suffix

# --- detect a feature since the last tag -----------------------------------
# Conventional commits: a subject starting with "feat" ("feat:" or
# "feat(scope):") counts as a feature.
if git log "$RANGE" --pretty=%s | grep -Eq '^feat(\(|:)|^feat$'; then
  MINOR=$((MINOR + 1))
  PATCH=0
  KIND="minor (a feat commit was found)"
else
  PATCH=$((PATCH + 1))
  KIND="patch"
fi

NEW="v${MAJOR}.${MINOR}.${PATCH}"

echo "last tag : ${LAST_TAG:-<none>}"
echo "range    : ${RANGE}"
echo "decision : ${KIND}"

# Guard against a no-op (e.g. base 0.0.0 with only patch -> would be 0.0.1,
# which we keep; but if the computed value equals the last tag, skip).
if [ "$NEW" = "$LAST_TAG" ]; then
  echo "SKIP"
  exit 0
fi

echo "$NEW"
