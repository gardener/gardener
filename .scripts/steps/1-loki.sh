#!/bin/bash
# Replace the Loki Github page with that of Vali

git grep -z -l github\\.com/grafana/loki -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i -E 's|github.com/grafana/loki|github.com/credativ/vali|g'
