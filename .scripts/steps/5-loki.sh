#!/bin/bash
# Replace loki with vali in file names

find ./* -type f -name '*loki*' -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' file
  do
    mv "$file" "${file/loki/vali}"
  done
