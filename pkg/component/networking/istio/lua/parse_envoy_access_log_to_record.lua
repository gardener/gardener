function parse_envoy_access_log_to_record(tag, timestamp, record)
    local line = record["log"]
    if line == nil or type(line) ~= "string" then
        return 0, timestamp, record
    end
    local method, path, proto = string.match(line, '"(%S+)%s+(%S+)%s+([^"]-)"')
    local resp_code, resp_flags, bytes_rcvd, bytes_sent, duration, upstream_time = string.match(line, '"%s+(%d+)%s+(%S+)%s+%S+%s+%S+%s+"[^"]*"%s+(%d+)%s+(%d+)%s+(%d+)%s+(%S+)')
    local x_forwarded_for, user_agent, request_id, authority, upstream_host, upstream_cluster = string.match(line, '"([^"]*)"%s+"([^"]*)"%s+"([^"]*)"%s+"([^"]*)"%s+"([^"]*)"%s+(%S+)')
    if method ~= nil then
        record["method"] = method
        record["path"] = path
        record["protocol"] = proto
    end
    if resp_code ~= nil then
        record["response_code"] = resp_code
        record["response_flags"] = resp_flags
        record["bytes_received"] = tonumber(bytes_rcvd)
        record["bytes_sent"] = tonumber(bytes_sent)
        record["duration"] = tonumber(duration)
        record["upstream_service_time"] = upstream_time
    end
    if authority ~= nil then
        record["x_forwarded_for"] = x_forwarded_for
        record["user_agent"] = user_agent
        record["request_id"] = request_id
        record["authority"] = authority
        record["upstream_host"] = upstream_host
        record["upstream_cluster"] = upstream_cluster
    end
    if user_agent ~= nil and user_agent ~= "-" then
        local client = string.match(user_agent, "^([^/%s]+)")
        if client ~= nil then
            record["client"] = client
        end
    end
    return 1, timestamp, record
end
