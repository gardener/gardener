-- SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
--
-- SPDX-License-Identifier: Apache-2.0
function stringify_records_nest_log(tag, timestamp, record)
    if record["log"] ~= nil and type(record["log"]) == "table" then
        local json_str = cjson.encode(record["log"])
        record["log"] = json_str
    end
    return 1, timestamp, record
end
