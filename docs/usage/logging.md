# Logging stack

### Motivation
Kubernetes uses the underlying container runtime logging, which does not persist logs for stopped and destroyed containers. This makes it difficult to investigate issues in the very common case of not running containers. Gardener provides a solution to this problem for the managed cluster components, by introducing its own logging stack.


### Components:
![](images/logging-architecture.png)
* A Fluent-bit daemonset which works like a log collector and custom custom Golang plugin which spreads log messages to their Loki instances
* One Loki Statefulset in the `garden` namespace which contains logs for the seed cluster and one per shoot namespace which contains logs for shoot's controlplane.
* One Grafana Deployment in `garden` namespace and two Deployments per shoot namespace (one exposed to the end users and one for the operators). Grafana is the UI component used in the logging stack.

### How to access the logs
The first step is to authenticate in front of the Grafana ingress. The secret with the credentials can be found in `garden-<project>` namespace under `<shoot-name>.monitoring`.
Logs are accessible via Grafana UI. Its URL can be found in the `Logging and Monitoring` section of a cluster in the Gardener Dashboard.

There are two methods to explore logs:
* The first option is to use the `Explore` view (available at the left side of the screen). It is used for creating log queries using the predefined filters in Loki. For example: 
`{pod_name='prometheus-0'}`
or with regex:
`{pod_name=~'prometheus.+'}`

* The other option is to use `Dashboards` panel. There are custom dashboards for pod logs with one selector field for `pod_name` and one `search` field. The `search` field allows to filter the logs for a particular string. The following dashboards can be used for logs:

  * Garden Grafana
    * Pod Logs
    * Extensions
    * Systemd Logs
  * User Grafana
    * Kube Apiserver
    * Kube Controller Manager
    * Kube Scheduler
    * Cluster Autoscaler  * Operator Grafana
  * Operator Grafana
    * All user's dashboards
    * Kubernetes Pods

### Expose logs for component to User Grafana
Exposing logs for a new component to the User's Grafana is described [here](../extensions/logging-and-monitoring.md#how-to-expose-logs-to-the-users)
### Configuration
#### Fluent-bit

The Fluent-bit configurations can be found on `charts/seed-bootstrap/charts/fluent-bit/templates/fluent-bit-configmap.yaml`
There are five different specifications:

* SERVICE: Defines the location of the server specifications
* INPUT: Defines the location of the input stream of the logs
* OUTPUT: Defines the location of the output source (Loki for example)
* FILTER: Defines filters which match specific keys
* PARSER: Defines parsers which are used by the filters

#### Loki
The Loki configurations can be found on `charts/seed-bootstrap/charts/loki/templates/loki-configmap.yaml`

The main specifications there are:

* Index configuration: Currently is used the following one:
```
    schema_config:
      configs:
      - from: 2018-04-15
        store: boltdb
        object_store: filesystem
        schema: v11
        index:
          prefix: index_
          period: 24h
```
* `from`: is the date from which logs collection is started. Using a date in the past is okay.
* `store`: The DB used for storing the index.
* `object_store`: Where the data is stored
* `schema`: Schema version which should be used (v11 is currently recommended)
* `index.prefix`: The prefix for the index.
* `index.period`: The period for updating the indices

**Adding of new index happens with new config block definition. `from` field should start from the current day + previous `index.period` and should not overlap with the current index. The `prefix` also should be different**
```
    schema_config:
      configs:
      - from: 2018-04-15
        store: boltdb
        object_store: filesystem
        schema: v11
        index:
          prefix: index_
          period: 24h
      - from: 2020-06-18
        store: boltdb
        object_store: filesystem
        schema: v11
        index:
          prefix: index_new_
          period: 24h
```

* chunk_store_config Configuration
```
    chunk_store_config: 
      max_look_back_period: 336h
```
**`chunk_store_config.max_look_back_period` should be the same as the `retention_period`**

* table_manager Configuration
```
    table_manager:
      retention_deletes_enabled: true
      retention_period: 336h
```
`table_manager.retention_period` is the living time for each log message. Loki will keep messages for sure for (`table_manager.retention_period` - `index.period`) time due to specification in the Loki implementation.

#### Grafana
The Grafana configurations can be found on  `charts/seed-bootstrap/charts/templates/grafana/grafana-datasources-configmap.yaml` and 
`charts/seed-monitoring/charts/grafana/tempates/grafana-datasources-configmap.yaml`

This is the Loki configuration that Grafana uses:

```
    - name: loki
      type: loki
      access: proxy
      url: http://loki.{{ .Release.Namespace }}.svc:3100
      jsonData:
        maxLines: 5000
```

* `name`: is the name of the datasource
* `type`: is the type of the datasource
* `access`: should be set to proxy
* `url`: Loki's url
* `svc`: Loki's port
* `jsonData.maxLines`: The limit of the log messages which Grafana will show to the users.

**Decrease this value if the browser works slowly!**
