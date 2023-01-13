#!/bin/bash
# Replace loki with vali in folder names

find ./* -type d -name loki -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' folder
  do
    mv "$folder" "${folder/loki/vali}"
  done
