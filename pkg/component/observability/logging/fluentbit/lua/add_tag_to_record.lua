-- SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
--
-- SPDX-License-Identifier: Apache-2.0
function add_tag_to_record(tag, timestamp, record)
  record["tag"] = tag
  return 1, timestamp, record
end
