# Logging Stack

This document contains logging stack related how-tos and configuration options for developers.

## Expose Logs for Component to User Plutono

Exposing logs for a new component to the User's Plutono is described in the [How to Expose Logs to the Users](../extensions/logging-and-monitoring.md#how-to-expose-logs-to-the-users) section.

## Configuration

### Fluent-bit

The Fluent-bit configurations can be found on `pkg/component/observability/logging/fluentoperator/customresources`
There are six different specifications:

* FluentBit: Defines the fluent-bit DaemonSet specifications
* ClusterFluentBitConfig: Defines the labelselectors of the resources which fluent-bit will match
* ClusterInput: Defines the location of the input stream of the logs
* ClusterOutput: Defines the location of the output source (Vali for example)
* ClusterFilter: Defines filters which match specific keys
* ClusterParser: Defines parsers which are used by the filters

### Vali

The Vali configurations can be found on `charts/seed-bootstrap/charts/vali/templates/vali-configmap.yaml`

The main specifications there are:

* Index configuration: Currently the following one is used:

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

* `from`: Is the date from which logs collection is started. Using a date in the past is okay.
* `store`: The DB used for storing the index.
* `object_store`: Where the data is stored.
* `schema`: Schema version which should be used (v11 is currently recommended).
* `index.prefix`: The prefix for the index.
* `index.period`: The period for updating the indices.

**Adding a new index happens with new config block definition. The `from` field should start from the current day + previous `index.period` and should not overlap with the current index. The `prefix` also should be different.**

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

`table_manager.retention_period` is the living time for each log message. Vali will keep messages for (`table_manager.retention_period` - `index.period`) time due to specification in the Vali implementation.

### Plutono

This is the Vali configuration that Plutono uses:

```
    - name: vali
      type: vali
      access: proxy
      url: http://logging.{{ .Release.Namespace }}.svc:3100
      jsonData:
        maxLines: 5000
```

* `name`: Is the name of the datasource.
* `type`: Is the type of the datasource.
* `access`: Should be set to proxy.
* `url`: Vali's url
* `svc`: Vali's port
* `jsonData.maxLines`: The limit of the log messages which Plutono will show to the users.

**Decrease this value if the browser works slowly!**
