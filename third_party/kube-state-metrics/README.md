# Why do we keep a copy of kube-state-metrics types here?

The struct definitions from `k8s.io/kube-state-metrics/v2/pkg/customresourcestate` and `k8s.io/kube-state-metrics/v2/pkg/metric` (version `v2.19.0`) are manually copied here to solve a transitive dependency issue.

`k8s.io/client-go@v0.36` added two methods to the `cache.Store` interface (see [kubernetes/kubernetes#134827](https://github.com/kubernetes/kubernetes/pull/134827)):
- `Bookmark(rv string)`
- `LastStoreSyncResourceVersion() string` 

The `MetricsStore` type in kube-state-metrics does not yet implement this method, causing a build failure when the module is included in the dependency graph. The upstream fix is tracked in [kubernetes/kube-state-metrics#2965](https://github.com/kubernetes/kube-state-metrics/pull/2965).

We only uses these packages for their type definitions to build a ConfigMap that is passed as configuration to the kube-state-metrics binary. No runtime logic from kube-state-metrics is used.

TODO(tobschli): Remove this copy and restore the direct dependency on `k8s.io/kube-state-metrics/v2` once [kubernetes/kube-state-metrics#2965](https://github.com/kubernetes/kube-state-metrics/pull/2965) is merged and released.
