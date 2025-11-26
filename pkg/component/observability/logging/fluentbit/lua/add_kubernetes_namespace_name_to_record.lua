-- SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
--
-- SPDX-License-Identifier: Apache-2.0
function add_kubernetes_namespace_name_to_record(tag, timestamp, record)
    -- Initialize kubernetes key if missing or wrong type
    if record["kubernetes"] == nil or type(record["kubernetes"]) ~= "table" then
    record["kubernetes"] = {}
    end

    -- Set kubernetes.namespace_name to the value of namespace_name if exists
    if record["namespace_name"] ~= nil then
    record["kubernetes"]["namespace_name"] = record["namespace_name"]

    -- TODO if container_name is not set then the lua filter ordering needs to be checked
    record["kubernetes"]["container_name"] = "istio-proxy"

    -- remove the temporary field
    record["namespace_name"] = nil  -- remove the temporary field
    end

    return 1, timestamp, record
end