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
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet</a>
</li><li>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeed">ManagedSeed</a>
</li><li>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSet">ManagedSeedSet</a>
</li></ul>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet
</h3>
<p>
<p>Gardenlet represents a Gardenlet configuration for an unmanaged seed.</p>
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
<td><code>Gardenlet</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
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
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletSpec">
GardenletSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the Gardenlet.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletSelfDeployment">
GardenletSelfDeployment
</a>
</em>
</td>
<td>
<p>Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
the image, etc.</p>
</td>
</tr>
<tr>
<td>
<code>config</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config is the GardenletConfiguration used to configure gardenlet.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfigSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeconfigSecretRef is a reference to a secret containing a kubeconfig for the cluster to which gardenlet should
be deployed. This is only used by gardener-operator for a very first gardenlet deployment. After that, gardenlet
will continuously upgrade itself. If this field is empty, gardener-operator deploys it into its own runtime
cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletStatus">
GardenletStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Gardenlet.</p>
</td>
</tr>
</tbody>
</table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
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
<em>(Optional)</em>
<p>Shoot references a Shoot that should be registered as Seed.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>gardenlet</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletConfig">
GardenletConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardenlet specifies that the ManagedSeed controller should deploy a gardenlet into the cluster
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
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSet">ManagedSeedSet
</h3>
<p>
<p>ManagedSeedSet represents a set of identical ManagedSeeds.</p>
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
<td><code>ManagedSeedSet</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
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
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetSpec">
ManagedSeedSetSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the desired identities of ManagedSeeds and Shoots in this set.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Replicas is the desired number of replicas of the given Template. Defaults to 1.</p>
</td>
</tr>
<tr>
<td>
<code>selector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>Selector is a label query over ManagedSeeds and Shoots that should match the replica count.
It must match the ManagedSeeds and Shoots template&rsquo;s labels. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>template</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedTemplate">
ManagedSeedTemplate
</a>
</em>
</td>
<td>
<p>Template describes the ManagedSeed that will be created if insufficient replicas are detected.
Each ManagedSeed created / updated by the ManagedSeedSet will fulfill this template.</p>
</td>
</tr>
<tr>
<td>
<code>shootTemplate</code></br>
<em>
<a href="./core.md#core.gardener.cloud/v1beta1.ShootTemplate">
github.com/gardener/gardener/pkg/apis/core/v1beta1.ShootTemplate
</a>
</em>
</td>
<td>
<p>ShootTemplate describes the Shoot that will be created if insufficient replicas are detected for hosting the corresponding ManagedSeed.
Each Shoot created / updated by the ManagedSeedSet will fulfill this template.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.UpdateStrategy">
UpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdateStrategy specifies the UpdateStrategy that will be
employed to update ManagedSeeds / Shoots in the ManagedSeedSet when a revision is made to
Template / ShootTemplate.</p>
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
<p>RevisionHistoryLimit is the maximum number of revisions that will be maintained
in the ManagedSeedSet&rsquo;s revision history. Defaults to 10. This field is immutable.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetStatus">
ManagedSeedSetStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the current status of ManagedSeeds and Shoots in this ManagedSeedSet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.Bootstrap">Bootstrap
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletConfig">GardenletConfig</a>)
</p>
<p>
<p>Bootstrap describes a mechanism for bootstrapping gardenlet connection to the Garden cluster.</p>
</p>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletConfig">GardenletConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">ManagedSeedSpec</a>)
</p>
<p>
<p>GardenletConfig specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.</p>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
If set to None, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster. Defaults to BootstrapToken.
This field is immutable.</p>
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
<p>MergeWithParent specifies whether the GardenletConfiguration of the parent gardenlet
should be merged with the specified GardenletConfiguration. Defaults to true. This field is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletDeployment">GardenletDeployment
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletConfig">GardenletConfig</a>, 
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletSelfDeployment">GardenletSelfDeployment</a>)
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
<p>ReplicaCount is the number of gardenlet replicas. Defaults to 2.</p>
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
<p>RevisionHistoryLimit is the number of old gardenlet ReplicaSets to retain to allow rollback. Defaults to 2.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcerequirements-v1-core">
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#volume-v1-core">
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#volumemount-v1-core">
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#envvar-v1-core">
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
<p>Deprecated: This field is deprecated and has no effect anymore. It will be removed in the future.
TODO(rfranzke): Remove this field after v1.110 has been released.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletHelm">GardenletHelm
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletSelfDeployment">GardenletSelfDeployment</a>)
</p>
<p>
<p>GardenletHelm is the Helm deployment configuration for gardenlet.</p>
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
<code>ociRepository</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1.OCIRepository
</em>
</td>
<td>
<p>OCIRepository defines where to pull the chart.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletSelfDeployment">GardenletSelfDeployment
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletSpec">GardenletSpec</a>)
</p>
<p>
<p>GardenletSelfDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
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
<code>GardenletDeployment</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletDeployment">
GardenletDeployment
</a>
</em>
</td>
<td>
<p>
(Members of <code>GardenletDeployment</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>GardenletDeployment specifies common gardenlet deployment parameters.</p>
</td>
</tr>
<tr>
<td>
<code>helm</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletHelm">
GardenletHelm
</a>
</em>
</td>
<td>
<p>Helm is the Helm deployment configuration.</p>
</td>
</tr>
<tr>
<td>
<code>imageVectorOverwrite</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageVectorOverwrite is the image vector overwrite for the components deployed by this gardenlet.</p>
</td>
</tr>
<tr>
<td>
<code>componentImageVectorOverwrite</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ComponentImageVectorOverwrite is the component image vector overwrite for the components deployed by this
gardenlet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletSpec">GardenletSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet</a>)
</p>
<p>
<p>GardenletSpec specifies gardenlet deployment parameters and the configuration used to configure gardenlet.</p>
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
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletSelfDeployment">
GardenletSelfDeployment
</a>
</em>
</td>
<td>
<p>Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
the image, etc.</p>
</td>
</tr>
<tr>
<td>
<code>config</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config is the GardenletConfiguration used to configure gardenlet.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfigSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeconfigSecretRef is a reference to a secret containing a kubeconfig for the cluster to which gardenlet should
be deployed. This is only used by gardener-operator for a very first gardenlet deployment. After that, gardenlet
will continuously upgrade itself. If this field is empty, gardener-operator deploys it into its own runtime
cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.GardenletStatus">GardenletStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.Gardenlet">Gardenlet</a>)
</p>
<p>
<p>GardenletStatus is the status of a Gardenlet.</p>
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
<a href="./core.md#core.gardener.cloud/v1beta1.Condition">
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Gardenlet&rsquo;s current state.</p>
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
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this Gardenlet. It corresponds to the Gardenlet&rsquo;s
generation, which is updated on mutation by the API Server.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#pullpolicy-v1-core">
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
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetSpec">ManagedSeedSetSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSet">ManagedSeedSet</a>)
</p>
<p>
<p>ManagedSeedSetSpec is the specification of a ManagedSeedSet.</p>
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
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Replicas is the desired number of replicas of the given Template. Defaults to 1.</p>
</td>
</tr>
<tr>
<td>
<code>selector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>Selector is a label query over ManagedSeeds and Shoots that should match the replica count.
It must match the ManagedSeeds and Shoots template&rsquo;s labels. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>template</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedTemplate">
ManagedSeedTemplate
</a>
</em>
</td>
<td>
<p>Template describes the ManagedSeed that will be created if insufficient replicas are detected.
Each ManagedSeed created / updated by the ManagedSeedSet will fulfill this template.</p>
</td>
</tr>
<tr>
<td>
<code>shootTemplate</code></br>
<em>
<a href="./core.md#core.gardener.cloud/v1beta1.ShootTemplate">
github.com/gardener/gardener/pkg/apis/core/v1beta1.ShootTemplate
</a>
</em>
</td>
<td>
<p>ShootTemplate describes the Shoot that will be created if insufficient replicas are detected for hosting the corresponding ManagedSeed.
Each Shoot created / updated by the ManagedSeedSet will fulfill this template.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.UpdateStrategy">
UpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdateStrategy specifies the UpdateStrategy that will be
employed to update ManagedSeeds / Shoots in the ManagedSeedSet when a revision is made to
Template / ShootTemplate.</p>
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
<p>RevisionHistoryLimit is the maximum number of revisions that will be maintained
in the ManagedSeedSet&rsquo;s revision history. Defaults to 10. This field is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetStatus">ManagedSeedSetStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSet">ManagedSeedSet</a>)
</p>
<p>
<p>ManagedSeedSetStatus represents the current state of a ManagedSeedSet.</p>
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
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>ObservedGeneration is the most recent generation observed for this ManagedSeedSet. It corresponds to the
ManagedSeedSet&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the number of replicas (ManagedSeeds and their corresponding Shoots) created by the ManagedSeedSet controller.</p>
</td>
</tr>
<tr>
<td>
<code>readyReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>ReadyReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller that have a Ready Condition.</p>
</td>
</tr>
<tr>
<td>
<code>nextReplicaNumber</code></br>
<em>
int32
</em>
</td>
<td>
<p>NextReplicaNumber is the ordinal number that will be assigned to the next replica of the ManagedSeedSet.</p>
</td>
</tr>
<tr>
<td>
<code>currentReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>CurrentReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller from the ManagedSeedSet version
indicated by CurrentRevision.</p>
</td>
</tr>
<tr>
<td>
<code>updatedReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>UpdatedReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller from the ManagedSeedSet version
indicated by UpdateRevision.</p>
</td>
</tr>
<tr>
<td>
<code>currentRevision</code></br>
<em>
string
</em>
</td>
<td>
<p>CurrentRevision, if not empty, indicates the version of the ManagedSeedSet used to generate ManagedSeeds with smaller
ordinal numbers during updates.</p>
</td>
</tr>
<tr>
<td>
<code>updateRevision</code></br>
<em>
string
</em>
</td>
<td>
<p>UpdateRevision, if not empty, indicates the version of the ManagedSeedSet used to generate ManagedSeeds with larger
ordinal numbers during updates</p>
</td>
</tr>
<tr>
<td>
<code>collisionCount</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>CollisionCount is the count of hash collisions for the ManagedSeedSet. The ManagedSeedSet controller
uses this field as a collision avoidance mechanism when it needs to create the name for the
newest ControllerRevision.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="./core.md#core.gardener.cloud/v1beta1.Condition">
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a ManagedSeedSet&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>pendingReplica</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.PendingReplica">
PendingReplica
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingReplica, if not empty, indicates the replica that is currently pending creation, update, or deletion.
This replica is in a state that requires the controller to wait for it to change before advancing to the next replica.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSpec">ManagedSeedSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeed">ManagedSeed</a>, 
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedTemplate">ManagedSeedTemplate</a>)
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
<em>(Optional)</em>
<p>Shoot references a Shoot that should be registered as Seed.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>gardenlet</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletConfig">
GardenletConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardenlet specifies that the ManagedSeed controller should deploy a gardenlet into the cluster
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
<a href="./core.md#core.gardener.cloud/v1beta1.Condition">
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition
</a>
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
<h3 id="seedmanagement.gardener.cloud/v1alpha1.ManagedSeedTemplate">ManagedSeedTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetSpec">ManagedSeedSetSpec</a>)
</p>
<p>
<p>ManagedSeedTemplate is a template for creating a ManagedSeed object.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
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
<p>Specification of the desired behavior of the ManagedSeed.</p>
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
<em>(Optional)</em>
<p>Shoot references a Shoot that should be registered as Seed.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>gardenlet</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.GardenletConfig">
GardenletConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardenlet specifies that the ManagedSeed controller should deploy a gardenlet into the cluster
with the given deployment parameters and GardenletConfiguration.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.PendingReplica">PendingReplica
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetStatus">ManagedSeedSetStatus</a>)
</p>
<p>
<p>PendingReplica contains information about a replica that is currently pending creation, update, or deletion.</p>
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
<p>Name is the replica name.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.PendingReplicaReason">
PendingReplicaReason
</a>
</em>
</td>
<td>
<p>Reason is the reason for the replica to be pending.</p>
</td>
</tr>
<tr>
<td>
<code>since</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Since is the moment in time since the replica is pending with the specified reason.</p>
</td>
</tr>
<tr>
<td>
<code>retries</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries is the number of times the shoot operation (reconcile or delete) has been retried after having failed.
Only applicable if Reason is ShootReconciling or ShootDeleting.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.PendingReplicaReason">PendingReplicaReason
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.PendingReplica">PendingReplica</a>)
</p>
<p>
<p>PendingReplicaReason is a string enumeration type that enumerates all possible reasons for a replica to be pending.</p>
</p>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.RollingUpdateStrategy">RollingUpdateStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.UpdateStrategy">UpdateStrategy</a>)
</p>
<p>
<p>RollingUpdateStrategy is used to communicate parameters for RollingUpdateStrategyType.</p>
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
<code>partition</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Partition indicates the ordinal at which the ManagedSeedSet should be partitioned. Defaults to 0.</p>
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
<h3 id="seedmanagement.gardener.cloud/v1alpha1.UpdateStrategy">UpdateStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.ManagedSeedSetSpec">ManagedSeedSetSpec</a>)
</p>
<p>
<p>UpdateStrategy specifies the strategy that the ManagedSeedSet
controller will use to perform updates. It includes any additional parameters
necessary to perform the update for the indicated strategy.</p>
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
<code>type</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.UpdateStrategyType">
UpdateStrategyType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type indicates the type of the UpdateStrategy. Defaults to RollingUpdate.</p>
</td>
</tr>
<tr>
<td>
<code>rollingUpdate</code></br>
<em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.RollingUpdateStrategy">
RollingUpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RollingUpdate is used to communicate parameters when Type is RollingUpdateStrategyType.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="seedmanagement.gardener.cloud/v1alpha1.UpdateStrategyType">UpdateStrategyType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#seedmanagement.gardener.cloud/v1alpha1.UpdateStrategy">UpdateStrategy</a>)
</p>
<p>
<p>UpdateStrategyType is a string enumeration type that enumerates
all possible update strategies for the ManagedSeedSet controller.</p>
</p>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
