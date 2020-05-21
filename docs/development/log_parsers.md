# How to create log parser for container into fluent-bit

If our log message is parsed correctly, it has to be showed in Grafana like this:

```jsonc
  {"log":"OpenAPI AggregationController: Processing item v1beta1.metrics.k8s.io","pid":"1","severity":"INFO","source":"controller.go:107"}
```

Otherwise it will looks like this:

```jsonc
{
  "log":"{
  \"level\":\"info\",\"ts\":\"2020-06-01T11:23:26.679Z\",\"logger\":\"gardener-resource-manager.health-reconciler\",\"msg\":\"Finished ManagedResource health checks\",\"object\":\"garden/provider-aws-dsm9r\"
  }\n"
  }
}
```

## Lets make a custom parser now

- First of all we need to know how does the log for the specific container look like (for example lets take a log from the `alertmanager` :
`level=info ts=2019-01-28T12:33:49.362015626Z caller=main.go:175 build_context="(go=go1.11.2, user=root@4ecc17c53d26, date=20181109-15:40:48)`)

- We can see that this log contains 4 subfields(severity=info, timestamp=2019-01-28T12:33:49.362015626Z, source=main.go:175 and the actual message).
So we have to write a regex which matches this log in 4 groups(We can use https://regex101.com/ like helping tool). So for this purpose our regex
looks like this:

```text
^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt].*[zZ])\s+caller=(?<source>[^\s]*+)\s+(?<log>.*)
```

- Now we have to create correct time format for the timestamp(We can use this site for this purpose: http://ruby-doc.org/stdlib-2.4.1/libdoc/time/rdoc/Time.html#method-c-strptime).
So our timestamp matches correctly the following format:

```text
%Y-%m-%dT%H:%M:%S.%L
```

- It's a time to apply our new regex into fluent-bit configuration. Go to fluent-bit-configmap.yaml and create new filter using the following template:

```text
[FILTER]
        Name                parser
        Match               kubernetes.<< pod-name >>*<< container-name >>*
        Key_Name            log
        Parser              << parser-name >>
        Reserve_Data        True
```

```text
EXAMPLE
[FILTER]
        Name                parser
        Match               kubernetes.alertmanager*alertmanager*
        Key_Name            log
        Parser              alermanagerParser
        Reserve_Data        True
```

- Now lets check if there is already exists parser with such a regex and time format that we need. if not, let`s create one:

```text
[PARSER]
        Name        << parser-name >>
        Format      regex
        Regex       << regex >>
        Time_Key    time
        Time_Format << time-format >>
```

```text
EXAMPLE
[PARSER]
        Name        alermanagerParser
        Format      regex
        Regex       ^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt].*[zZ])\s+caller=(?<source>[^\s]*+)\s+(?<log>.*)
        Time_Key    time
        Time_Format %Y-%m-%dT%H:%M:%S.%L
```

```text
Follow your development setup to validate that parsers are working correctly.
```
