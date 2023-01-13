#!/bin/bash
# Replace GF_ with PL_ in file contents

# GF_ is the Grafana prefix that is used in environment variables.

git grep -z -l " GF_" -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i 's/GF_/PL_/g'
