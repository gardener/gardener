#!/bin/bash
# Use the Plutono Github page as a generic web link

# The generic web link 'https://github.com/credativ/plutono'
# is not as specific as some of the previous links, but we do not
# have documentation for the Plutono and Vali projects, so it
# is up to the reader to find the matching documentation pages
# by following the links from the Plutono landing page.

git grep -z -l grafana\\.com -- ':!/vendor' ':!/.scripts' ':!NOTICE.md' \
| xargs -0 sed -i -E 's|grafana.com([^) ]*)|github.com/credativ/plutono|g'
