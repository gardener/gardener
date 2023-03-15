# How to Create Log Parser for Container into fluent-bit

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

## Create a Custom Parser

- First of all, we need to know how the log for the specific container looks like (for example, lets take a log from the `alertmanager` :
`level=info ts=2019-01-28T12:33:49.362015626Z caller=main.go:175 build_context="(go=go1.11.2, user=root@4ecc17c53d26, date=20181109-15:40:48)`)

- We can see that this log contains 4 subfields(severity=info, timestamp=2019-01-28T12:33:49.362015626Z, source=main.go:175 and the actual message).
So we have to write a regex which matches this log in 4 groups(We can use https://regex101.com/ like helping tool). So, for this purpose our regex looks like this:

```text
^level=(?<severity>\w+)\s+ts=(?<time>\d{4}-\d{2}-\d{2}[Tt].*[zZ])\s+caller=(?<source>[^\s]*+)\s+(?<log>.*)
```

- Now we have to create correct time format for the timestamp (We can use this site for this purpose: http://ruby-doc.org/stdlib-2.4.1/libdoc/time/rdoc/Time.html#method-c-strptime).
So our timestamp matches correctly the following format:

```text
%Y-%m-%dT%H:%M:%S.%L
```

- It's time to apply our new regex into fluent-bit configuration. To achieve that we can just deploy in the cluster where the `fluent-operator` is deployed the following custom resources:

```yaml
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterFilter
metadata:
  labels:
    fluentbit.gardener/type: seed
  name: << pod-name >>--(<< container-name >>)
spec:
  filters:
  - parser:
      keyName: log
      parser: << container-name >>-parser
      reserveData: true
  match: kubernetes.<< pod-name >>*<< container-name >>*
```

```yaml
EXAMPLE
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterFilter
metadata:
  labels:
    fluentbit.gardener/type: seed
  name: alertmanager
spec:
  filters:
  - parser:
      keyName: log
      parser: alertmanager-parser
      reserveData: true
  match: "kubernetes.alertmanager*alertmanager*"
```

- Now lets check if there already exists `ClusterParser` with such a regex and time format that we need. If it doesn't, create one:

```yaml
apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterParser
metadata:
  name:  << container-name >>-parser
  labels:
    fluentbit.gardener/type: "seed"
spec:
  regex:
    timeKey: time
    timeFormat: << time-format >>
    regex: "<< regex >>"
```

```yaml
EXAMPLE

apiVersion: fluentbit.fluent.io/v1alpha2
kind: ClusterParser
metadata:
  name: alermanager-parser
  labels:
    fluentbit.gardener/type: "seed"
spec:
  regex:
    timeKey: time
    timeFormat: "%Y-%m-%dT%H:%M:%S.%L"
    regex: "^level=(?<severity>\\w+)\\s+ts=(?<time>\\d{4}-\\d{2}-\\d{2}[Tt].*[zZ])\\s+caller=(?<source>[^\\s]*+)\\s+(?<log>.*)"
```

```text
Follow your development setup to validate that the parsers are working correctly.
```
