#!/bin/bash

i=0
for step in "$(dirname "$(realpath "$0")")"/steps/*; do
  "$step"
  git add -A
  (( i++ ))
  sed -E "1d
          2s/^# /# $i. /
          s/^#( |$)//" < "$step" \
  | git commit --allow-empty -F -
done
