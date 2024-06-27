---
weight: 15
title: DNS Autoscaling
---

# DNS Autoscaling

This is a short guide describing different options how to automatically scale CoreDNS in the shoot cluster.

## Background

Currently, Gardener uses CoreDNS as DNS server. Per default, it is installed as a deployment into the shoot cluster that is auto-scaled horizontally to cover for QPS-intensive applications. However, doing so does not seem to be enough to completely circumvent DNS bottlenecks such as:

- Cloud provider limits for DNS lookups.
- Unreliable UDP connections that forces a period of timeout in case packets are dropped.
- Unnecessary node hopping since CoreDNS is not deployed on all nodes, and as a result DNS queries end-up traversing multiple nodes before reaching the destination server.
- Inefficient load-balancing of services (e.g., round-robin might not be enough when using IPTables mode).
- Overload of the CoreDNS replicas as the maximum amount of replicas is fixed.
- and more ...

As an alternative with extended configuration options, Gardener provides cluster-proportional autoscaling of CoreDNS. This guide focuses on the configuration of cluster-proportional autoscaling of CoreDNS and its advantages/disadvantages compared to the horizontal
autoscaling.
Please note that there is also the option to use a [node-local DNS cache](../dns/node-local-dns.md), which helps mitigate potential DNS bottlenecks (see [Trade-offs in conjunction with NodeLocalDNS](#trade-offs-in-conjunction-with-nodelocaldns) for considerations regarding using NodeLocalDNS together with one of the CoreDNS autoscaling approaches).

## Configuring Cluster-Proportional DNS Autoscaling

All that needs to be done to enable the usage of cluster-proportional autoscaling of CoreDNS is to set the corresponding option (`spec.systemComponents.coreDNS.autoscaling.mode`) in the `Shoot` resource to `cluster-proportional`:

```yaml
...
spec:
  ...
  systemComponents:
    coreDNS:
      autoscaling:
        mode: cluster-proportional
...
```

To switch back to horizontal DNS autoscaling, you can set the `spec.systemComponents.coreDNS.autoscaling.mode` to `horizontal` (or remove the `coreDNS` section).

Once the cluster-proportional autoscaling of CoreDNS has been enabled and the Shoot cluster has been reconciled afterwards, a ConfigMap called `coredns-autoscaler` will be created in the `kube-system` namespace with the default settings. The content will be similar to the following:

```yaml
linear: '{"coresPerReplica":256,"min":2,"nodesPerReplica":16}'
```

It is possible to adapt the ConfigMap according to your needs in case the defaults do not work as desired. The number of CoreDNS replicas is calculated according to the following formula:

```
replicas = max( ceil( cores × 1 / coresPerReplica ) , ceil( nodes × 1 / nodesPerReplica ) )
```

Depending on your needs, you can adjust `coresPerReplica` or `nodesPerReplica`, but it is also possible to override `min` if required.

## Trade-Offs of Horizontal and Cluster-Proportional DNS Autoscaling

The horizontal autoscaling of CoreDNS as implemented by Gardener is fully managed, i.e., you do not need to perform any configuration changes. It scales according to the CPU usage of CoreDNS replicas, meaning that it will create new replicas if the existing ones are under heavy load. This approach scales between 2 and 5 instances, which is sufficient for most workloads. In case this is not enough, the cluster-proportional autoscaling approach can be used instead, with its more flexible configuration options.

The cluster-proportional autoscaling of CoreDNS as implemented by Gardener is fully managed, but allows more configuration options to adjust the default settings to your individual needs. It scales according to the cluster size, i.e., if your cluster grows in terms of cores/nodes so will the amount of CoreDNS replicas. However, it does not take the actual workload, e.g., CPU consumption, into account.

Experience shows that the horizontal autoscaling of CoreDNS works for a variety of workloads. It does reach its limits if a cluster has a high amount of DNS requests, though. The cluster-proportional autoscaling approach allows to fine-tune the amount of CoreDNS replicas. It helps to scale in clusters of changing size. However, please keep in mind that you need to cater for the maximum amount of DNS requests as the replicas will not be adapted according to the workload, but only according to the cluster size (cores/nodes).

## Trade-Offs in Conjunction with NodeLocalDNS

Using a [node-local DNS cache](../dns/node-local-dns.md) can mitigate a lot of the potential DNS related problems. It works fine with a DNS workload that can be handle through the cache and reduces the inter-node DNS communication. As [node-local DNS cache](../dns/node-local-dns.md) reduces the amount of traffic being sent to the cluster's CoreDNS replicas, it usually works fine with horizontally scaled CoreDNS. Nevertheless, it also works with CoreDNS scaled in a cluster-proportional approach. In this mode, though, it might make sense to adapt the default settings as the CoreDNS workload is likely significantly reduced.

Overall, you can view the DNS options on a scale. Horizontally scaled DNS provides a small amount of DNS servers. Especially for bigger clusters, a cluster-proportional approach will yield more CoreDNS instances and hence may yield a more balanced DNS solution. By adapting the settings you can further increase the amount of CoreDNS replicas. On the other end of the spectrum, a [node-local DNS cache](../dns/node-local-dns.md) provides DNS on every node and allows to reduce the amount of (backend) CoreDNS instances regardless if they are horizontally or cluster-proportionally scaled.
