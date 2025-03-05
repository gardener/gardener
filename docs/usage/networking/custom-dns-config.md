---
title: Custom DNS Configuration
---

# Custom DNS Configuration

Gardener provides Kubernetes-Clusters-As-A-Service where all the system components (e.g., kube-proxy, networking, dns) are managed.
As a result, Gardener needs to ensure and auto-correct additional configuration to those system components to avoid unnecessary down-time.

In some cases, auto-correcting system components can prevent users from deploying applications on top of the cluster that requires bits of customization, DNS configuration can be a good example.

To allow for customizations for DNS configuration (that could potentially lead to downtime) while having the option to "undo", we utilize the `import` plugin from CoreDNS [1].
which enables in-line configuration changes.

## How to use

To customize your CoreDNS cluster config, you can simply edit a `ConfigMap` named `coredns-custom` in the `kube-system` namespace.
By editing, this `ConfigMap`, you are modifying CoreDNS configuration, therefore care is advised.

For example, to apply new config to CoreDNS that would point all `.global` DNS requests to another DNS pod, simply edit the configuration as follows:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-custom
  namespace: kube-system
data:
  istio.server: |
    global:8053 {
            errors
            cache 30
            forward . 1.2.3.4
        }
  corefile.override: |
         # <some-plugin> <some-plugin-config>
         debug
         whoami
```

The port number 8053 in `global:8053` is the specific port that CoreDNS is bound to and cannot be changed to any other port if it should act on ordinary name resolution requests from pods. Otherwise, CoreDNS will open a second port, but you are responsible to direct the traffic to this port. `kube-dns` service in `kube-system` namespace will direct name resolution requests within the cluster to port 8053 on the CoreDNS pods.
Moreover, additional network policies are needed to allow corresponding ingress traffic to CoreDNS pods.
In order for the destination DNS server to be reachable, it must listen on port 53 as it is required by network policies. Other ports are only possible if additional network policies allow corresponding egress traffic from CoreDNS pods.

It is important to have the `ConfigMap` keys ending with `*.server` (if you would like to add a new server) or `*.override`
if you want to customize the current server configuration (it is optional setting both).

## Warning
Be careful when overriding plugins `log`, `forward` or `cache`.
- Increasing log level can lead to increased load/reduced throughput. 
- Changing the forward target may lead to unexpected results. 
- Playing with the cache settings can impact the timeframe how long it takes for changes to become visible.

`*.override` and `*.server` data points from `coredns-custom` `ConfigMap` are imported into Corefile as follows.
Please consult `coredns` [plugin documentation](https://coredns.io/plugins/) for potential side-effects.
```yaml
.:8053 {
  health {
      lameduck 15s
  }
  ready
  [search-rewrites]
  kubernetes[clusterDomain]in-addr.arpa ip6.arpa {
      pods insecure
      fallthrough in-addr.arpa ip6.arpa
      ttl 30
  }
  prometheus :9153
  loop
  import custom/*.override
  errors
  log . {
      class error
  }
  forward . /etc/resolv.conf
  cache 30
  reload
  loadbalance round_robin
}
import custom/*.server
```

## [Optional] Reload CoreDNS

As Gardener is configuring the `reload` [plugin](https://coredns.io/plugins/reload/) of CoreDNS a restart of the CoreDNS components is typically not necessary to propagate `ConfigMap` changes. However, if you don't want to wait for the default (30s) to kick in, you can roll-out your CoreDNS deployment using:

```bash
kubectl -n kube-system rollout restart deploy coredns
```

This will reload the config into CoreDNS.

The approach we follow here was inspired by AKS's approach [2].

## Anti-Pattern

Applying a configuration that is in-compatible with the running version of CoreDNS is an anti-pattern (sometimes plugin configuration changes,
simply applying a configuration can break DNS).

If incompatible changes are applied by mistake, simply delete the content of the `ConfigMap` and re-apply.
This should bring the cluster DNS back to functioning state.

## Node Local DNS

Custom DNS configuration] may not work as expected in conjunction with `NodeLocalDNS`.
With `NodeLocalDNS`, ordinary DNS queries targeted at the upstream DNS servers, i.e. non-kubernetes domains,
will not end up at CoreDNS, but will instead be directly sent to the upstream DNS server. Therefore, configuration
applying to non-kubernetes entities, e.g. the `istio.server` block in the
[custom DNS configuration](custom-dns-config.md) example, may not have any effect with `NodeLocalDNS` enabled.
If this kind of custom configuration is required, forwarding to upstream DNS has to be disabled.
This can be done by setting the option (`spec.systemComponents.nodeLocalDNS.disableForwardToUpstreamDNS`) in the `Shoot` resource to `true`:
```yaml
...
spec:
  ...
  systemComponents:
    nodeLocalDNS:
      enabled: true
      disableForwardToUpstreamDNS: true
...
```

## References

[1] [Import plugin](https://github.com/coredns/coredns/tree/master/plugin/import)
[2] [AKS Custom DNS](https://docs.microsoft.com/en-us/azure/aks/coredns-custom)
