--[[ 
SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
SPDX-License-Identifier: Apache-2.0
--]]
function add_kubernetes_namespace_name_to_record(tag, timestamp, record)
    if record["kubernetes"] == nil or type(record["kubernetes"]) ~= "table" then
    record["kubernetes"] = {}
    end
    if record["namespace_name"] ~= nil then
    record["kubernetes"]["namespace_name"] = record["namespace_name"]
    record["kubernetes"]["container_name"] = "istio-proxy"
    record["namespace_name"] = nil
    end
    return 1, timestamp, record
end