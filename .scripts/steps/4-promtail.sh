#!/bin/bash
# Replace promtail with valitail in folder names

find ./* -type d -name promtail -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' folder
  do
    mv "$folder" "${folder/promtail/valitail}"
  done
