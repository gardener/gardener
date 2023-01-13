#!/bin/bash
# Replace Grafana with Plutono in file contents

git grep -z -l Grafana -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i 's/Grafana/Plutono/g'
