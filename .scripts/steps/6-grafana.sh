#!/bin/bash
# Replace grafana with plutono in file contents

git grep -z -l grafana -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i 's/grafana/plutono/g'
