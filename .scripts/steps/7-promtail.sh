#!/bin/bash
# Replace Promtail with Valitail in file contents

git grep -z -l Promtail -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i 's/Promtail/Valitail/g'
