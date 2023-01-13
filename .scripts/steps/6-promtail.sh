#!/bin/bash
# Replace promtail with valitail in file contents

git grep -z -l promtail -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i 's/promtail/valitail/g'
