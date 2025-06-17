# Manual Worker Pool Rollout

There may be cases when an end user might want to trigger a manual worker pool rollout. For example, the dual-stack migration requires to roll nodes.
This can be accomplished by annotating the shoot with the `rollout-workers` annotation and specifying which worker pools you'd like to be rolled out.

```bash
kubectl -n <shoot-namespace> annotate shoot <shoot-name> gardener.cloud/operation=rollout-workers=<pool1-name>[,<pool2-name>,...]
```

Alternatively, you can use `*` to roll out all worker pools:

```bash
kubectl -n <shoot-namespace> annotate shoot <shoot-name> gardener.cloud/operation=rollout-workers=*
```

This will cause a new status called `manualWorkerPoolRollout` to be set on the shoot. It will keep track of the worker pools that are currently being rolled out
along with information about the last manual rollout that has been triggered.
