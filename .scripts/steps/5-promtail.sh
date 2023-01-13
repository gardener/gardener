#!/bin/bash
# Replace promtail with valitail in file names

find ./* -type f -name '*promtail*' -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' file
  do
    mv "$file" "${file/promtail/valitail}"
  done
