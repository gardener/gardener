


#### Cluster Certificate Authority Bundle

*Name*: `<shoot-name>.ca-cluster`

*Description*: Certificate Authority (CA) bundle of the Cluster (`Secret` key: `ca.crt`).

This bundle contains one or multiple CAs which are used for signing serving certificates of the Shoot's API server. Hence, the certificates contained in this `Secret` can be used to verify the API server's identity when communicating with its public endpoint (e.g. as `certificate-authority-data` in a Kubeconfig).
This is the same certificate that is also contained in the Kubeconfig's `certificate-authority-data` field.

*Rotation*: Not supported yet, but work is in progress. See [gardener/gardener#3292](https://github.com/gardener/gardener/issues/3292) and [GEP-18](https://github.com/gardener/gardener/blob/release-v1.42/docs/proposals/18-shoot-CA-rotation.md) for more details.
