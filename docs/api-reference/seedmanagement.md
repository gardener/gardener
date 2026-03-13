<p>Packages:</p>
<ul>
<li>
<a href="#seedmanagement.gardener.cloud%2fv1alpha1">seedmanagement.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="seedmanagement.gardener.cloud/v1alpha1">seedmanagement.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#gardenlet">Gardenlet</a>
</li>
<li>
<a href="#managedseed">ManagedSeed</a>
</li>
<li>
<a href="#managedseedset">ManagedSeedSet</a>
</li>
<li>
<a href="#managedseedtemplate">ManagedSeedTemplate</a>
</li>
</ul>

<h3 id="bootstrap">Bootstrap
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#gardenletconfig">GardenletConfig</a>)
</p>

<p>
Bootstrap describes a mechanism for bootstrapping gardenlet connection to the Garden cluster.
</p>


<h3 id="gardenlet">Gardenlet
</h3>


<p>
Gardenlet represents a Gardenlet configuration for an unmanaged seed.
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#gardenletspec">GardenletSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the Gardenlet.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#gardenletstatus">GardenletStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Gardenlet.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenletconfig">GardenletConfig
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedspec">ManagedSeedSpec</a>)
</p>

<p>
GardenletConfig specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
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
<a href="#gardenletdeployment">GardenletDeployment</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,<br />the image, etc.</p>
</td>
</tr>
<tr>
<td>
<code>config</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
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
<a href="#bootstrap">Bootstrap</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bootstrap is the mechanism that should be used for bootstrapping gardenlet connection to the Garden cluster. One of ServiceAccount, BootstrapToken, None.<br />If set to ServiceAccount or BootstrapToken, a service account or a bootstrap token will be created in the garden cluster and used to compute the bootstrap kubeconfig.<br />If set to None, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster. Defaults to BootstrapToken.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>mergeWithParent</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>MergeWithParent specifies whether the GardenletConfiguration of the parent gardenlet<br />should be merged with the specified GardenletConfiguration. Defaults to true. This field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenletdeployment">GardenletDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#gardenletconfig">GardenletConfig</a>, <a href="#gardenletselfdeployment">GardenletSelfDeployment</a>)
</p>

<p>
GardenletDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
the image, etc.
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
integer
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
integer
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
<a href="#image">Image</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core">ResourceRequirements</a>
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
object (keys:string, values:string)
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
object (keys:string, values:string)
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#volume-v1-core">Volume</a> array
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#volumemount-v1-core">VolumeMount</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalVolumeMounts is the list of additional pod volumes to mount into the gardenlet container's filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>env</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envvar-v1-core">EnvVar</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Env is the list of environment variables to set in the gardenlet container.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#toleration-v1-core">Toleration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations are the tolerations to be applied to gardenlet pods.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenlethelm">GardenletHelm
</h3>


<p>
(<em>Appears on:</em><a href="#gardenletselfdeployment">GardenletSelfDeployment</a>)
</p>

<p>
GardenletHelm is the Helm deployment configuration for gardenlet.
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
<a href="#ocirepository">OCIRepository</a>
</em>
</td>
<td>
<p>OCIRepository defines where to pull the chart.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenletselfdeployment">GardenletSelfDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#gardenletspec">GardenletSpec</a>)
</p>

<p>
GardenletSelfDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
the image, etc.
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
integer
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
integer
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
<a href="#image">Image</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core">ResourceRequirements</a>
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
object (keys:string, values:string)
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
object (keys:string, values:string)
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#volume-v1-core">Volume</a> array
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#volumemount-v1-core">VolumeMount</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalVolumeMounts is the list of additional pod volumes to mount into the gardenlet container's filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>env</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envvar-v1-core">EnvVar</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Env is the list of environment variables to set in the gardenlet container.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#toleration-v1-core">Toleration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations are the tolerations to be applied to gardenlet pods.</p>
</td>
</tr>
<tr>
<td>
<code>helm</code></br>
<em>
<a href="#gardenlethelm">GardenletHelm</a>
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
<p>ComponentImageVectorOverwrite is the component image vector overwrite for the components deployed by this<br />gardenlet.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenletspec">GardenletSpec
</h3>


<p>
(<em>Appears on:</em><a href="#gardenlet">Gardenlet</a>)
</p>

