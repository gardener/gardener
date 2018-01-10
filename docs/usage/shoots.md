# Creating / Deleting a Shoot cluster

As you have already prepared an example Shoot manifest in the steps described in the development documentation, please open another Terminal pane/window with the `KUBECONFIG` environment variable pointing to the Garden development cluster and send the manifest to the Kubernetes API server:

```bash
$ kubectl apply -f $GOPATH/src/github.com/gardener/gardener/dev/shoot-aws.yaml
```

You should see that the Gardener has immediately picked up your manifest and started to deploy the Shoot cluster.

In order to investigate what is happening in the Seed cluster, please download its proper Kubeconfig yourself (see next paragraph). The namespace of the Shoot cluster in the Seed cluster will look like that: `shoot-johndoe-johndoe-1`, whereas the first `johndoe` is your namespace in the Garden cluster (also called "project") and the `johndoe-1` suffix is the actual name of the Shoot cluster.

To connect to the newly created Shoot cluster, you must download its Kubeconfig as well. Please connect to the proper Seed cluster, navigate to the Shoot namespace, and download the Kubeconfig from the `kubecfg` secret in that namespace.

In order to delete your cluster, you have to set an annotation confirming the deletion first, and trigger the deletion after that. You can use the prepared `delete-shoot` script which takes the Shoot name as first parameter. The namespace can be specified by the second parameter, but it is optional. If you don't state it, it defaults to your namespace (the username you are logged in with to your machine).

```bash
$ ./hack/delete-shoot johndoe-1
```
