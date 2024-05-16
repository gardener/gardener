# machine-controller-manager-provider-local

Out of tree (controller-based) implementation for `local` as a new provider.
The local out-of-tree provider implements the interface defined at [MCM OOT driver](https://github.com/gardener/machine-controller-manager/blob/master/pkg/util/provider/driver/driver.go).

## Fundamental Design Principles

Following are the basic principles kept in mind while developing the external plugin.

- Communication between this Machine Controller (MC) and Machine Controller Manager (MCM) is achieved using the Kubernetes native declarative approach.
- Machine Controller (MC) behaves as the controller used to interact with the cloud provider AWS and manage the VMs corresponding to the machine objects.
- Machine Controller Manager (MCM) deals with higher level objects such as machine-set and machine-deployment objects.