<p>
GardenletSpec specifies gardenlet deployment parameters and the configuration used to configure gardenlet.
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
<a href="#gardenletselfdeployment">GardenletSelfDeployment</a>
</em>
</td>
<td>
<p>Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,<br />the image, etc.</p>
</td>
</tr>
<tr>
<td>
<code>config</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeconfigSecretRef is a reference to a secret containing a kubeconfig for the cluster to which gardenlet should<br />be deployed. This is only used by gardener-operator for a very first gardenlet deployment. After that, gardenlet<br />will continuously upgrade itself. If this field is empty, gardener-operator deploys it into its own runtime<br />cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenletstatus">GardenletStatus
</h3>


<p>
(<em>Appears on:</em><a href="#gardenlet">Gardenlet</a>)
</p>

<p>
GardenletStatus is the status of a Gardenlet.
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
Condition array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Gardenlet's current state.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this Gardenlet. It corresponds to the Gardenlet's<br />generation, which is updated on mutation by the API Server.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="image">Image
</h3>


<p>
(<em>Appears on:</em><a href="#gardenletdeployment">GardenletDeployment</a>, <a href="#gardenletselfdeployment">GardenletSelfDeployment</a>)
</p>

<p>
Image specifies container image parameters.
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#pullpolicy-v1-core">PullPolicy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PullPolicy is the image pull policy. One of Always, Never, IfNotPresent.<br />Defaults to Always if latest tag is specified, or IfNotPresent otherwise.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseed">ManagedSeed
</h3>


<p>
ManagedSeed represents a Shoot that is registered as Seed.
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#managedseedspec">ManagedSeedSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the ManagedSeed.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#managedseedstatus">ManagedSeedStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the ManagedSeed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseedset">ManagedSeedSet
</h3>


<p>
ManagedSeedSet represents a set of identical ManagedSeeds.
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#managedseedsetspec">ManagedSeedSetSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the desired identities of ManagedSeeds and Shoots in this set.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#managedseedsetstatus">ManagedSeedSetStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the current status of ManagedSeeds and Shoots in this ManagedSeedSet.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseedsetspec">ManagedSeedSetSpec
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedset">ManagedSeedSet</a>)
</p>

<p>
ManagedSeedSetSpec is the specification of a ManagedSeedSet.
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
integer
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<p>Selector is a label query over ManagedSeeds and Shoots that should match the replica count.<br />It must match the ManagedSeeds and Shoots template's labels. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>template</code></br>
<em>
<a href="#managedseedtemplate">ManagedSeedTemplate</a>
</em>
</td>
<td>
<p>Template describes the ManagedSeed that will be created if insufficient replicas are detected.<br />Each ManagedSeed created / updated by the ManagedSeedSet will fulfill this template.</p>
</td>
</tr>
<tr>
<td>
<code>shootTemplate</code></br>
<em>
<a href="#shoottemplate">ShootTemplate</a>
</em>
</td>
<td>
<p>ShootTemplate describes the Shoot that will be created if insufficient replicas are detected for hosting the corresponding ManagedSeed.<br />Each Shoot created / updated by the ManagedSeedSet will fulfill this template.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#updatestrategy">UpdateStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdateStrategy specifies the UpdateStrategy that will be<br />employed to update ManagedSeeds / Shoots in the ManagedSeedSet when a revision is made to<br />Template / ShootTemplate.</p>
</td>
</tr>
<tr>
<td>
<code>revisionHistoryLimit</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>RevisionHistoryLimit is the maximum number of revisions that will be maintained<br />in the ManagedSeedSet's revision history. Defaults to 10. This field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseedsetstatus">ManagedSeedSetStatus
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedset">ManagedSeedSet</a>)
</p>

