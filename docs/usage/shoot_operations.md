# Trigger shoot operations

You can trigger a few explicit operations by annotating the `Shoot` with an operation annotation.
This might allow you to induct certain behavior without the need to change the `Shoot` specification.
Some of the operations can also not be caused by changing something in the shoot specification because they can't properly be reflected here.

## Immediate reconciliation

Annotate the shoot with `shoot.garden.sapcloud.io/operation=reconcile` to make the `gardener-controller-manager` start a reconciliation operation without changing the shoot spec and possibly without being in its maintenance time window:

```bash
$ kubectl -n garden-<project-name> annotate shoot <shoot-name> shoot.garden.sapcloud.io/operation=reconcile
```

## Immediate maintenance

Annotate the shoot with `shoot.garden.sapcloud.io/operation=maintain` to make the `gardener-controller-manager` start maintaining your shoot immediately (possibly without being in its maintenance time window).
If no reconciliation starts then nothing needed to be maintained:

```bash
$ kubectl -n garden-<project-name> annotate shoot <shoot-name> shoot.garden.sapcloud.io/operation=maintain
```

## Retry failed operation

Annotate the shoot with `shoot.garden.sapcloud.io/operation=retry` to make the `gardener-controller-manager` start a new reconciliation loop on a failed shoot.
Failed shoots are only reconciled again if a new Gardener version is deployed, the shoot specification is changed or this annotation is set

```bash
$ kubectl -n garden-<project-name> annotate shoot <shoot-name> shoot.garden.sapcloud.io/operation=retry
```

## Rotate kubeconfig credentials

Annotate the shoot with `shoot.garden.sapcloud.io/operation=rotate-kubeconfig-credentials` to make the `gardener-controller-manager` exchange the credentials in your shoot cluster's kubeconfig.
Please note that only the token (and basic auth password, if enabled) are exchanged. The cluster CAs remain the same.

```bash
$ kubectl -n garden-<project-name> annotate shoot <shoot-name> shoot.garden.sapcloud.io/operation=rotate-kubeconfig-credentials
```
