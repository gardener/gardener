# Reconcile trigger

Gardener dictates the time of reconciliation for resources of the API group `extensions.gardener.cloud`.
It does that by annotating the respected resource with `gardener.cloud/operation=reconcile`.
Extension controllers shall react to this annotation and start reconciling the resource.
They have to remove this annotation as soon as they begin with their reconcile operation and maintain the `status` of the extension resource accordingly.

The reason for this behaviour is that it is possible to configure Gardener to reconcile only in the shoots' maintenance time windows.
In order to avoid that extension controllers reconcile outside of the shoot's maintenance time window we have introduced this contract.
This way extension controllers don't need to care about when the shoot maintenance time window happens.
Gardener keeps control and decides when the shoot shall be reconciled/updated.

Our [extension controller library](https://github.com/gardener/gardener-extensions) provides all the required utilities to conveniently implement this behaviour.
