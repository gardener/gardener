--[[ 
SPDX-FileCopyrightText: Contributors to the Gardener project
SPDX-License-Identifier: Apache-2.0
--]]
function add_kubernetes_namespace_name_to_record(tag, timestamp, record)
    local ns = record["namespace_name"]
    --[[ If parser did not capture any namespace_name from logs do nothing and return 0 to ensure we don't override the record --]]
    if ns == nil or ns == "" then
        return 0, timestamp, record
    end
    if record["kubernetes"] == nil or type(record["kubernetes"]) ~= "table" then
        record["kubernetes"] = {}
    end
    record["kubernetes"]["namespace_name"] = ns
    record["kubernetes"]["container_name"] = "istio-proxy"
    record["namespace_name"] = nil
    return 1, timestamp, record
end
