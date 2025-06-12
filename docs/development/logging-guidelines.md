# Logging Guidelines in Gardener Components

This document aims at providing a general developer guideline on different aspects of logging practices and conventions used in the Gardener codebase.
It contains mostly Gardener-specific points, and references other existing and commonly accepted logging guidelines for general advice.
Developers and reviewers should consult this guide when writing, refactoring, and reviewing Gardener code.
If parts are unclear or new learnings arise, this guide should be adapted accordingly.

## Logging Libraries / Implementations

Historically, Gardener components have been using [logrus](https://github.com/sirupsen/logrus).
There is a global logrus logger ([`logger.Logger`](https://github.com/gardener/gardener/blob/626ba7c10e1150819b3905116d3988512c18c9ee/pkg/logger/logrus.go#L28)) that is initialized by components on startup and used across the codebase.
In most places, it is used as a `printf`-style logger and only in some instances we make use of logrus' structured logging functionality.

In the process of migrating our components to native controller-runtime components (see [gardener/gardener#4251](https://github.com/gardener/gardener/issues/4251)), we also want to make use of controller-runtime's built-in mechanisms for streamlined logging.
controller-runtime uses [logr](https://github.com/go-logr/logr), a simple structured logging interface, for library-internal logging and logging in controllers.

logr itself is only an interface and doesn't provide an implementation out of the box.
Instead, it needs to be backed by a logging implementation like [zapr](https://github.com/go-logr/zapr). Code that uses the logr interface is thereby not tied to a specific logging implementation and makes the implementation easily exchangeable.
controller-runtime already provides a [set of helpers](https://github.com/kubernetes-sigs/controller-runtime/tree/v0.11.0/pkg/log/zap) for constructing zapr loggers, i.e., logr loggers backed by [zap](https://github.com/uber-go/zap), which is a popular logging library in the go community.
Hence, we are migrating our component logging from logrus to logr (backed by zap) as part of [gardener/gardener#4251](https://github.com/gardener/gardener/issues/4251).

> ⚠️ `logger.Logger` (logrus logger) is deprecated in Gardener and shall not be used in new code – use logr loggers when writing new code! (also see [Migration from logrus to logr](#migration-from-logrus-to-logr))
>
> ℹ️ Don't use zap loggers directly, always use the logr interface in order to avoid tight coupling to a specific logging implementation.

gardener-apiserver differs from the other components as it is based on the [apiserver library](https://github.com/kubernetes/apiserver) and therefore uses [klog](https://github.com/kubernetes/klog) – just like kube-apiserver.
As gardener-apiserver writes (almost) no logs in our coding (outside the apiserver library), there is currently no plan for switching the logging implementation.
Hence, the following sections focus on logging in the controller and admission components only.

## `logcheck` Tool

To ensure a smooth migration to logr and make logging in Gardener components more consistent, the [`logcheck` tool](../../hack/tools/logcheck) was added.
It enforces (parts of) this guideline and detects programmer-level errors early on in order to prevent bugs.
Please check out the [tool's documentation](../../hack/tools/logcheck) for a detailed description. 

## Structured Logging

Similar to [efforts in the Kubernetes project](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/migration-to-structured-logging.md), we want to migrate our component logs to structured logging.
As motivated above, we will use the logr interface instead of klog though.

You can read more about the motivation behind structured logging in [logr's background and FAQ](https://github.com/go-logr/logr#background) (also see [this blog post by Dave Cheney](http://dave.cheney.net/2015/11/05/lets-talk-about-logging)).
Also, make sure to check out controller-runtime's [logging guideline](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.0/TMP-LOGGING.md) with specifics for projects using the library.
The following sections will focus on the most important takeaways from those guidelines and give general instructions on how to apply them to Gardener and its controller-runtime components.

> Note: Some parts in this guideline differ slightly from controller-runtime's document.

### TL;DR of Structured Logging

❌ Stop using `printf`-style logging:
```go
var logger *logrus.Logger
logger.Infof("Scaling deployment %s/%s to %d replicas", deployment.Namespace, deployment.Name, replicaCount)
```

✅ Instead, write static log messages and enrich them with additional structured information in form of key-value pairs:

```go
var logger logr.Logger
logger.Info("Scaling deployment", "deployment", client.ObjectKeyFromObject(deployment), "replicas", replicaCount)
```

## Log Configuration

Gardener components can be configured to either log in `json` (default) or `text` format:
`json` format is supposed to be used in production, while `text` format might be nicer for development.

```text
# json
{"level":"info","ts":"2021-12-16T08:32:21.059+0100","msg":"Hello botanist","garden":"eden"}

# text
2021-12-16T08:32:21.059+0100    INFO    Hello botanist  {"garden": "eden"}
```

Components can be set to one of the following log levels (with increasing verbosity): `error`, `info` (default), `debug`.


## Log Levels

logr uses [V-levels](https://github.com/go-logr/logr#why-v-levels) (numbered log levels), higher V-level means higher verbosity.
V-levels are relative (in contrast to `klog`'s absolute V-levels), i.e., `V(1)` creates a logger, that is one level more verbose than its parent logger.

In Gardener components, the mentioned log levels in the component config (`error`, `info`, `debug`) map to the zap levels with the same names (see [here](https://github.com/gardener/gardener/blob/770fc01a34b70f6cb77b8cfe929d9daef0026d1c/pkg/logger/zap.go#L43-L55)).
Hence, our loggers follow the same mapping from numerical logr levels to named zap levels like described in [zapr](https://github.com/go-logr/zapr/tree/v1.1.0#increasing-verbosity), i.e.:

- component config specifies `debug` ➡️ both `V(0)` and `V(1)` are enabled
- component config specifies `info` ➡️ `V(0)` is enabled, `V(1)` will not be shown
- component config specifies `error` ➡️ neither `V(0)` nor `V(1)` will be shown
- `Error()` logs will always be shown

This mapping applies to the components' root loggers (the ones that are not "derived" from any other logger; constructed on component startup).
If you derive a new logger with e.g. `V(1)`, the mapping will shift by one. For example, `V(0)` will then log at zap's `debug` level.

There is no `warning` level (see [Dave Cheney's post](https://dave.cheney.net/2015/11/05/lets-talk-about-logging)).
If there is an error condition (e.g., unexpected error received from a called function), the error should either be handled or logged at `error` if it is neither handled nor returned.
If you have an `error` value at hand that doesn't represent an actual error condition, but you still want to log it as an informational message, log it at `info` level with key `err`.

We might consider to make use of a broader range of log levels in the future when introducing more logs and common command line flags for our components (comparable to `--v` of Kubernetes components).
For now, we stick to the mentioned two log levels like controller-runtime: info (`V(0)`) and debug (`V(1)`).

## Logging in Controllers

### Named Loggers

Controllers should use named loggers that include their name, e.g.:
```go
controllerLogger := rootLogger.WithName("controller").WithName("shoot")
controllerLogger.Info("Deploying kube-apiserver")
```
results in
```text
2021-12-16T09:27:56.550+0100    INFO    controller.shoot    Deploying kube-apiserver
```
Logger names are hierarchical. You can make use of it, where controllers are composed of multiple "subcontrollers", e.g., `controller.shoot.hibernation` or `controller.shoot.maintenance`.

Using the global logger `logf.Log` directly is discouraged and should be rather exceptional because it makes correlating logs with code harder.
Preferably, all parts of the code should use some named logger.

### Reconciler Loggers

In your `Reconcile` function, retrieve a logger from the given `context.Context`.
It inherits from the controller's logger (i.e., is already named) and is preconfigured with `name` and `namespace` values for the reconciliation request:

```go
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
  log := logf.FromContext(ctx)
  log.Info("Reconciling Shoot")
  // ...
  return reconcile.Result{}, nil
}
```
results in
```text
2021-12-16T09:35:59.099+0100    INFO    controller.shoot    Reconciling Shoot        {"name": "sunflower", "namespace": "garden-greenhouse"}
```

The logger is injected by controller-runtime's `Controller` implementation. The logger returned by `logf.FromContext` is never `nil`. If the context doesn't carry a logger, it falls back to the global logger (`logf.Log`), which might discard logs if not configured, but is also never `nil`.

> ⚠️ Make sure that you don't overwrite the `name` or `namespace` value keys for such loggers, otherwise you will lose information about the reconciled object.

The controller implementation (controller-runtime) itself takes care of logging the error returned by reconcilers.
Hence, don't log an error that you are returning.
Generally, functions should not return an error, if they already logged it, because that means the error is already handled and not an error anymore.
See [Dave Cheney's post](https://dave.cheney.net/2015/11/05/lets-talk-about-logging) for more on this.

### Messages

- Log messages should be static. Don't put variable content in there, i.e., no `fmt.Sprintf` or string concatenation (`+`). Use key-value pairs instead.
- Log messages should be capitalized. Note: This contrasts with error messages, that should not be capitalized. However, both should not end with a punctuation mark.

### Keys and Values

- Use `WithValues` instead of repeatedly adding key-value pairs for multiple log statements. `WithValues` creates a new logger from the parent, that carries the given key-value pairs. E.g., use it when acting on one object in multiple steps and logging something for each step:

  ```go
  log := parentLog.WithValues("infrastructure", client.ObjectKeyFromObject(infrastructure))
  // ...
  log.Info("Creating Infrastructure")
  // ...
  log.Info("Waiting for Infrastructure to be reconciled")
  // ...
  ```

> Note: `WithValues` bypasses controller-runtime's special zap encoder that nicely encodes `ObjectKey`/`NamespacedName` and `runtime.Object` values, see [kubernetes-sigs/controller-runtime#1290](https://github.com/kubernetes-sigs/controller-runtime/issues/1290).
> Thus, the end result might look different depending on the value and its `Stringer` implementation.

- Use [lowerCamelCase](https://en.wiktionary.org/wiki/lowerCamelCase) for keys. Don't put spaces in keys, as it will make log processing with simple tools like `jq` harder.
- Keys should be constant, human-readable, consistent across the codebase and naturally match parts of the log message, see [logr guideline](https://github.com/go-logr/logr#how-do-i-choose-my-keys).

- When logging object keys (name and namespace), use the object's type as the log key and a `client.ObjectKey`/`types.NamespacedName` value as value, e.g.:

  ```go
  var deployment *appsv1.Deployment
  log.Info("Creating Deployment", "deployment", client.ObjectKeyFromObject(deployment))
  ```
  
  which results in

  ```text
  {"level":"info","ts":"2021-12-16T08:32:21.059+0100","msg":"Creating Deployment","deployment":{"name": "bar", "namespace": "foo"}}
  ```

- There are cases where you don't have the full object key or the object itself at hand, e.g., if an object references another object (in the same namespace) by name (think `secretRef` or similar). 
  In such a cases, either construct the full object key including the implied namespace or log the object name under a key ending in `Name`, e.g.:
  
  ```go
  var (
    // object to reconcile
    shoot *gardencorev1beta1.Shoot
    // retrieved via logf.FromContext, preconfigured by controller with namespace and name of reconciliation request
    log logr.Logger
  )
  
  // option a: full object key, manually constructed
  log.Info("Shoot uses SecretBinding", "secretBinding", client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.SecretBindingName})
  // option b: only name under respective *Name log key
  log.Info("Shoot uses SecretBinding", "secretBindingName", *shoot.Spec.SecretBindingName)
  ```
  
  Both options result in well-structured logs, that are easy to interpret and process:

  ```text
  {"level":"info","ts":"2022-01-18T18:00:56.672+0100","msg":"Shoot uses SecretBinding","name":"my-shoot","namespace":"garden-project","secretBinding":{"namespace":"garden-project","name":"aws"}}
  {"level":"info","ts":"2022-01-18T18:00:56.673+0100","msg":"Shoot uses SecretBinding","name":"my-shoot","namespace":"garden-project","secretBindingName":"aws"}
  ```

- When handling generic `client.Object` values (e.g. in helper funcs), use `object` as key.
- When adding timestamps to key-value pairs, use `time.Time` values. By this, they will be encoded in the same format as the log entry's timestamp.  
  Don't use `metav1.Time` values, as they will be encoded in a different format by their `Stringer` implementation. Pass `<someTimestamp>.Time` to loggers in case you have a `metav1.Time` value at hand.
- Same applies to durations. Use `time.Duration` values instead of `*metav1.Duration`. Durations can be handled specially by zap just like timestamps.
- Event recorders not only create `Event` objects but also log them.
  However, both Gardener's manually instantiated event recorders and the ones that controller-runtime provides log to `debug` level and use generic formats, that are not very easy to interpret or process (no structured logs).
  Hence, don't use event recorders as replacements for well-structured logs.
  If a controller records an event for a completed action or important information, it should probably log it as well, e.g.:
  
  ```go
  log.Info("Creating ManagedSeed", "replica", r.GetObjectKey())
  a.recorder.Eventf(managedSeedSet, corev1.EventTypeNormal, EventCreatingManagedSeed, "Creating ManagedSeed %s", r.GetFullName())
  ```

## Logging in Test Code

- If the tested production code requires a logger, you can pass `logr.Discard()` or `logf.NullLogger{}` in your test, which simply discards all logs.
- `logf.Log` is safe to use in tests and will not cause a nil pointer deref, even if it's not initialized via `logf.SetLogger`.
  It is initially set to a `NullLogger` by default, which means all logs are discarded, unless `logf.SetLogger` is called in the first 30 seconds of execution.
- Pass `zap.WriteTo(GinkgoWriter)` in tests where you want to see the logs on test failure but not on success, for example:

  ```go
  logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
  log := logf.Log.WithName("test")
  ```
