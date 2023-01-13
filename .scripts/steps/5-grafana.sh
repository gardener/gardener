#!/bin/bash
# Replace grafana with plutono in file names

find ./* -type f -name '*grafana*' -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' file
  do
    mv "$file" "${file/grafana/plutono}"
  done
