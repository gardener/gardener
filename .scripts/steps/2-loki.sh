#!/bin/bash
# Replace the Loki and Promtail container images with that of Vali and Valitail

git grep -z -l "repository: eu.gcr.io/gardener-project/3rd/grafana/loki" -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i -E 's|repository: eu.gcr.io/gardener-project/3rd/grafana/loki|repository: ghcr.io/credativ/vali|
                      s|repository: eu.gcr.io/gardener-project/3rd/grafana/promtail|repository: ghcr.io/credativ/valitail|
                      s/tag: "2.2.1"/tag: "v2.2.5"/'
