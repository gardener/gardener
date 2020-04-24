# Trigger shoot operations

You can trigger a few explicit operations by annotating the `Shoot` with an operation annotation.
This might allow you to induct certain behavior without the need to change the `Shoot` specification.
Some of the operations can also not be caused by changing something in the shoot specification because they can't properly be reflected here.

Please note: If `.spec.maintenance.confineSpecUpdateRollout=true` then the only way to trigger a shoot reconciliation is by setting the `reconcile` operation, see below.

## Immediate reconciliation

Annotate the shoot with `gardener.cloud/operation=reconcile` to make the `gardenlet` start a reconciliation operation without changing the shoot spec and possibly without being in its maintenance time window:

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=reconcile
```

## Immediate maintenance

Annotate the shoot with `gardener.cloud/operation=maintain` to make the `gardener-controller-manager` start maintaining your shoot immediately (possibly without being in its maintenance time window).
If no reconciliation starts then nothing needed to be maintained:

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=maintain
```

## Retry failed operation

Annotate the shoot with `gardener.cloud/operation=retry` to make the `gardenlet` start a new reconciliation loop on a failed shoot.
Failed shoots are only reconciled again if a new Gardener version is deployed, the shoot specification is changed or this annotation is set

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=retry
```

## Rotate kubeconfig credentials

Annotate the shoot with `gardener.cloud/operation=rotate-kubeconfig-credentials` to make the `gardenlet` exchange the credentials in your shoot cluster's kubeconfig.
Please note that only the token (and basic auth password, if enabled) are exchanged. The cluster CAs remain the same.

```bash
kubectl -n garden-<project-name> annotate shoot <shoot-name> gardener.cloud/operation=rotate-kubeconfig-credentials
```
