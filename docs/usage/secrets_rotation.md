


#### Cluster Certificate Authority Bundle

*Name*: `<shoot-name>.ca-cluster`

*Description*: Certificate Authority (CA) bundle of the Cluster (`Secret` key: `ca.crt`).

This bundle contains one or multiple CAs which are used for signing serving certificates of the Shoot's API server. Hence, the certificates contained in this `Secret` can be used to verify the API server's identity when communicating with its public endpoint (e.g. as `certificate-authority-data` in a Kubeconfig).
This is the same certificate that is also contained in the Kubeconfig's `certificate-authority-data` field.

*Rotation*: Not supported yet, but work is in progress. See [gardener/gardener#3292](https://github.com/gardener/gardener/issues/3292) and [GEP-18](https://github.com/gardener/gardener/blob/release-v1.42/docs/proposals/18-shoot-CA-rotation.md) for more details.

#### Monitoring

*Name*: `<shoot-name>.monitoring`

*Description*: Username/password for accessing the user Grafana instance of a shoot cluster (`Secret` keys: `username`/`password`).

*Rotation*: Not supported yet.

#### SSH-Keypair

*Name*: `<shoot-name>.ssh-keypair`

*Description*: SSH-Keypair that is propagated to the worker nodes of the shoot cluster.
The private key can be used to establish an SSH connection to the workers for troubleshooting purposes (`Secret` keys: `id_rsa`/`id_rsa.pub`).

*Rotation*: Keypair can be rotated by annotating the shoot resource with `gardener.cloud/operation: rotate-ssh-keypair`.
Propagating the new keypair to all worker nodes may take longer than the initiated reconciliation of the shoot.
The previous keypair can still be found in the `<shoot-name>.ssh-keypair.old` secret and is still valid until the next rotation. 

You can check the `.status.credentials.rotation.sshKeypair` field in the `Shoot` to see when the rotation was last initiated or last completed.
