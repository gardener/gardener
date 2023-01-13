#!/bin/bash
# Replace the Grafana Github page with that of Plutono

git grep -z -l github\\.com/grafana/grafana -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i -E 's|github.com/grafana/grafana|github.com/credativ/plutono|g'
