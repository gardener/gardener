<p>Packages:</p>
<ul>
<li>
<a href="#seedmanagement.gardener.cloud%2fv1alpha1">seedmanagement.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="seedmanagement.gardener.cloud/v1alpha1">seedmanagement.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeed">ManagedSeed</a>
</li></ul>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeed">ManagedSeed
</h3>
<p>
<p>ManagedSeed represents a Shoot that is registered as Seed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
seedmanagement.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ManagedSeed</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">
ManagedSeedSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the ManagedSeed.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>shoot</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Shoot">
Shoot
</a>
</em>
</td>
<td>
<p>Shoot references a Shoot that should be registered as Seed.</p>
</td>
</tr>
<tr>
<td>
<code>seedTemplate</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.SeedTemplate">
SeedTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedTemplate is a template for a Seed object, that should be used to register a given cluster as a Seed.
Either SeedTemplate or Gardenlet must be specified. When Seed is specified, the ManagedSeed controller will not deploy a gardenlet into the cluster
and an existing gardenlet reconciling the new Seed is required.</p>
</td>
</tr>
<tr>
<td>
<code>gardenlet</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">
Gardenlet
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardenlet specifies, that the ManagedSeed controller should deploy a gardenlet into the cluster
with the given deployment parameters and GardenletConfiguration.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedStatus">
ManagedSeedStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the ManagedSeed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.Bootstrap">Bootstrap
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet</a>)
</p>
<p>
<p>Bootstrap describes a mechanism for bootstrapping gardenlet connection to the Garden cluster.</p>
</p>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">ManagedSeedSpec</a>)
</p>
<p>
<p>Gardenlet specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletDeployment">
GardenletDeployment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
the image, etc.</p>
</td>
</tr>
<tr>
<td>
<code>config</code></br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config is the GardenletConfiguration used to configure gardenlet.</p>
</td>
</tr>
<tr>
<td>
<code>bootstrap</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Bootstrap">
Bootstrap
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bootstrap is the mechanism that should be used for bootstrapping gardenlet connection to the Garden cluster. One of ServiceAccount, BootstrapToken, None.
If set to ServiceAccount or BootstrapToken, a service account or a bootstrap token will be created in the garden cluster and used to compute the bootstrap kubeconfig.
If set to None, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster. Defaults to BootstrapToken.</p>
</td>
</tr>
<tr>
<td>
<code>mergeWithParent</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>MergeWithParent specifies whether the deployment parameters and GardenletConfiguration of the parent gardenlet
should be merged with the specified deployment parameters and GardenletConfiguration. Defaults to true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletDeployment">GardenletDeployment
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet</a>)
</p>
<p>
<p>GardenletDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
the image, etc.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicaCount</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReplicaCount is the number of gardenlet replicas. Defaults to 1.</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>RevisionHistoryLimit is the number of old gardenlet ReplicaSets to retain to allow rollback. Defaults to 1.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountName is the name of the ServiceAccount to use to run gardenlet pods.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Image">
Image
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image is the gardenlet container image.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources are the compute resources required by the gardenlet container.</p>
</td>
</tr>
<tr>
<td>
<code>podLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodLabels are the labels on gardenlet pods.</p>
</td>
</tr>
<tr>
<td>
<code>podAnnotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodAnnotations are the annotations on gardenlet pods.</p>
</td>
</tr>
<tr>
<td>
<code>additionalVolumes</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalVolumes is the list of additional volumes that should be mounted by gardenlet containers.</p>
</td>
</tr>
<tr>
<td>
<code>additionalVolumeMounts</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalVolumeMounts is the list of additional pod volumes to mount into the gardenlet container&rsquo;s filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>env</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Env is the list of environment variables to set in the gardenlet container.</p>
</td>
</tr>
<tr>
<td>
<code>vpa</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>VPA specifies whether to enable VPA for gardenlet. Defaults to true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.Image">Image
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletDeployment">GardenletDeployment</a>)
</p>
<p>
<p>Image specifies container image parameters.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repository</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Repository is the image repository.</p>
</td>
</tr>
<tr>
<td>
<code>tag</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tag is the image tag.</p>
</td>
</tr>
<tr>
<td>
<code>pullPolicy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PullPolicy is the image pull policy. One of Always, Never, IfNotPresent.
Defaults to Always if latest tag is specified, or IfNotPresent otherwise.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">ManagedSeedSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeed">ManagedSeed</a>)
</p>
<p>
<p>ManagedSeedSpec is the specification of a ManagedSeed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>shoot</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Shoot">
Shoot
</a>
</em>
</td>
<td>
<p>Shoot references a Shoot that should be registered as Seed.</p>
</td>
</tr>
<tr>
<td>
<code>seedTemplate</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.SeedTemplate">
SeedTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedTemplate is a template for a Seed object, that should be used to register a given cluster as a Seed.
Either SeedTemplate or Gardenlet must be specified. When Seed is specified, the ManagedSeed controller will not deploy a gardenlet into the cluster
and an existing gardenlet reconciling the new Seed is required.</p>
</td>
</tr>
<tr>
<td>
<code>gardenlet</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">
Gardenlet
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardenlet specifies, that the ManagedSeed controller should deploy a gardenlet into the cluster
with the given deployment parameters and GardenletConfiguration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedStatus">ManagedSeedStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeed">ManagedSeed</a>)
</p>
<p>
<p>ManagedSeedStatus is the status of a ManagedSeed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code></br>
<em>
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a ManagedSeed&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>ObservedGeneration is the most recent generation observed for this ManagedSeed. It corresponds to the
ManagedSeed&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.SeedTemplate">SeedTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">ManagedSeedSpec</a>)
</p>
<p>
<p>SeedTemplate is a template for creating a Seed object, when registering a cluster as a ManagedSeed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedSpec
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the Seed.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>backup</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedBackup
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup holds the object store configuration for the backups of shoot (currently only etcd).
If it is not specified, then there won&rsquo;t be any backups taken for shoots associated with this seed.
If backup field is present in seed, then backups of the etcd from shoot control plane will be stored
under the configured object store.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedDNS
</em>
</td>
<td>
<p>DNS contains DNS-relevant information about this seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedNetworks
</em>
</td>
<td>
<p>Networks defines the pod, service and worker network of the Seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedProvider
</em>
</td>
<td>
<p>Provider defines the provider type and region for this Seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretRef is a reference to a Secret object containing the Kubeconfig and the cloud provider credentials for
the account the Seed cluster has been deployed to.</p>
</td>
</tr>
<tr>
<td>
<code>taints</code></br>
<em>
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedTaint
</em>
</td>
<td>
<em>(Optional)</em>
<p>Taints describes taints on the seed.</p>
</td>
</tr>
<tr>
<td>
<code>volume</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedVolume
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains settings for persistentvolumes created in the seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>settings</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.SeedSettings
</em>
</td>
<td>
<em>(Optional)</em>
<p>Settings contains certain settings for this seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.Ingress
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ingress configures Ingress specific settings of the Seed cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.Shoot">Shoot
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">ManagedSeedSpec</a>)
</p>
<p>
<p>Shoot identifies the Shoot that should be registered as Seed.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the Shoot that will be registered as Seed.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
