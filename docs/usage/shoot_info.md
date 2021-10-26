# Shoot Info

Inside a shoots kubernetes cluster, gardener provides a [ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/)
with informations about the cluster itself.
The map is in the namespace `kube-system` named `shoot-info`.

Example: `kubectl get configmap -n kube-system shoot-info -o yaml`

## Fields

Following fields are provided:

| Field name           | Description                                                                                                        |
| -------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `domain`             | Domain under which this cluster is reachable.                                                                      |
| `extensions`         | List of activated extensions.                                                                                      |
| `kubernetesVersion`  | Current kubernetes version deployed                                                                                |
| `maintenanceBegin`   | Time when maintenance window starts                                                                                |
| `maintenanceEnd`     | Time when maintenance window ends                                                                                  |
| `nodeNetwork`        | CIDR of cluster node network                                                                                       |
| `podNetwork`         | CIDR of pod network                                                                                                |
| `projectName`        | Project name under which this cluster resides.                                                                     |
| `provider`           | Provider extension used for creating this cluster                                                                  |
| `region`             | Region this cluster is placed                                                                                      |
| `serviceNetwork`     | CIDR of service network                                                                                            |
| `shootName`          | Name of the shoot cluster                                                                                          |