<p>
ManagedSeedSetStatus represents the current state of a ManagedSeedSet.
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
integer
</em>
</td>
<td>
<p>ObservedGeneration is the most recent generation observed for this ManagedSeedSet. It corresponds to the<br />ManagedSeedSet's generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
integer
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
integer
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
integer
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
integer
</em>
</td>
<td>
<p>CurrentReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller from the ManagedSeedSet version<br />indicated by CurrentRevision.</p>
</td>
</tr>
<tr>
<td>
<code>updatedReplicas</code></br>
<em>
integer
</em>
</td>
<td>
<p>UpdatedReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller from the ManagedSeedSet version<br />indicated by UpdateRevision.</p>
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
<p>CurrentRevision, if not empty, indicates the version of the ManagedSeedSet used to generate ManagedSeeds with smaller<br />ordinal numbers during updates.</p>
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
<p>UpdateRevision, if not empty, indicates the version of the ManagedSeedSet used to generate ManagedSeeds with larger<br />ordinal numbers during updates</p>
</td>
</tr>
<tr>
<td>
<code>collisionCount</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>CollisionCount is the count of hash collisions for the ManagedSeedSet. The ManagedSeedSet controller<br />uses this field as a collision avoidance mechanism when it needs to create the name for the<br />newest ControllerRevision.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
Condition array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a ManagedSeedSet's current state.</p>
</td>
</tr>
<tr>
<td>
<code>pendingReplica</code></br>
<em>
<a href="#pendingreplica">PendingReplica</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingReplica, if not empty, indicates the replica that is currently pending creation, update, or deletion.<br />This replica is in a state that requires the controller to wait for it to change before advancing to the next replica.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseedspec">ManagedSeedSpec
</h3>


<p>
(<em>Appears on:</em><a href="#managedseed">ManagedSeed</a>, <a href="#managedseedtemplate">ManagedSeedTemplate</a>)
</p>

<p>
ManagedSeedSpec is the specification of a ManagedSeed.
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
<a href="#shoot">Shoot</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Shoot references a Shoot that should be registered as Seed.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>gardenlet</code></br>
<em>
<a href="#gardenletconfig">GardenletConfig</a>
</em>
</td>
<td>
<p>Gardenlet specifies that the ManagedSeed controller should deploy a gardenlet into the cluster<br />with the given deployment parameters and GardenletConfiguration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseedstatus">ManagedSeedStatus
</h3>


<p>
(<em>Appears on:</em><a href="#managedseed">ManagedSeed</a>)
</p>

<p>
ManagedSeedStatus is the status of a ManagedSeed.
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
Condition array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a ManagedSeed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<p>ObservedGeneration is the most recent generation observed for this ManagedSeed. It corresponds to the<br />ManagedSeed's generation, which is updated on mutation by the API Server.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedseedtemplate">ManagedSeedTemplate
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedsetspec">ManagedSeedSetSpec</a>)
</p>

<p>
ManagedSeedTemplate is a template for creating a ManagedSeed object.
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta">ObjectMeta</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#managedseedspec">ManagedSeedSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the ManagedSeed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="pendingreplica">PendingReplica
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedsetstatus">ManagedSeedSetStatus</a>)
</p>

<p>
PendingReplica contains information about a replica that is currently pending creation, update, or deletion.
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
<a href="#pendingreplicareason">PendingReplicaReason</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Retries is the number of times the shoot operation (reconcile or delete) has been retried after having failed.<br />Only applicable if Reason is ShootReconciling or ShootDeleting.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="pendingreplicareason">PendingReplicaReason
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#pendingreplica">PendingReplica</a>)
</p>

<p>
PendingReplicaReason is a string enumeration type that enumerates all possible reasons for a replica to be pending.
</p>


<h3 id="rollingupdatestrategy">RollingUpdateStrategy
</h3>


<p>
(<em>Appears on:</em><a href="#updatestrategy">UpdateStrategy</a>)
</p>

<p>
RollingUpdateStrategy is used to communicate parameters for RollingUpdateStrategyType.
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Partition indicates the ordinal at which the ManagedSeedSet should be partitioned. Defaults to 0.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shoot">Shoot
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedspec">ManagedSeedSpec</a>)
</p>

<p>
Shoot identifies the Shoot that should be registered as Seed.
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


<h3 id="updatestrategy">UpdateStrategy
</h3>


<p>
(<em>Appears on:</em><a href="#managedseedsetspec">ManagedSeedSetSpec</a>)
</p>

<p>
UpdateStrategy specifies the strategy that the ManagedSeedSet
controller will use to perform updates. It includes any additional parameters
necessary to perform the update for the indicated strategy.
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
<a href="#updatestrategytype">UpdateStrategyType</a>
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
<a href="#rollingupdatestrategy">RollingUpdateStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RollingUpdate is used to communicate parameters when Type is RollingUpdateStrategyType.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="updatestrategytype">UpdateStrategyType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#updatestrategy">UpdateStrategy</a>)
</p>

<p>
UpdateStrategyType is a string enumeration type that enumerates
all possible update strategies for the ManagedSeedSet controller.
</p>


