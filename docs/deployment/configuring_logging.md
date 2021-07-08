# Configuring the Logging stack via Gardenlet configurations

# Enable the Logging

Logging feature gate must be enabled in order to install the Gardener Logging stack.

In the Gardenlet configuration add:
```yaml
featureGates:
  Logging: true
```

From now on each Seed is going to have a logging stack which will collect logs from all pods and some systemd services. Logs related to Shoots with `testing` purpose are dropped in the `fluent-bit` output plugin. Shoots with a purpose different than `testing` have the same type of log aggregator (but different instance) as the Seed. The logs can be viewed in the Grafana in the `garden` namespace for the Seed components and in the respective shoot control plane namespaces.

# Enable logs from the Shoot's node systemd services.

The logs from the systemd services on each node can be retrieved by enabling the `logging.shootNodeLogging` option in the Gardenlet configuration:
```yaml
featureGates:
  Logging: true
logging:
  shootNodeLogging:
    shootPurposes:
    - "evaluation"
    - "deployment"
```

Under the `shootPurpose` section just list all the shoot purposes for which the Shoot node logging feature will be enabled. Specifying the `testing` purpose has no effect because this purpose prevents the logging stack installation.
Logs can be  viewed in the operator Grafana!
The dedicated labels are `unit`, `syslog_identifier` and `nodename` in the `Explore` menu.

# Configuring the log processor

Under `logging.fluentBit` there is three optional sections.
- `input`: This overwrite the input configuration of the fluent-bit log processor.
 - `output`: This overwrite the output configuration of the fluent-bit log processor.
 - `service`: This overwrite the service configuration of the fluent-bit log processor.

```yaml
featureGates:
  Logging: true
logging:
  fluentBit:
    output: |-
      [Output]
          ...
    input: |-
      [Input]
          ...
    service: |-
      [Service]
          ...
```

# Configuring the Loki PriorityClass

The central Loki, which is in the `garden` namespace, contains all the logs from the most important seed components. When the central Loki `PriorityClass` is with low value then its pods can be preempted and often moved from one node to another while Kubernetes tries to free space for more important pods. The persistent volume will be detached/attached again as well. Based on the performance of the underlying infrastructure, this leads to great central Loki downtime. To give greater priority of the seed Loki you can use the `logging.loki.garden.priority` option.

```yaml
featureGates:
  Logging: true
logging:
  loki:
    garden:
      priority: 100
```