# How to create log parser for container into fluent-bit


If our log message is parsed correctly, it has to be showed in Kibana like this:

```
{
  "_source": {
    "kubernetes": {
      "pod_name": "calico-node-z54s9",
      "pod_id": "c4ac4458-17e1-11e9-870f-0a114344f55b",
      "namespace_name": "kube-system"
    },
    "source": "int_dataplane.go 765",
    "log": "Finished applying updates to dataplane. msecToApply=1.325953",
    "severity": "INF",
    "pid": "58",
    "@timestamp": "2019-01-16T16:06:00.543688505+00:00"
  },
}
```
Otherwise it will looks like this:
```
{
  "_source": {
    "log": "2019-01-16 16:09:59.826 [INFO][66] health.go 150: Overall health summary=u0026health.HealthReport{Live:true, Ready:true}\n",
    "kubernetes": {
      "pod_name": "calico-node-ht6nh",
      "namespace_name": "kube-system",
      "pod_id": "97208bcb-15f9-11e9-bb8c-4632b18c254a",
      },
    },
  },
}
```


### Lets make a custom parser now

- First of all we need to know how does the log for the specific container look like (for example lets take a log from the `alertmanager` : 
`level=info ts=2019-01-28T12:33:49.362015626Z caller=main.go:175 build_context="(go=go1.11.2, user=root@4ecc17c53d26, date=20181109-15:40:48)`)

- We can see that this log contains 4 subfields(severity=info, timestamp=2019-01-28T12:33:49.362015626Z, source=main.go:175 and the actual message).
So we have to write a regex which matches this log in 4 groups(We can use https://regex101.com/ like helping tool). So for this purpose our regex
looks like this:
```
^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt].*[zZ])\s+caller=(?<source>[^\s]*+)\s+(?<log>.*)
```

- Now we have to create correct time format for the timestamp(We can use this site for this purpose: http://ruby-doc.org/stdlib-2.4.1/libdoc/time/rdoc/Time.html#method-c-strptime).
So our timestamp matches correctly the following format: 
```
%Y-%m-%dT%H:%M:%S.%L
```

- It's a time to apply our new regex into fluent-bit configuration. Go to fluent-bit-configmap.yaml and create new filter using the 
following template:
```
[FILTER]
        Name                parser
        Match               kubernetes.<< pod-name >>*<< container-name >>*
        Key_Name            log
        Parser              << parser-name >>
        Reserve_Data        True
```
```
EXAMPLE
[FILTER]
        Name                parser
        Match               kubernetes.alertmanager*alertmanager*
        Key_Name            log
        Parser              alermanagerParser
        Reserve_Data        True
```
- Now lets check if there is already exists parser with such a regex and time format that we need. if not, let`s create one:
```
[PARSER]
        Name        << parser-name >>
        Format      regex
        Regex       << regex >>
        Time_Key    time
        Time_Format << time-format >>
```
```
EXAMPLE
[PARSER]
        Name        alermanagerParser
        Format      regex
        Regex       ^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt].*[zZ])\s+caller=(?<source>[^\s]*+)\s+(?<log>.*)
        Time_Key    time
        Time_Format %Y-%m-%dT%H:%M:%S.%L
```
```
Follow your development setup to validate that parsers are working correctly.
```
