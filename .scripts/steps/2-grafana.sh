#!/bin/bash
# Replace the Grafana container image with that of Plutono

git grep -z -l "repository: eu.gcr.io/gardener-project/3rd/grafana/grafana" -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i -E 's|repository: eu.gcr.io/gardener-project/3rd/grafana/grafana|repository: ghcr.io/credativ/plutono|
                      s/tag: "7.5.17"/tag: "v7.5.21"/'
