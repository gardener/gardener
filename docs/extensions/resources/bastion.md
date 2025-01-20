# Contract: `Bastion` Resource

The Gardener project allows users to connect to Shoot worker nodes via SSH. As nodes are usually firewalled and not directly accessible from the public internet, [GEP-15](../../proposals/15-manage-bastions-and-ssh-key-pair-rotation.md) introduced the concept of "Bastions". A bastion is a dedicated server that only serves to allow SSH ingress to the worker nodes.

`Bastion` resources contain the user's public SSH key and IP address, in order to provision the server accordingly: The public key is put onto the Bastion and SSH ingress is only authorized for the given IP address (in fact, it's not a single IP address, but a set of IP ranges, however for most purposes a single IP is be used).

## What Is the Lifespan of a `Bastion`?

Once a `Bastion` has been created in the garden, it will be replicated to the appropriate seed cluster, where a controller then reconciles a server and firewall rules etc., on the cloud provider used by the target Shoot. When the Bastion is ready (i.e. has a public IP), that IP is stored in the `Bastion`'s status and from there it is picked up by the garden cluster and `gardenctl` eventually.

To make multiple SSH sessions possible, the existence of the `Bastion` is not directly tied to the execution of `gardenctl`: users can exit out of `gardenctl` and use `ssh` manually to connect to the bastion and worker nodes.

However, `Bastion`s have an expiry date, after which they will be garbage collected.

When SSH access is set to `false` for the `Shoot` in the workers settings (see [Shoot Worker Nodes Settings](../../usage/shoot/shoot_workers_settings.md)), `Bastion` resources are deleted during `Shoot` reconciliation and new `Bastion`s are prevented from being created.

## What Needs to Be Implemented to Support a New Infrastructure Provider?

As part of the shoot flow, Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: Bastion
metadata:
  name: mybastion
  namespace: shoot--foo--bar
spec:
  type: aws
  # userData is base64-encoded cloud provider user data; this contains the
  # user's SSH key
  userData: IyEvYmluL2Jhc2ggL....Nlcgo=
  ingress:
    - ipBlock:
        cidr: 192.88.99.0/32 # this is most likely the user's IP address
```

Your controller is supposed to create a new instance at the given cloud provider, firewall it to only allow SSH (TCP port 22) from the given IP blocks, and then configure the firewall for the worker nodes to allow SSH from the bastion instance. When a `Bastion` is deleted, all these changes need to be reverted.

## Implementation Details

### `ConfigValidator` Interface

For bastion controllers, the generic `Reconciler` also delegates to a [`ConfigValidator` interface](../../../extensions/pkg/controller/bastion/configvalidator.go) that contains a single `Validate` method. This method is called by the generic `Reconciler` at the beginning of every reconciliation, and can be implemented by the extension to validate the `.spec.providerConfig` part of the `Bastion` resource with the respective cloud provider, typically the existence and validity of cloud provider resources such as VPCs, images, etc.

The `Validate` method returns a list of errors. If this list is non-empty, the generic `Reconciler` will fail with an error. This error will have the error code `ERR_CONFIGURATION_PROBLEM`, unless there is at least one error in the list that has its `ErrorType` field set to `field.ErrorTypeInternal`.

## References and Additional Resources

* [`Bastion` API Reference](../../api-reference/extensions.md#bastion)
* [Exemplary Implementation for the AWS Provider](https://github.com/gardener/gardener-extension-provider-aws/tree/master/pkg/controller/bastion)
* [GEP-15](../../proposals/15-manage-bastions-and-ssh-key-pair-rotation.md)
