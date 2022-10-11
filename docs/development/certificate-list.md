# List of certificates use in Gardener and its location.
|Serial No.|    Certificates        | Cluster         |Cluster (Base/virtual)|Storage location|  Description               |
| ---------|----------------| --------------- |----------------------|----------------|-----------------------------|
|1.        | kubeApiServer            | SHOOT Master  |Both                  |/etc/kubernetes/pki| use for secure communication between kube-apiserver and kubelet |
|2.        | kubelet                  | SHOOT Worker  |Both                  |/etc/kubernetes/pki| used for signing client certificates for talking to kubelet API, e.g. kube-apiserver-kubelet |
|3.        |etcd CA                  | SHOOT master  |Both                  |/etc/kubernetes/pki/etcd| CA (used for signing etcd serving certificates and client certificates |
|4.        | front-proxy CA           | SHOOT master  |Both                  |/etc/kubernetes/pki| used for signing client certificates that kube-aggregator (part of kube-apiserver) uses to talk to extension API servers, filled into extension-apiserver-authentication ConfigMap and read by extension API servers to verify incoming kube-aggregator requests) |
|5.        |metrics-server            | SHOOT master |Both                  |/etc/kubernetes/pki| used for signing serving certificates, filled into APIService caBundle field and read by kube-aggregator to verify the presented serving certificate |
|6.        | ReversedVPN CA            | SHOOT master |Both                  |/etc/kubernetes/pki| used for signing vpn-seed-server serving certificate and vpn-shoot client certificate|
|7.        |vpn-seed-server          | shoot         |Base                   |/etc/envoy         | This is used by the Shoot API Server and the prometheus pods to communicate over VPN/proxy with pods in the Shoot cluster. **Note: only if rversed vpn disable**|
|8.        |shoot-cluste-autoscaler  | shoot         |                      |/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig|To establish a secure communication between HPA/VPA and metrics server |
|9.        |ca-provider-vsphere-controlplane| shoot  |Both                  |                     |Gardener Extension need to communicate with vsphere provider, for Gardener to leverage vSphere clusters for machine provisioning|
|10.        |ca-provider-vsphere-controlplane-bundle|shoot| Both             |                     |                       |
|11.        |Cluster CA               |             |                       |                     |used for signing kube-apiserver serving certificates and client certificates | 
|12.        |gardenLet CA             |garden       |                       |                     |Used for communication between gardnerlet and gardner API server|
|13.        |ca-seed                  |garden       |Both                   |                     |certificate generated for a seed cluster|
|14.        |ca-seed-bundle           |garden       |Both                   |                     |contains ROOT and intermediate certificates| 
|15.        |garden-etcd-events-ca    |Garden       |Base                   |/var/etcd/ssl/client,server| Secure communication between garden API server and etcd, etcd-events store that contains all Event objects (events.k8s.io) of a cluster|
|16.        |garden-etcd-main-ca      |Garden       |Base                   |/var/etcd/ssl/client,server| Secure communication between garden API server and etcd, etcd main store contains all “cluster critical” or “long-term” objects. for backup|
|17.        |garden-kube-aggregator-ca|Garden       |Base                   |/var/etcd/ssl/client,server| Secure communication between garden API server and kube aggregator|
|18.        |garden-kube-apiserver-ca |Garden       |Base                   |/var/etcd/ssl/client,server| Secure communication between gardenlet and kube apiServer |
|19.        |identity-ca              |garden       |Base                   |                       | identity provider using OIDC|
|20.        |Istio-ca                 |istio-system |                       |/var/run/secrets/tokens    | multiple kube-apiservers behind a single LoadBalancer, an intermediate proxy must be placed between the Cloud-Provider’s LoadBalancer and kube-apiservers|
|21.        |cert-manager-webhook     |Cert-manager |Base                   |                           |                     |
|22.        |managedresource-cluster  |garden       |Both                   |                           |                     |
|23.        |networking-calico  |garden       |Base                   |                           |                     |
|24.        |istio-ca-secret          |             |Both                   |                           |Istio secret         |

### Pod processes identity using service account
|Serial No.|    secret                          |       type                           | Cluster(Base/virtual)| Description   |
|----------|--------------------------|--------------------------------------|----------------------|---------------|
|1.        |cert-manager-cainjector-token       |kubernetes.io/service-account-token|Base                  |               |
|2.        | default-token                       |kubernetes.io/service-account-token|Both                  |               |
|3.        | gardener-extension-networking-calico|kubernetes.io/service-account-token|Both                  |               |
|4.        | calico-kube-controllers-token       |kubernetes.io/service-account-token|Both                  |               |                     
|5.        | calico-node-cpva-token              |kubernetes.io/service-account-token|Virtual               |               |                                                                                                  
|6.        | calico-node-token                   |kubernetes.io/service-account-token|Both                  |               |                                                                                                   
|7.        | calico-typha-token                  |kubernetes.io/service-account-token|Virtual               |               |                                                                                                   
|8.        | certificate-controller              |kubernetes.io/service-account-token|Both                  |Kubernetes service accounts are Kubernetes resources, created and managed using the Kubernetes API|
|9.        | cluster-autoscaler-token            |kubernetes.io/service-account-token|Both                  |               |                                                                                                   
|10.        | horizontal-pod-autoscaler-token    |kubernetes.io/service-account-token|Both                  |               |                                                                                                   
|11.        | replicaset-controller-token        |kubernetes.io/service-account-token|Both                  |               |                                                                                                   
|12.        | replication-controller-token       |kubernetes.io/service-account-token|Both                  |               |                                                                                                   
|13.        | root-ca-cert-publisher-token       |kubernetes.io/service-account-token|Both                  |               |                                                                                                   
|14.        | cluster-autoscaler-token           |kubernetes.io/service-account-token|Both                  |               |
### Certificate required to secure a domain
* **Domain CA:** stores in source cluster
* **Ingress CA:** stores in source cluster
* **Service CA:** stores in source cluster
