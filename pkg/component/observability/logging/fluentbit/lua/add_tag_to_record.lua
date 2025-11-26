-- SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
--
-- SPDX-License-Identifier: Apache-2.0
function add_tag_to_record(tag, timestamp, record)
    local message = record["log"]

    -- Check if message is a string before calling string.match
    if type(message) ~= "string" then
        message = tostring(message)
    end

    local project, shoot_name = string.match(message, "shoot%-%-([^%-]+)%-%-([^%s]+)")

    if project and shoot_name then
        record["tag"] =  "shoot--" .. project .. "--" .. shoot_name
    else
        record["tag"] = tag
    end
    return 1, timestamp, record
end
