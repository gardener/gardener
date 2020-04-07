#!/bin/bash
#
# Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

echo "> Generate Check"

makefile="$1/Makefile"
check_branch="__generate_check"
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
    echo "Already on go generate check branch, aborting"
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
  git commit -q --allow-empty -m 'check-generate checkpoint'

  old_status="$(git status -s)"
  if ! out=$(make -f "$makefile" clean 2>&1); then
    echo "Error during calling make clean: $out"
    exit 1
  fi
  generated=true

  if ! out=$(make -f "$makefile" generate 2>&1); then
    echo "Error during calling make generate: $out"
    exit 1
  fi
  new_status="$(git status -s)"

  if [[ "$old_status" != "$new_status" ]]; then
    echo "go generate needs to be run:"
    echo "$new_status"
    exit 1
  fi
else
  echo "No git detected, cannot run generate check"
fi
exit 0
