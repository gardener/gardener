#!/bin/bash
# Replace grafana with plutono in folder names

find ./* -type d -name grafana -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' folder
  do
    mv "$folder" "${folder/grafana/plutono}"
  done

find ./* -type l -not -path './vendor/*' -print0 \
| while IFS= read -r -d '' link
  do
    target=$(readlink "$link")
    rm "$link"
    ln -s "${target/grafana/plutono}" "$link"
  done
