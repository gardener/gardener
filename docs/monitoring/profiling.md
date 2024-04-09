# Profiling Gardener Components

Similar to Kubernetes, Gardener components support profiling using [standard Go tools](https://golang.org/doc/diagnostics#profiling) for analyzing CPU and memory usage by different code sections and more.
This document shows how to enable and use profiling handlers with Gardener components.

Enabling profiling handlers and the ports on which they are exposed differs between components.
However, once the handlers are enabled, they provide profiles via the same HTTP endpoint paths, from which you can retrieve them via `curl`/`wget` or directly using `go tool pprof`.
(You might need to use `kubectl port-forward` in order to access HTTP endpoints of Gardener components running in clusters.)

For example (gardener-controller-manager):
```bash
$ curl http://localhost:2718/debug/pprof/heap > /tmp/heap-controller-manager
$ go tool pprof /tmp/heap-controller-manager
Type: inuse_space
Time: Sep 3, 2021 at 10:05am (CEST)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof)
```
or 
```bash
$ go tool pprof http://localhost:2718/debug/pprof/heap
Fetching profile over HTTP from http://localhost:2718/debug/pprof/heap
Saved profile in /Users/timebertt/pprof/pprof.alloc_objects.alloc_space.inuse_objects.inuse_space.008.pb.gz
Type: inuse_space
Time: Sep 3, 2021 at 10:05am (CEST)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof)
```

## gardener-apiserver

`gardener-apiserver` provides the same flags as `kube-apiserver` for enabling profiling handlers (enabled by default):

```
--contention-profiling    Enable lock contention profiling, if profiling is enabled
--profiling               Enable profiling via web interface host:port/debug/pprof/ (default true)
```

The handlers are served on the same port as the API endpoints (configured via `--secure-port`).
This means that you will also have to authenticate against the API server according to the configured authentication and authorization policy.

## gardener-{admission-controller,controller-manager,scheduler,resource-manager}, gardenlet

`gardener-controller-manager`, `gardener-admission-controller`, `gardener-scheduler`, `gardener-resource-manager` and `gardenlet` also allow enabling profiling handlers via their respective component configs (currently disabled by default).
Here is an example for the `gardener-admission-controller`'s configuration and how to enable it (it looks similar for the other components):

```yaml
apiVersion: admissioncontroller.config.gardener.cloud/v1alpha1
kind: AdmissionControllerConfiguration
# ...
server:
  metrics:
    port: 2723
debugging:
  enableProfiling: true
  enableContentionProfiling: true
```

However, the handlers are served on the same port as configured in `server.metrics.port` via HTTP.

For example (gardener-admission-controller):

```bash
$ curl http://localhost:2723/debug/pprof/heap > /tmp/heap
$ go tool pprof /tmp/heap
```
