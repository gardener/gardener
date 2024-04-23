#!/usr/bin/env bash

# Copyright 2015 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This file was copied from the kubernetes/kubernetes project
# https://github.com/kubernetes/kubernetes/blob/v1.20.0/hack/cherry_pick_pull.sh
#
# Modifications Copyright SAP SE or an SAP affiliate company and Gardener contributors

# Usage Instructions: https://github.com/gardener/gardener/blob/master/docs/development/process.md#cherry-picks

# Checkout a PR from GitHub. (Yes, this is sitting in a Git tree. How
# meta.) Assumes you care about pulls from remote "upstream" and
# checks them out to a branch named:
#  automated-cherry-pick-of-<pr>-<target branch>-<timestamp>

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
declare -r REPO_ROOT
cd "${REPO_ROOT}"

STARTINGBRANCH=$(git symbolic-ref --short HEAD)
declare -r STARTINGBRANCH
declare -r REBASEMAGIC="${REPO_ROOT}/.git/rebase-apply"
DRY_RUN=${DRY_RUN:-""}
UPSTREAM_REMOTE=${UPSTREAM_REMOTE:-upstream}
FORK_REMOTE=${FORK_REMOTE:-origin}
MAIN_REPO_ORG=${MAIN_REPO_ORG:-$(git remote get-url "$UPSTREAM_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $3}')}
MAIN_REPO_NAME=${MAIN_REPO_NAME:-$(git remote get-url "$UPSTREAM_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $4}')}
DEPRECATED_RELEASE_NOTE_CATEGORY="|noteworthy|improvement|action"
RELEASE_NOTE_CATEGORY="(breaking|feature|bugfix|doc|other${DEPRECATED_RELEASE_NOTE_CATEGORY})"
RELEASE_NOTE_TARGET_GROUP="(user|operator|developer|dependency)"

if [[ -z ${GITHUB_USER:-} ]]; then
  echo "Please export GITHUB_USER=<your-user> (or GH organization, if that's where your fork lives)"
  exit 1
fi

if ! which hub > /dev/null; then
  echo "Can't find 'hub' tool in PATH, please install from https://github.com/github/hub"
  exit 1
fi

if [[ "$#" -lt 2 ]]; then
  echo "${0} <upstream remote>/<remote branch> <pr-number>...: cherry pick one or more <pr> onto <remote branch> and leave instructions for proposing pull request"
  echo
  echo "  Checks out <upstream remote>/<remote branch> and handles the cherry-pick of <pr> (possibly multiple) for you."
  echo "  Examples:"
  echo "    $0 ${UPSTREAM_REMOTE}/release-3.14 12345        # Cherry-picks PR 12345 onto ${UPSTREAM_REMOTE}/release-3.14 and proposes that as a PR."
  echo "    $0 ${UPSTREAM_REMOTE}/release-3.14 12345 56789  # Cherry-picks PR 12345, then 56789 and proposes the combination as a single PR."
  echo
  echo "  Set the DRY_RUN environment var to skip git push and creating PR."
  echo "  This is useful for creating patches to a release branch without making a PR."
  echo "  When DRY_RUN is set the script will leave you in a branch containing the commits you cherry-picked."
  echo
  echo "  Set UPSTREAM_REMOTE (default: upstream) and FORK_REMOTE (default: origin)"
  echo "  to override the default remote names to what you have locally."
  exit 2
fi

if git_status=$(git status --porcelain --untracked=no 2>/dev/null) && [[ -n "${git_status}" ]]; then
  echo "!!! Dirty tree. Clean up and try again."
  exit 1
fi

if [[ -e "${REBASEMAGIC}" ]]; then
  echo "!!! 'git rebase' or 'git am' in progress. Clean up and try again."
  exit 1
fi

declare -r BRANCH="$1"
shift 1
declare -r PULLS=( "$@" )

function join { local IFS="$1"; shift; echo "$*"; }
PULLDASH=$(join - "${PULLS[@]/#/#}") # Generates something like "#12345-#56789"
declare -r PULLDASH
PULLSUBJ=$(join " " "${PULLS[@]/#/#}") # Generates something like "#12345 #56789"
declare -r PULLSUBJ

echo "+++ Updating remotes..."
git remote update "${UPSTREAM_REMOTE}" "${FORK_REMOTE}"

if ! git log -n1 --format=%H "${BRANCH}" >/dev/null 2>&1; then
  echo "!!! '${BRANCH}' not found. The second argument should be something like ${UPSTREAM_REMOTE}/release-0.21."
  echo "    (In particular, it needs to be a valid, existing remote branch that I can 'git checkout'.)"
  exit 1
fi

NEWBRANCHREQ="automated-cherry-pick-of-${PULLDASH}" # "Required" portion for tools.
declare -r NEWBRANCHREQ
NEWBRANCH="$(echo "${NEWBRANCHREQ}-${BRANCH}" | sed 's/\//-/g')"
declare -r NEWBRANCH
NEWBRANCHUNIQ="${NEWBRANCH}-$(date +%s)"
declare -r NEWBRANCHUNIQ
echo "+++ Creating local branch ${NEWBRANCHUNIQ}"

cleanbranch=""
prtext=""
gitamcleanup=false
function return_to_kansas {
  if [[ "${gitamcleanup}" == "true" ]]; then
    echo
    echo "+++ Aborting in-progress git am."
    git am --abort >/dev/null 2>&1 || true
  fi

  # return to the starting branch and delete the PR text file
  if [[ -z "${DRY_RUN}" ]]; then
    echo
    echo "+++ Returning you to the ${STARTINGBRANCH} branch and cleaning up."
    git checkout -f "${STARTINGBRANCH}" >/dev/null 2>&1 || true
    if [[ -n "${cleanbranch}" ]]; then
      git branch -D "${cleanbranch}" >/dev/null 2>&1 || true
    fi
    if [[ -n "${prtext}" ]]; then
      rm "${prtext}"
    fi
  fi
}
trap return_to_kansas EXIT

SUBJECTS=()
RELEASE_NOTES=()
LABELS=()
function make-a-pr() {
  local rel
  rel="$(basename "${BRANCH}")"
  echo
  echo "+++ Creating a pull request on GitHub at ${GITHUB_USER}:${NEWBRANCH}"

  # This looks like an unnecessary use of a tmpfile, but it avoids
  # https://github.com/github/hub/issues/976 Otherwise stdin is stolen
  # when we shove the heredoc at hub directly, tickling the ioctl
  # crash.
  prtext="$(mktemp -t prtext.XXXX)" # cleaned in return_to_kansas
  local numandtitle
  numandtitle=$(printf '%s\n' "${SUBJECTS[@]}")
  relnotes=$(printf "${RELEASE_NOTES[@]}")
  labels=$(printf "${LABELS[@]}")
  cat >"${prtext}" <<EOF
[${rel}] Automated cherry pick of ${numandtitle}

${labels}

Cherry pick of ${PULLSUBJ} on ${rel}.

${numandtitle}

**Release Notes:**
${relnotes}
EOF

  hub pull-request -F "${prtext}" -h "${GITHUB_USER}:${NEWBRANCH}" -b "${MAIN_REPO_ORG}:${rel}"
}

git checkout -b "${NEWBRANCHUNIQ}" "${BRANCH}"
cleanbranch="${NEWBRANCHUNIQ}"

gitamcleanup=true
for pull in "${PULLS[@]}"; do
  echo "+++ Downloading patch to /tmp/${pull}.patch (in case you need to do this again)"

  curl -o "/tmp/${pull}.patch" -sSL "https://github.com/${MAIN_REPO_ORG}/${MAIN_REPO_NAME}/pull/${pull}.patch"
  echo
  echo "+++ About to attempt cherry pick of PR. To reattempt:"
  echo "  $ git am -3 /tmp/${pull}.patch"
  echo
  git am -3 "/tmp/${pull}.patch" || {
    conflicts=false
    while unmerged=$(git status --porcelain | grep ^U) && [[ -n ${unmerged} ]] \
      || [[ -e "${REBASEMAGIC}" ]]; do
      conflicts=true # <-- We should have detected conflicts once
      echo
      echo "+++ Conflicts detected:"
      echo
      (git status --porcelain | grep ^U) || echo "!!! None. Did you git am --continue?"
      echo
      echo "+++ Please resolve the conflicts in another window (and remember to 'git add / git am --continue')"
      read -p "+++ Proceed (anything but 'y' aborts the cherry-pick)? [y/n] " -r
      echo
      if ! [[ "${REPLY}" =~ ^[yY]$ ]]; then
        echo "Aborting." >&2
        exit 1
      fi
    done

    if [[ "${conflicts}" != "true" ]]; then
      echo "!!! git am failed, likely because of an in-progress 'git am' or 'git rebase'"
      exit 1
    fi
  }

  # set the subject
  pr_info=$(curl "https://api.github.com/repos/${MAIN_REPO_ORG}/${MAIN_REPO_NAME}/pulls/${pull}" -sS)
  subject=$(echo ${pr_info} | jq -cr '.title')
  SUBJECTS+=("#${pull}: ${subject}")
  labels=$(echo ${pr_info} | jq '.labels[].name' -cr | grep -P '^(area|kind)' | sed -e 's|/| |' -e 's|^|/|g')
  LABELS+=("${labels}")

  # remove the patch file from /tmp
  rm -f "/tmp/${pull}.patch"

  # get the release notes
  notes=$(echo ${pr_info} | jq '.body' | grep -Po "\`\`\` *${RELEASE_NOTE_CATEGORY} ${RELEASE_NOTE_TARGET_GROUP}.*?\`\`\`" || true)
  RELEASE_NOTES+=("${notes}")
done
gitamcleanup=false

if [[ -n "${DRY_RUN}" ]]; then
  echo "!!! Skipping git push and PR creation because you set DRY_RUN."
  echo "To return to the branch you were in when you invoked this script:"
  echo
  echo "  git checkout ${STARTINGBRANCH}"
  echo
  echo "To delete this branch:"
  echo
  echo "  git branch -D ${NEWBRANCHUNIQ}"
  exit 0
fi

if git remote -v | grep ^"${FORK_REMOTE}" | grep "${MAIN_REPO_ORG}/${MAIN_REPO_NAME}.git"; then
  echo "!!! You have ${FORK_REMOTE} configured as your ${MAIN_REPO_ORG}/${MAIN_REPO_NAME}.git"
  echo "This isn't normal. Leaving you with push instructions:"
  echo
  echo "+++ First manually push the branch this script created:"
  echo
  echo "  git push REMOTE ${NEWBRANCHUNIQ}:${NEWBRANCH}"
  echo
  echo "where REMOTE is your personal fork (maybe ${UPSTREAM_REMOTE}? Consider swapping those.)."
  echo "OR consider setting UPSTREAM_REMOTE and FORK_REMOTE to different values."
  echo
  make-a-pr
  cleanbranch=""
  exit 0
fi

echo
echo "+++ I'm about to do the following to push to GitHub (and I'm assuming ${FORK_REMOTE} is your personal fork):"
echo
echo "  git push ${FORK_REMOTE} ${NEWBRANCHUNIQ}:${NEWBRANCH}"
echo
read -p "+++ Proceed (anything but 'y' aborts the cherry-pick)? [y/n] " -r
if ! [[ "${REPLY}" =~ ^[yY]$ ]]; then
  echo "Aborting." >&2
  exit 1
fi

git push "${FORK_REMOTE}" -f "${NEWBRANCHUNIQ}:${NEWBRANCH}"
make-a-pr
