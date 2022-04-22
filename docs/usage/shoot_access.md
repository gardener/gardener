# Shoot_access.md

There are several scenarios where end-user would like to access shoot cluster. There are several options available to user to access the shoot cluster.

## Static token kubeconfig :

  - To access the shoot cluster with the static token kubeconfig, shoot should have `.spec.kubernetes.enableStaticTokenKubeconfig` option set to `true`. enableStaticTokenKubeconfig will allow the creation of kubeconfig secret in the project namespace. End users can fetch the secret and use the kubeconfig inside it. Static token belongs to `system:masters` group and grants `cluster-admin` privileges to cluster.

    ```yaml
    apiVersion: core.gardener.cloud/v1beta1
    kind: Shoot
    metadata:
      name: local
      namespace: garden-local
      ...
    spec:
      kubernetes:
        version: 1.23.1
        enableStaticTokenKubeconfig: true
      ...
    ```
  This is not the recommended method to access the shoot cluster as the static token kubeconfig has some security flaws associated with it.

   - Static token in the kubeconfig doesn't have any expiration date. To revoke the static token, the user needs to rotate the kuebcconfig credentials.
   - Static token doesn't have user identity associated with it. User in that token will always be system:cluster-admin irrespective of the person accessing the cluster. Hence it is impossible to audit the events in cluster.

  To disable the creation of static token kubeconfig, set the value of `.spec.kubernetes.enableStaticTokenKubeconfig`  field in the shoot spec to `false`.
  ```yaml
    apiVersion: core.gardener.cloud/v1beta1
    kind: Shoot
    metadata:
      name: local
      namespace: garden-local
      ...
    spec:
      kubernetes:
        version: 1.23.1
        enableStaticTokenKubeconfig: true
      ...
  ```

## Dynamic kubeconfig

  Shoot subresource called `adminKubeconfig` allows user to dynamically generate short lived `kubeconfig` that can be used to access shoot cluster with `cluster-admin` priviledge. `shoots/adminkubeconfig` resource can only accept `CREATE` calls and accept   `AdminKubeconfigRequest` .Kubeconfig is generated only when `AdminKubeconfigRequest` sent to the subresource.

  User identity of kubeconfig will be the user authenticating to gardener API server. Following command can be used to request a kubeconfig using the `AdminKubeconfigRequest`.

  ```bash
  export NAMESPACE=my-namespace
  export SHOOT_NAME=my-shoot
  kubectl create \
      -f example/shoot-kubeconfig/kubeconfig-request.json \
      --raw /apis/core.gardener.cloud/v1beta1/namespaces/${NAMESPACE}/shoots/${SHOOT_NAME}/adminkubeconfig | jq -r ".status.kubeconfig" | base64 -d
  ```

  Here  `kubeconfig-request.json` has the followng content where `expirationSeconds` is in seconds.

  ```json
  {
      "apiVersion": "authentication.gardener.cloud/v1alpha1",
      "kind": "AdminKubeconfigRequest",
      "spec": {
          "expirationSeconds": 1000
      }
  }
  ```

  