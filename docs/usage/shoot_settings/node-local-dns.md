---
title: NodeLocalDNS Configuration
weight: 14
---

# NodeLocalDNS Configuration

This is a short guide describing how to enable DNS caching on the shoot cluster nodes.

## Background

Currently in Gardener we are using CoreDNS as a deployment that is auto-scaled horizontally to cover for QPS-intensive applications. However, doing so does not seem to be enough to completely circumvent DNS bottlenecks such as:

- Cloud provider limits for DNS lookups.
- Unreliable UDP connections that forces a period of timeout in case packets are dropped.
- Unnecessary node hopping since CoreDNS is not deployed on all nodes, and as a result DNS queries end-up traversing multiple nodes before reaching the destination server.
- Inefficient load-balancing of services (e.g., round-robin might not be enough when using IPTables mode)
- and more ...

To workaround the issues described above, `node-local-dns` was introduced. The architecture is described below. The idea is simple:

- For new queries, the connection is upgraded from UDP to TCP and forwarded towards the cluster IP for the original CoreDNS server.
- For previously resolved queries, an immediate response from the same node where the requester workload / pod resides is provided.

![node-local-dns-architecture](./images/node-local-dns.png)

## Configuring NodeLocalDNS

All that needs to be done to enable the usage of the `node-local-dns` feature is to set the corresponding option (`spec.systemComponents.nodeLocalDNS.enabled`) in the `Shoot` resource to `true`:

```yaml
...
spec:
  ...
  systemComponents:
    nodeLocalDNS:
      enabled: true
...
```

It is worth noting that: 

- When migrating from IPVS to IPTables, existing pods will continue to leverage the node-local-dns cache.
- When migrating from IPtables to IPVS, only newer pods will be switched to the node-local-dns cache.
- During the reconfiguration of the node-local-dns there might be a short disruption in terms of domain name resolution depending on the setup. Usually, DNS requests are repeated for some time as UDP is an unreliable protocol, but that strictly depends on the application/way the domain name resolution happens. It is recommended to let the shoot be reconciled during the next maintenance period.
- Enabling or disabling node-local-dns triggers a rollout of all shoot worker nodes, see also [this document](shoot_updates.md#rolling-update-triggers).

For more information about `node-local-dns`, please refer to the [KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-network/1024-nodelocal-cache-dns/README.md) or to the [usage documentation](https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/). 

## Known Issues

Custom DNS configuration may not work as expected in conjunction with `NodeLocalDNS`.
Please refer to [Custom DNS Configuration](custom-dns-config.md#node-local-dns).
