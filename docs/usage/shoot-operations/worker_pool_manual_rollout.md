# Manual Worker Pool Rollout

There may be cases when an end-user might want to trigger a manual worker pool rollout.
For example, the [dual-stack migration](../networking/dual-stack-networking-migration.md) requires to roll nodes.
This can be accomplished by annotating the `Shoot` with the `rollout-workers` operation annotation and specifying which worker pools you'd like to be rolled out.

```bash
kubectl -n <shoot-namespace> annotate shoot <shoot-name> 'gardener.cloud/operation=rollout-workers=<pool1-name>[,<pool2-name>,...]'
```

Alternatively, you can use `*` to roll out all worker pools:

```bash
kubectl -n <shoot-namespace> annotate shoot <shoot-name> 'gardener.cloud/operation=rollout-workers=*'
```

This will cause the status field `manualWorkerPoolRollout` to be set on the `Shoot`. 
It will keep track of the worker pools that are currently being rolled out.

Example status field:
```yaml
    manualWorkerPoolRollout:
      pendingWorkersRollouts:
      - lastInitiationTime: "2025-10-17T14:38:13Z"
        name: local
```
