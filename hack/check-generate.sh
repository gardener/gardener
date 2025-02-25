#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Generate"

makefile="$1/Makefile"
check_branch="__check"
initialized_git=false
stashed=false
checked_out=false
generated=false

function delete-check-branch {
  git rev-parse --verify "$check_branch" &>/dev/null && git branch -q -D "$check_branch" || :
}

function cleanup {
  if [[ "$generated" == true ]]; then
    if ! clean_err="$(make -f "$makefile" clean && git reset --hard -q && git clean -qdf)"; then
      echo "Could not clean: $clean_err"
    fi
  fi

  if [[ "$checked_out" == true ]]; then
    if ! checkout_err="$(git checkout -q -)"; then
      echo "Could not checkout to previous branch: $checkout_err"
    fi
  fi

  if [[ "$stashed" == true ]]; then
    if ! stash_err="$(git stash pop -q)"; then
      echo "Could not pop stash: $stash_err"
    fi
  fi

  if [[ "$initialized_git" == true ]]; then
    if ! rm_err="$(rm -rf .git)"; then
      echo "Could not delete git directory: $rm_err"
    fi
  fi

  delete-check-branch
}

trap cleanup EXIT SIGINT SIGTERM

if which git &>/dev/null; then
  if ! git rev-parse --git-dir &>/dev/null; then
    initialized_git=true
    git init -q
    git add --all
    git config --global user.name 'Gardener'
    git config --global user.email 'gardener@cloud'
    git commit -q --allow-empty -m 'initial commit'
  fi

  if [[ "$(git rev-parse --abbrev-ref HEAD)" == "$check_branch" ]]; then
    echo "Already on check branch, aborting"
    exit 1
  fi
  delete-check-branch

  if [[ "$(git status -s)" != "" ]]; then
    stashed=true
    git stash --include-untracked -q
    git stash apply -q &>/dev/null
  fi

  checked_out=true
  git checkout -q -b "$check_branch"
  git add --all
  git commit -q --allow-empty -m 'checkpoint'

  old_status="$(git status -s)"
  if ! out=$(make -f "$makefile" tools-for-generate 2>&1); then
    echo "Error during calling make tools-for-generate: $out"
    exit 1
  fi

  if ! out=$(make -f "$makefile" clean 2>&1); then
    echo "Error during calling make clean: $out"
    exit 1
  fi

  echo ">> make generate"
  generated=true
  if ! out=$(make -f "$makefile" generate 2>&1); then
    echo "Error during calling make generate: $out"
    exit 1
  fi
  new_status="$(git status -s)"

  if [[ "$old_status" != "$new_status" ]]; then
    echo "make generate needs to be run:"
    echo "$new_status"
    exit 1
  fi

  repo_root="$(git rev-parse --show-toplevel)"
  if [[ -d "$repo_root/vendor" ]]; then
    echo ">> make revendor"
    if ! out=$(make -f "$makefile" revendor 2>&1); then
      echo "Error during calling make revendor: $out"
      exit 1
    fi
    new_status="$(git status -s)"

    if [[ "$old_status" != "$new_status" ]]; then
      echo "make revendor needs to be run:"
      echo "$new_status"
      exit 1
    fi
  else
    echo ">> make tidy"
    if ! out=$(make -f "$makefile" tidy 2>&1); then
      echo "Error during calling make tidy: $out"
      exit 1
    fi
    new_status="$(git status -s)"

    if [[ "$old_status" != "$new_status" ]]; then
      echo "make tidy needs to be run:"
      echo "$new_status"
      exit 1
    fi
  fi
else
  echo "No git detected, cannot run make check-generate"
fi
exit 0
