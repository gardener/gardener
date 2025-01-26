# Gardener Info `ConfigMap`

## Overview

The gardener apiserver maintains a [ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/) inside the Garden cluster that contains information about the gardener apiserver itself.
The ConfigMap is named `gardener-info` and located in the `gardener-system-info` namespace and is visible to all authenticated users.

## Fields

The following fields are provided:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gardener-info
  namespace: gardener-system-info
data:
  gardenerAPIServer: |                                                      # key name of the gardener-apiserver section
    featureGates: ShootForceDeletion=true,UseNamespacedCloudProfile=true    # list of the configured feature gates
    version: v1.111.0                                                       # version of the gardener-apiserver
    workloadIdentityIssuerURL: https://issuer.gardener.cloud.local          # the URL of the authority that issues workload identity tokens
```
