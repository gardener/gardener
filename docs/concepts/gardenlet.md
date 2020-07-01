# Gardenlet

Right from the beginning of the Gardener Project we started implementing the [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/):
We have a custom controller-manager that acts on our own custom resources.
Now, when you start thinking about the [Gardener architecture](https://github.com/gardener/documentation/wiki/Architecture), you will recognize some interesting similarity with respect to the Kubernetes architecture:
Shoot clusters can be compared with pods, and seed clusters can be seen as worker nodes.
Guided by this observation we introduced the **gardener-scheduler** ([#356](https://github.com/gardener/gardener/issues/356)).
Its main task is to find an appropriate seed cluster to host the control-plane for newly ordered clusters, similar to how the kube-scheduler finds an appropriate node for newly created pods.
By providing multiple seed clusters for a region (or provider) and distributing the workload, we reduce the blast-radius of potential hick-ups as well.

Yet, there was still a significant difference between the Kubernetes and the Gardener architectures:
Kubernetes runs a primary "agent" on every node, the kubelet, which is mainly responsible for managing pods and containers on its particular node.
Gardener used its single controller-manager which was responsible for all shoot clusters on all seed clusters, and it was performing its reconciliation loops centrally from the garden cluster.

While this works well at scale for thousands of clusters today, our goal is to enable true scalability following the Kubernetes principles (beyond the capacity of a single controller-manager):
We have now worked on distributing the logic (or the Gardener operator) into the seed cluster and introduced a corresponding component, adequately named the **Gardenlet**.
It is Gardener's primary "agent" on every seed cluster and is only responsible for shoot clusters located in its particular seed cluster.

![](gardenlet-architecture-similarities.png)

The gardener-controller-manager still kept its control loops for other resources of the Gardener API, however, it does no longer talk to seed/shoot clusters.

Reversing the control flow will even allow placing seed/shoot clusters behind firewalls without the necessity of direct accessibility (via VPN tunnels) anymore.

![](gardenlet-architecture-detailed.png)

## TLS Bootstrapping

Kubernetes does not manage worker nodes itself, and it is also not responsible for the lifecycle of the kubelet running on the workers.
Similarly, Gardener does not manage seed clusters itself (although, it is possible that you create a shoot cluster that you later register a seed again, of course), hence, Gardener is also not responsible for the lifecycle of the Gardenlet running on the seeds.

As explained in the above motivation, the Gardenlet can be compared with the kubelet.
After you depoyed it yourself into your seed clusters, it initializes a bootstrapping process that is very similar to the [Kubelet's TLS bootstrapping](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-tls-bootstrapping/):

* Gardenlet starts up with a bootstrap kubeconfig having a bootstrap token that allows to create `CertificateSigningRequest` resources
* After the CSR is signed it downloads the created client certificate, creates a new kubeconfig with it, and stores it inside a `Secret` in the seed cluster
* It deletes the bootstrap kubeconfig secret and starts up with its new kubeconfig
* Gardenlet starts normal operation

Basically, you can follow the Kubernetes documentation regarding the process.
The gardener-controller-manager runs a control loop that automatically signs CSRs created by Gardenlets.

Optionally, if you don't want to run this bootstrap process, then you can create a kubeconfig pointing to the Garden cluster for the Gardenlet yourself, and simply provide it to it (field `gardenClientConnection.kubeconfig` in the Gardenlet configuration).

### Gardenlet certificate rotation

The Gardenlet tries to automatically rotate the certificate when it approaches expiration of its one year lifetime (at ~80% already passed).
In order to use certificate rotation, the Gardenlet [component configuration](#component-configuration) needs to set the field `.gardenClientConnection.kubeconfigSecret`, specifying the secret to store the kubeconfig with the rotated certificate.

The same control loop in the gardener-controller-manager that signs the CSRs during the initial TLS Bootstrapping, also automatically signs the CSR during a certificate rotation.
This works when the Gardenlet created the certificate during the initial TLS Bootstrapping using the Bootstrap kubeconfig. 

However, when trying to rotate a Kubeconfig containing a custom certificate (not created by Gardenlet TLS Bootstrap), the x509 certificate's `Subject` field needs to conform to the following:
  - the Common Name (CN) is prefixed with `gardener.cloud:system:seed:`
  - the Organisation (O) equals `gardener.cloud:system:seeds`

Otherwise, the gardener-controller-manager will not automatically sign the CSR.
In this case, an external component/user needs to approve the CSR manually (e.g. via `kubectl certificate approve  seed-csr-<...>`).
If that does not happen within 15 minutes, the Gardenlet repeats the process and creates another CSR.

## Seed Config vs. Seed Selector

While the kubelet has to run on the worker node directly in order to talk to the container runtime and to manage the lifecycle of containers, the Gardenlet can potentially run outside of the seed cluster as long as it can talk to the seed's API server.
Also, it's possible that the Gardenlet controls more than one seed.
Theoretically, if network connectivity is available, a Gardenlet installed inside the Garden cluster could be responsible all seed clusters in the system, just like the gardener-controller-manager back in previous Gardener versions.
However, this scenario has been mainly implemented for development purposes.

For production use, though, mainly motivated with scalability arguments and a better distribution of responsibilities, it is recommend to run one Gardenlet per seed inside the seed cluster itself.

If you want the Gardenlet in the standard way then please provide a `seedConfig` that contains information about the seed cluster itself, see [the example Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml#L69-L102).
Once bootstrapped, the Gardenlet will create and update its `Seed` object itself.

If you want the Gardenlet to manage multiple seeds then please provide a `seedSelector` that incorporates a label selector for the targeted `Seed`s, see [the example Gardenlet configuration](../../example/20-componentconfig-gardenlet.yaml#L68).
In this case, you have to create the `Seed` objects (together with a kubeconfig pointing to the cluster) yourself (like with previous Gardener versions).

## Component Configuration

You can find an example configuration file [here](../../example/20-componentconfig-gardenlet.yaml).

Basically, it is possible to define settings for the Kubernetes clients interacting with the various clusters, settings for the control loops inside the Gardenlet, settings for leader election and log levels, feature gates, and seed selection/configuration.

Most of the configuration options are similar to what the gardener-controller-manager offered with previous Gardener versions.

## Heartbeats

Similar to how Kubernetes is meanwhile using `Lease` objects for node heart beats (see [KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/0009-node-heartbeat.md)), the Gardenlet is using `Lease`s objects for seed heart beats.
Every two seconds it is checking its connectivity to the seed and then it renews its lease.
The status will be reported in the `GardenletReady` condition in its `Seed` object(s).
Similarly to the `node-lifecycle-controller` inside the kube-controller-manager, the gardener-controller-manager features a `seed-lifecycle-controller` that will set the `GardenletReady` condition to `Unknown` in case the Gardenlet stops sending its heartbeat signals.
Carrying on, this will make the gardener-scheduler not considering this seed for newly created shoots anymore.

### `/healthz` Endpoint

[gardener/gardener#2309](https://github.com/gardener/gardener/pull/2309) has enhanced the Gardenlet with a HTTPS server that serves a `/healthz` endpoint.
It is used as a liveness probe in the `Deployment` of the Gardenlet.
If the Gardenlet fails trying to renew its lease then the endpoint will return `500 Internal Server Error`, otherwise it will return `200 OK`.

⚠️ In case the Gardenlet is managing mutliple seeds (i.e., a seed selector is used) then the `/healthz` will report `500 Internal Server Error` if there is at least one seed for which it could not renew its lease.
Only if it can renew the lease for all seeds then it will report `200 OK`.

## Shooted Seeds

If the Gardenlet manages a shoot cluster that has been marked to be used as seed then it will automatically deploy itself into the cluster, unless you prevent this by using the `no-gardenlet` configuration in the `shoot.gardener.cloud/use-as-seed` annotation (in this case, you have to deploy the Gardenlet on your own into the seed cluster).

*Example: Annotate the shoot with `shoot.gardener.cloud/use-as-seed="true,no-gardenlet,invisible"` to mark it as invisible (meaning, that the gardener-scheduler won't consider it) and to express your desire to deploy the gardenlet into the cluster on your own.*

For automatic deployments, the Gardenlet will use the same version and the same configuration for the clone it deploys.

## Migrating from previous Gardener versions

You have to make sure that your Garden cluster is exposed in a way that it is reachable from all your seed clusters.
Otherwise, you cannot upgrade Gardener to v1.
Also, you should at least Gardener 0.31 before upgrading to v1.

Apart from these constraints, there is no special migration task required.
With previous Gardener versions, you had deployed the Gardener Helm chart (incorporating the API server, controller-manager, and scheduler).
With v1, this will stay the same, but you now have to deploy the Gardenlet Helm chart as well (into all of your seed (if they are not shooted, see above)).

Please follow the [general deployment guide](../deployment/kubernetes.md) for all instructions.
