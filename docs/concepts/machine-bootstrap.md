# Machine-Controller integration into Gardener
The Gardener deploys the [machine-controller-manager](https://github.com/gardener/machine-controller-manager) as part of a Shoot controlplane onto a Seed to manage the nodes/machines of the Shoot cluster. The machine-controller-manager will create, bootstrap, update and delete the nodes of the Shoot cluster and the underlying virtual machines.

## Flow of creating and bootstrapping Machines/Nodes
The following steps will be ensured by the gardener-controller-manager during a creation/reconciliation flow:
- The machine-controller-manager get deployed into the Shoot namespace of the Seed.
- The proper ``<cloudprovider>MachineClass`` objects and their corresponding secrets get deployed in the Shoot namespace of the Seed. Those secrets contain information how to bootstrap the nodes and will be used by the machine-controller-manager to configure cloud-config/user-data of the VM. Furthermore it contain the credentials of the cloud provider account which will be used to create the VMs.
- The ``MachineDeployment``s for the worker pools specified in the Shoot manifest will be applied. Those deployments will result in the required ``Machine`` objects in the Shoot namespace of the Seed.
- A secret, which contains a valid bootstrap token and the cloud-config for bootstrapping nodes, will be created in the kube-system namespace of the Shoot. Bootstrapping VMs is done via OS-specific configuration (e.g., [CoreOS cloud-config]((https://coreos.com/os/docs/latest/cloud-config.html))).

The machine-controller-manager will pick up the ``Machine`` objects and create the VMs on the respective cloud provider, with the credentials stored in the ``<cloudprovider>MachineClass`` secret.

The secret containing the cloud-config and the bootstrap token will be downloaded from the Shoot API server. This is achieved by copying the contents of the machineclass secret onto the machine. This secret comprises of the configuration to register a systemd service ``cloud-config-downloader`` and a kubeconfig to fetch the secret, which include the cloud-config and the bootstrap-token, from the Shoot API server. The downloaded cloud-config configuration does contain the actual cloud-config, which include all the services and configurations for the VM to register as Kubernetes node.

The kubelet requires a suitable kubeconfig to join the cluster and it will generate it by relying on the [tls-bootstraping](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/) functionality of the kubelet.

#### Details
The ``cloud-config-downloader`` fetches the bootstrap-token from Shoot API server and checks if already a kubeconfig file exists on the path which the kubelet configures via its ``--kubeconfig`` flag. If the file exists the downloader does nothing because it assumes that a proper kubeconfig already exists. If the file does not exists, then a bootstrap-kubeconfig will be created with the bootstrap token to authenticate. This bootstrap-kubeconfig will be placed on the path configured via kubelets [``--bootstrap-kubeconfig``](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/) flag. Afterwards, the kubelet will perform the tls-bootstrapping process to generate a suitable kubeconfig, which will be stored on the path the kubelets ``--kubeconfig`` flag is pointing to.

The ``cloud-config-downloader`` checks periodically for changes in the secret which contains the cloud-config and restarts the systemd services on the node in case of a change. That's how Kubernetes patch updates work for the kubelet.
