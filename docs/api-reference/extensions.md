<p>Packages:</p>
<ul>
<li>
<a href="#extensions.gardener.cloud%2fv1alpha1">extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="extensions.gardener.cloud/v1alpha1">extensions.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#backupbucket">BackupBucket</a>
</li>
<li>
<a href="#backupentry">BackupEntry</a>
</li>
<li>
<a href="#bastion">Bastion</a>
</li>
<li>
<a href="#cluster">Cluster</a>
</li>
<li>
<a href="#containerruntime">ContainerRuntime</a>
</li>
<li>
<a href="#controlplane">ControlPlane</a>
</li>
<li>
<a href="#dnsrecord">DNSRecord</a>
</li>
<li>
<a href="#extension">Extension</a>
</li>
<li>
<a href="#infrastructure">Infrastructure</a>
</li>
<li>
<a href="#network">Network</a>
</li>
<li>
<a href="#operatingsystemconfig">OperatingSystemConfig</a>
</li>
<li>
<a href="#selfhostedshootexposure">SelfHostedShootExposure</a>
</li>
<li>
<a href="#worker">Worker</a>
</li>
</ul>

<h3 id="backupbucket">BackupBucket
</h3>


<p>
BackupBucket is a specification for backup bucket.
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
<a href="#backupbucketspec">BackupBucketSpec</a>
</em>
</td>
<td>
<p>Specification of the BackupBucket.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#backupbucketstatus">BackupBucketStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupbucketspec">BackupBucketSpec
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucket">BackupBucket</a>)
</p>

<p>
BackupBucketSpec is the spec for an BackupBucket resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the region of this bucket. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the credentials to access object store.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupbucketstatus">BackupBucketStatus
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucket">BackupBucket</a>)
</p>

<p>
BackupBucketStatus is the status for an BackupBucket resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>generatedSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GeneratedSecretRef is reference to the secret generated by backup bucket, which<br />will have object store specific credentials.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupentry">BackupEntry
</h3>


<p>
BackupEntry is a specification for backup Entry.
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
<a href="#backupentryspec">BackupEntrySpec</a>
</em>
</td>
<td>
<p>Specification of the BackupEntry.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#backupentrystatus">BackupEntryStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupentryspec">BackupEntrySpec
</h3>


<p>
(<em>Appears on:</em><a href="#backupentry">BackupEntry</a>)
</p>

<p>
BackupEntrySpec is the spec for an BackupEntry resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>backupBucketProviderStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>BackupBucketProviderStatus contains the provider status that has<br />been generated by the controller responsible for the `BackupBucket` resource.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the region of this Entry. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>bucketName</code></br>
<em>
string
</em>
</td>
<td>
<p>BucketName is the name of backup bucket for this Backup Entry.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the credentials to access object store.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupentrystatus">BackupEntryStatus
</h3>


<p>
(<em>Appears on:</em><a href="#backupentry">BackupEntry</a>)
</p>

<p>
BackupEntryStatus is the status for an BackupEntry resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastion">Bastion
</h3>


<p>
Bastion is a bastion or jump host that is dynamically created
to provide SSH access to shoot nodes.
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
<a href="#bastionspec">BastionSpec</a>
</em>
</td>
<td>
<p>Spec is the specification of this Bastion.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#bastionstatus">BastionStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status is the bastion's status.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastioningresspolicy">BastionIngressPolicy
</h3>


<p>
(<em>Appears on:</em><a href="#bastionspec">BastionSpec</a>)
</p>

<p>
BastionIngressPolicy represents an ingress policy for SSH bastion hosts.
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
<code>ipBlock</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#ipblock-v1-networking">IPBlock</a>
</em>
</td>
<td>
<p>IPBlock defines an IP block that is allowed to access the bastion.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastionspec">BastionSpec
</h3>


<p>
(<em>Appears on:</em><a href="#bastion">Bastion</a>)
</p>

<p>
BastionSpec contains the specification for an SSH bastion host.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>userData</code></br>
<em>
integer array
</em>
</td>
<td>
<p>UserData is the base64-encoded user data for the bastion instance. This should<br />contain code to provision the SSH key on the bastion instance.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
<a href="#bastioningresspolicy">BastionIngressPolicy</a> array
</em>
</td>
<td>
<p>Ingress controls from where the created bastion host should be reachable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastionstatus">BastionStatus
</h3>


<p>
(<em>Appears on:</em><a href="#bastion">Bastion</a>)
</p>

<p>
BastionStatus holds the most recently observed status of the Bastion.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#loadbalanceringress-v1-core">LoadBalancerIngress</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ingress is the external IP and/or hostname of the bastion host.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="carotation">CARotation
</h3>


<p>
(<em>Appears on:</em><a href="#credentialsrotation">CredentialsRotation</a>)
</p>

<p>
CARotation contains information about the certificate authority credential rotation.
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
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationTime is the most recent time when the certificate authority credential rotation was initiated.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="criconfig">CRIConfig
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>)
</p>

<p>
CRIConfig contains configurations of the CRI library.
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
<a href="#criname">CRIName</a>
</em>
</td>
<td>
<p>Name is a mandatory string containing the name of the CRI library. Supported values are `containerd`.</p>
</td>
</tr>
<tr>
<td>
<code>cgroupDriver</code></br>
<em>
<a href="#cgroupdrivername">CgroupDriverName</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CgroupDriver configures the CRI's cgroup driver. Supported values are `cgroupfs` or `systemd`.</p>
</td>
</tr>
<tr>
<td>
<code>containerd</code></br>
<em>
<a href="#containerdconfig">ContainerdConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContainerdConfig is the containerd configuration.<br />Only to be set for OperatingSystemConfigs with purpose 'reconcile'.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="criname">CRIName
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#criconfig">CRIConfig</a>)
</p>

<p>
CRIName is a type alias for the CRI name string.
</p>


<h3 id="cgroupdrivername">CgroupDriverName
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#criconfig">CRIConfig</a>)
</p>

<p>
CgroupDriverName is a string denoting the CRI cgroup driver.
</p>


<h3 id="cloudconfig">CloudConfig
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigstatus">OperatingSystemConfigStatus</a>)
</p>

<p>
CloudConfig contains the generated output for the given operating system
config spec. It contains a reference to a secret as the result may contain confidential data.
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
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the actual result of the generated cloud config.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cluster">Cluster
</h3>


<p>
Cluster is a specification for a Cluster resource.
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
<a href="#clusterspec">ClusterSpec</a>
</em>
</td>
<td>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="clusterautoscaleroptions">ClusterAutoscalerOptions
</h3>


<p>
(<em>Appears on:</em><a href="#workerpool">WorkerPool</a>)
</p>

<p>
ClusterAutoscalerOptions contains the cluster autoscaler configurations for a worker pool.
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
<code>scaleDownUtilizationThreshold</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownUtilizationThreshold defines the threshold in fraction (0.0 - 1.0) under which a node is being removed.</p>
</td>
</tr>
<tr>
<td>
<code>scaleDownGpuUtilizationThreshold</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownGpuUtilizationThreshold defines the threshold in fraction (0.0 - 1.0) of gpu resources under which a node is being removed.</p>
</td>
</tr>
<tr>
<td>
<code>scaleDownUnneededTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down.</p>
</td>
</tr>
<tr>
<td>
<code>scaleDownUnreadyTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownUnreadyTime defines how long an unready node should be unneeded before it is eligible for scale down.</p>
</td>
</tr>
<tr>
<td>
<code>maxNodeProvisionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNodeProvisionTime defines how long cluster autoscaler should wait for a node to be provisioned.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="clusterspec">ClusterSpec
</h3>


<p>
(<em>Appears on:</em><a href="#cluster">Cluster</a>)
</p>

<p>
ClusterSpec is the spec for a Cluster resource.
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
<code>cloudProfile</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>CloudProfile is a raw extension field that contains the cloudprofile resource referenced<br />by the shoot that has to be reconciled.</p>
</td>
</tr>
<tr>
<td>
<code>seed</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>Seed is a raw extension field that contains the seed resource referenced by the shoot that<br />has to be reconciled.</p>
</td>
</tr>
<tr>
<td>
<code>shoot</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>Shoot is a raw extension field that contains the shoot resource that has to be reconciled.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="containerruntime">ContainerRuntime
</h3>


<p>
ContainerRuntime is a specification for a container runtime resource.
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
<a href="#containerruntimespec">ContainerRuntimeSpec</a>
</em>
</td>
<td>
<p>Specification of the ContainerRuntime.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#containerruntimestatus">ContainerRuntimeStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="containerruntimespec">ContainerRuntimeSpec
</h3>


<p>
(<em>Appears on:</em><a href="#containerruntime">ContainerRuntime</a>)
</p>

<p>
ContainerRuntimeSpec is the spec for a ContainerRuntime resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>binaryPath</code></br>
<em>
string
</em>
</td>
<td>
<p>BinaryPath is the Worker's machine path where container runtime extensions should copy the binaries to.</p>
</td>
</tr>
<tr>
<td>
<code>workerPool</code></br>
<em>
<a href="#containerruntimeworkerpool">ContainerRuntimeWorkerPool</a>
</em>
</td>
<td>
<p>WorkerPool identifies the worker pool of the Shoot.<br />For each worker pool and type, Gardener deploys a ContainerRuntime CRD.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="containerruntimestatus">ContainerRuntimeStatus
</h3>


<p>
(<em>Appears on:</em><a href="#containerruntime">ContainerRuntime</a>)
</p>

<p>
ContainerRuntimeStatus is the status for a ContainerRuntime resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="containerruntimeworkerpool">ContainerRuntimeWorkerPool
</h3>


<p>
(<em>Appears on:</em><a href="#containerruntimespec">ContainerRuntimeSpec</a>)
</p>

<p>
ContainerRuntimeWorkerPool identifies a Shoot worker pool by its name and selector.
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
<p>Name specifies the name of the worker pool the container runtime should be available for.<br />This field is immutable.</p>
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
<p>Selector is the label selector used by the extension to match the nodes belonging to the worker pool.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="containerdconfig">ContainerdConfig
</h3>


<p>
(<em>Appears on:</em><a href="#criconfig">CRIConfig</a>)
</p>

<p>
ContainerdConfig contains configuration options for containerd.
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
<code>registries</code></br>
<em>
<a href="#registryconfig">RegistryConfig</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Registries configures the registry hosts for containerd.</p>
</td>
</tr>
<tr>
<td>
<code>sandboxImage</code></br>
<em>
string
</em>
</td>
<td>
<p>SandboxImage configures the sandbox image for containerd.</p>
</td>
</tr>
<tr>
<td>
<code>plugins</code></br>
<em>
<a href="#pluginconfig">PluginConfig</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Plugins configures the plugins section in containerd's config.toml.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplane">ControlPlane
</h3>


<p>
ControlPlane is a specification for a ControlPlane resource.
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
<a href="#controlplanespec">ControlPlaneSpec</a>
</em>
</td>
<td>
<p>Specification of the ControlPlane.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#controlplanestatus">ControlPlaneStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplaneendpoint">ControlPlaneEndpoint
</h3>


<p>
(<em>Appears on:</em><a href="#selfhostedshootexposurespec">SelfHostedShootExposureSpec</a>)
</p>

<p>
ControlPlaneEndpoint is an endpoint that should be exposed.
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
<code>nodeName</code></br>
<em>
string
</em>
</td>
<td>
<p>NodeName is the name of the node to expose.</p>
</td>
</tr>
<tr>
<td>
<code>addresses</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#nodeaddress-v1-core">NodeAddress</a> array
</em>
</td>
<td>
<p>Addresses is a list of addresses of type NodeAddress to expose.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplanespec">ControlPlaneSpec
</h3>


<p>
(<em>Appears on:</em><a href="#controlplane">ControlPlane</a>)
</p>

<p>
ControlPlaneSpec is the spec of a ControlPlane resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>infrastructureProviderStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InfrastructureProviderStatus contains the provider status that has<br />been generated by the controller responsible for the `Infrastructure` resource.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the region of this control plane. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the cloud provider specific credentials.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplanestatus">ControlPlaneStatus
</h3>


<p>
(<em>Appears on:</em><a href="#controlplane">ControlPlane</a>)
</p>

<p>
ControlPlaneStatus is the status of a ControlPlane resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="credentialsrotation">CredentialsRotation
</h3>


<p>
(<em>Appears on:</em><a href="#inplaceupdates">InPlaceUpdates</a>)
</p>

<p>
CredentialsRotation is a structure containing information about the last initiation time of the certificate authority and service account key rotation.
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
<code>certificateAuthorities</code></br>
<em>
<a href="#carotation">CARotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertificateAuthorities contains information about the certificate authority credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountKey</code></br>
<em>
<a href="#serviceaccountkeyrotation">ServiceAccountKeyRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountKey contains information about the service account key credential rotation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsrecord">DNSRecord
</h3>


<p>
DNSRecord is a specification for a DNSRecord resource.
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
<a href="#dnsrecordspec">DNSRecordSpec</a>
</em>
</td>
<td>
<p>Specification of the DNSRecord.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#dnsrecordstatus">DNSRecordStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsrecordspec">DNSRecordSpec
</h3>


<p>
(<em>Appears on:</em><a href="#dnsrecord">DNSRecord</a>)
</p>

<p>
DNSRecordSpec is the spec of a DNSRecord resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the cloud provider specific credentials.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Region is the region of this DNS record. If not specified, the region specified in SecretRef will be used.<br />If that is also not specified, the extension controller will use its default region.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone is the DNS hosted zone of this DNS record. If not specified, it will be determined automatically by<br />getting all hosted zones of the account and searching for the longest zone name that is a suffix of Name.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the fully qualified domain name, e.g. "api.<shoot domain>". This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>recordType</code></br>
<em>
<a href="#dnsrecordtype">DNSRecordType</a>
</em>
</td>
<td>
<p>RecordType is the DNS record type. Only A, CNAME, and TXT records are currently supported. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
string array
</em>
</td>
<td>
<p>Values is a list of IP addresses for A records, a single hostname for CNAME records, or a list of texts for TXT records.</p>
</td>
</tr>
<tr>
<td>
<code>ttl</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTL is the time to live in seconds. Defaults to 120.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsrecordstatus">DNSRecordStatus
</h3>


<p>
(<em>Appears on:</em><a href="#dnsrecord">DNSRecord</a>)
</p>

<p>
DNSRecordStatus is the status of a DNSRecord resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone is the DNS hosted zone of this DNS record.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsrecordtype">DNSRecordType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#dnsrecordspec">DNSRecordSpec</a>)
</p>

<p>
DNSRecordType is a string alias.
</p>


<h3 id="datavolume">DataVolume
</h3>


<p>
(<em>Appears on:</em><a href="#workerpool">WorkerPool</a>)
</p>

<p>
DataVolume contains information about a data volume.
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
<p>Name of the volume to make it referenceable.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the type of the volume.</p>
</td>
</tr>
<tr>
<td>
<code>size</code></br>
<em>
string
</em>
</td>
<td>
<p>Size is the of the root volume.</p>
</td>
</tr>
<tr>
<td>
<code>encrypted</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Encrypted determines if the volume should be encrypted.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="defaultspec">DefaultSpec
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketspec">BackupBucketSpec</a>, <a href="#backupentryspec">BackupEntrySpec</a>, <a href="#bastionspec">BastionSpec</a>, <a href="#containerruntimespec">ContainerRuntimeSpec</a>, <a href="#controlplanespec">ControlPlaneSpec</a>, <a href="#dnsrecordspec">DNSRecordSpec</a>, <a href="#extensionspec">ExtensionSpec</a>, <a href="#infrastructurespec">InfrastructureSpec</a>, <a href="#networkspec">NetworkSpec</a>, <a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>, <a href="#selfhostedshootexposurespec">SelfHostedShootExposureSpec</a>, <a href="#workerspec">WorkerSpec</a>)
</p>

<p>
DefaultSpec contains common status fields for every extension resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="defaultstatus">DefaultStatus
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketstatus">BackupBucketStatus</a>, <a href="#backupentrystatus">BackupEntryStatus</a>, <a href="#bastionstatus">BastionStatus</a>, <a href="#containerruntimestatus">ContainerRuntimeStatus</a>, <a href="#controlplanestatus">ControlPlaneStatus</a>, <a href="#dnsrecordstatus">DNSRecordStatus</a>, <a href="#extensionstatus">ExtensionStatus</a>, <a href="#infrastructurestatus">InfrastructureStatus</a>, <a href="#networkstatus">NetworkStatus</a>, <a href="#operatingsystemconfigstatus">OperatingSystemConfigStatus</a>, <a href="#selfhostedshootexposurestatus">SelfHostedShootExposureStatus</a>, <a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
DefaultStatus contains common status fields for every extension resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dropin">DropIn
</h3>


<p>
(<em>Appears on:</em><a href="#unit">Unit</a>)
</p>

<p>
DropIn is a drop-in configuration for a systemd unit.
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
<p>Name is the name of the drop-in.</p>
</td>
</tr>
<tr>
<td>
<code>content</code></br>
<em>
string
</em>
</td>
<td>
<p>Content is the content of the drop-in.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="extension">Extension
</h3>


<p>
Extension is a specification for a Extension resource.
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
<a href="#extensionspec">ExtensionSpec</a>
</em>
</td>
<td>
<p>Specification of the Extension.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#extensionstatus">ExtensionStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="extensionclass">ExtensionClass
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#backupbucketspec">BackupBucketSpec</a>, <a href="#backupentryspec">BackupEntrySpec</a>, <a href="#bastionspec">BastionSpec</a>, <a href="#containerruntimespec">ContainerRuntimeSpec</a>, <a href="#controlplanespec">ControlPlaneSpec</a>, <a href="#dnsrecordspec">DNSRecordSpec</a>, <a href="#defaultspec">DefaultSpec</a>, <a href="#extensionspec">ExtensionSpec</a>, <a href="#infrastructurespec">InfrastructureSpec</a>, <a href="#networkspec">NetworkSpec</a>, <a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>, <a href="#selfhostedshootexposurespec">SelfHostedShootExposureSpec</a>, <a href="#workerspec">WorkerSpec</a>)
</p>

<p>
ExtensionClass is a string alias for an extension class.
</p>


<h3 id="extensionspec">ExtensionSpec
</h3>


<p>
(<em>Appears on:</em><a href="#extension">Extension</a>)
</p>

<p>
ExtensionSpec is the spec for a Extension resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="extensionstatus">ExtensionStatus
</h3>


<p>
(<em>Appears on:</em><a href="#extension">Extension</a>)
</p>

<p>
ExtensionStatus is the status for a Extension resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="file">File
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>, <a href="#operatingsystemconfigstatus">OperatingSystemConfigStatus</a>)
</p>

<p>
File is a file that should get written to the host's file system. The content can either be inlined or
referenced from a secret in the same namespace.
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
<code>path</code></br>
<em>
string
</em>
</td>
<td>
<p>Path is the path of the file system where the file should get written to.</p>
</td>
</tr>
<tr>
<td>
<code>permissions</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Permissions describes with which permissions the file should get written to the file system.<br />If no permissions are set, the operating system's defaults are used.</p>
</td>
</tr>
<tr>
<td>
<code>content</code></br>
<em>
<a href="#filecontent">FileContent</a>
</em>
</td>
<td>
<p>Content describe the file's content.</p>
</td>
</tr>
<tr>
<td>
<code>hostName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HostName contains the name of the host for host-specific configurations.<br />If HostName is not empty the corresponding file will only be rolled out to the host with the specified name.<br />Duplicate paths are only allowed if HostName is specified for all of them, none is nil and all values differ.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="filecodecid">FileCodecID
</h3>
<p><em>Underlying type: string</em></p>


<p>
FileCodecID is the id of a FileCodec for cloud-init scripts.
</p>


<h3 id="filecontent">FileContent
</h3>


<p>
(<em>Appears on:</em><a href="#file">File</a>)
</p>

<p>
FileContent can either reference a secret or contain inline configuration.
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
<code>secretRef</code></br>
<em>
<a href="#filecontentsecretref">FileContentSecretRef</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretRef is a struct that contains information about the referenced secret.</p>
</td>
</tr>
<tr>
<td>
<code>inline</code></br>
<em>
<a href="#filecontentinline">FileContentInline</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Inline is a struct that contains information about the inlined data.</p>
</td>
</tr>
<tr>
<td>
<code>transmitUnencoded</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>TransmitUnencoded set to true will ensure that the os-extension does not encode the file content when sent to the node.<br />This for example can be used to manipulate the clear-text content before it reaches the node.</p>
</td>
</tr>
<tr>
<td>
<code>imageRef</code></br>
<em>
<a href="#filecontentimageref">FileContentImageRef</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageRef describes a container image which contains a file.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="filecontentimageref">FileContentImageRef
</h3>


<p>
(<em>Appears on:</em><a href="#filecontent">FileContent</a>)
</p>

<p>
FileContentImageRef describes a container image which contains a file
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
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image contains the container image repository with tag.</p>
</td>
</tr>
<tr>
<td>
<code>filePathInImage</code></br>
<em>
string
</em>
</td>
<td>
<p>FilePathInImage contains the path in the image to the file that should be extracted.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="filecontentinline">FileContentInline
</h3>


<p>
(<em>Appears on:</em><a href="#filecontent">FileContent</a>)
</p>

<p>
FileContentInline contains keys for inlining a file content's data and encoding.
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
<code>encoding</code></br>
<em>
string
</em>
</td>
<td>
<p>Encoding is the file's encoding (e.g. base64).</p>
</td>
</tr>
<tr>
<td>
<code>data</code></br>
<em>
string
</em>
</td>
<td>
<p>Data is the file's data.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="filecontentsecretref">FileContentSecretRef
</h3>


<p>
(<em>Appears on:</em><a href="#filecontent">FileContent</a>)
</p>

<p>
FileContentSecretRef contains keys for referencing a file content's data from a secret in the same namespace.
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
<p>Name is the name of the secret.</p>
</td>
</tr>
<tr>
<td>
<code>dataKey</code></br>
<em>
string
</em>
</td>
<td>
<p>DataKey is the key in the secret's `.data` field that should be read.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ipfamily">IPFamily
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#networkspec">NetworkSpec</a>, <a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
IPFamily is a type for specifying an IP protocol version to use in Gardener clusters.
</p>


<h3 id="inplaceupdates">InPlaceUpdates
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>)
</p>

<p>
InPlaceUpdates is a structure containing configuration for in-place updates.
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
<code>operatingSystemVersion</code></br>
<em>
string
</em>
</td>
<td>
<p>OperatingSystemVersion is the version of the operating system.</p>
</td>
</tr>
<tr>
<td>
<code>kubelet</code></br>
<em>
string
</em>
</td>
<td>
<p>KubeletVersion is the version of the kubelet.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsRotation</code></br>
<em>
<a href="#credentialsrotation">CredentialsRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CredentialsRotation is a structure containing information about the last initiation time of the certificate authority and service account key rotation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="inplaceupdatesstatus">InPlaceUpdatesStatus
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigstatus">OperatingSystemConfigStatus</a>)
</p>

<p>
InPlaceUpdatesStatus is a structure containing configuration for in-place updates.
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
<code>osUpdate</code></br>
<em>
<a href="#osupdate">OSUpdate</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OSUpdate defines the configuration for the operating system update.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="inplaceupdatesworkerstatus">InPlaceUpdatesWorkerStatus
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
InPlaceUpdatesWorkerStatus contains the configuration for in-place updates.
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
<code>workerPoolToHashMap</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkerPoolToHashMap is a map of worker pool names to their corresponding hash.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructure">Infrastructure
</h3>


<p>
Infrastructure is a specification for cloud provider infrastructure.
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
<a href="#infrastructurespec">InfrastructureSpec</a>
</em>
</td>
<td>
<p>Specification of the Infrastructure.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#infrastructurestatus">InfrastructureStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurespec">InfrastructureSpec
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructure">Infrastructure</a>)
</p>

<p>
InfrastructureSpec is the spec for an Infrastructure resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the region of this infrastructure. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the cloud provider credentials.</p>
</td>
</tr>
<tr>
<td>
<code>sshPublicKey</code></br>
<em>
integer array
</em>
</td>
<td>
<em>(Optional)</em>
<p>SSHPublicKey is the public SSH key that should be used with this infrastructure.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestatus">InfrastructureStatus
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructure">Infrastructure</a>)
</p>

<p>
InfrastructureStatus is the status for an Infrastructure resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>nodesCIDR</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodesCIDR is the CIDR of the node network that was optionally created by the acting extension controller.<br />This might be needed in environments in which the CIDR for the network for the shoot worker node cannot<br />be statically defined in the Shoot resource but must be computed dynamically.</p>
</td>
</tr>
<tr>
<td>
<code>egressCIDRs</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>EgressCIDRs is a list of CIDRs used by the shoot as the source IP for egress traffic. For certain environments the egress<br />IPs may not be stable in which case the extension controller may opt to not populate this field.</p>
</td>
</tr>
<tr>
<td>
<code>networking</code></br>
<em>
<a href="#infrastructurestatusnetworking">InfrastructureStatusNetworking</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Networking contains information about cluster networking such as CIDRs.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestatusnetworking">InfrastructureStatusNetworking
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
InfrastructureStatusNetworking is a structure containing information about the node, service and pod network ranges.
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
<code>pods</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Pods are the CIDRs of the pod network.</p>
</td>
</tr>
<tr>
<td>
<code>nodes</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes are the CIDRs of the node network.</p>
</td>
</tr>
<tr>
<td>
<code>services</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Services are the CIDRs of the service network.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinedeployment">MachineDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
MachineDeployment is a created machine deployment.
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
<p>Name is the name of the `MachineDeployment` resource.</p>
</td>
</tr>
<tr>
<td>
<code>minimum</code></br>
<em>
integer
</em>
</td>
<td>
<p>Minimum is the minimum number for this machine deployment.</p>
</td>
</tr>
<tr>
<td>
<code>maximum</code></br>
<em>
integer
</em>
</td>
<td>
<p>Maximum is the maximum number for this machine deployment.</p>
</td>
</tr>
<tr>
<td>
<code>priority</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Priority (or weight) is the importance by which this machine deployment will be scaled by cluster autoscaling.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimage">MachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#workerpool">WorkerPool</a>)
</p>

<p>
MachineImage contains logical information about the name and the version of the machie image that
should be used. The logical information must be mapped to the provider-specific information (e.g.,
AMIs, ...) by the provider itself.
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
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version of the machine image.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="network">Network
</h3>


<p>
Network is the specification for cluster networking.
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
<a href="#networkspec">NetworkSpec</a>
</em>
</td>
<td>
<p>Specification of the Network.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#networkstatus">NetworkStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="networkspec">NetworkSpec
</h3>


<p>
(<em>Appears on:</em><a href="#network">Network</a>)
</p>

<p>
NetworkSpec is the spec for an Network resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>podCIDR</code></br>
<em>
string
</em>
</td>
<td>
<p>PodCIDR defines the CIDR that will be used for pods. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>serviceCIDR</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceCIDR defines the CIDR that will be used for services. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>ipFamilies</code></br>
<em>
<a href="#ipfamily">IPFamily</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamilies specifies the IP protocol versions to use for shoot networking.<br />See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networkstatus">NetworkStatus
</h3>


<p>
(<em>Appears on:</em><a href="#network">Network</a>)
</p>

<p>
NetworkStatus is the status for an Network resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>ipFamilies</code></br>
<em>
<a href="#ipfamily">IPFamily</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamilies specifies the IP protocol versions that actually are used for shoot networking.<br />During dual-stack migration, this field may differ from the spec.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="nodetemplate">NodeTemplate
</h3>


<p>
(<em>Appears on:</em><a href="#workerpool">WorkerPool</a>)
</p>

<p>
NodeTemplate contains information about the expected node properties.
</p>


<h3 id="osupdate">OSUpdate
</h3>


<p>
(<em>Appears on:</em><a href="#inplaceupdatesstatus">InPlaceUpdatesStatus</a>)
</p>

<p>
OSUpdate contains the configuration for the operating system update.
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
<code>command</code></br>
<em>
string
</em>
</td>
<td>
<p>Command defines the command responsible for performing machine image updates.</p>
</td>
</tr>
<tr>
<td>
<code>args</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Args provides a mechanism to pass additional arguments or flags to the Command.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="object">Object
</h3>
<p><em>Underlying type: interface{GetExtensionSpec() Spec; GetExtensionStatus() Status; k8s.io/apimachinery/pkg/apis/meta/v1.Object; k8s.io/apimachinery/pkg/runtime.Object}</em></p>


<p>
Object is an extension object resource.
</p>


<h3 id="operatingsystemconfig">OperatingSystemConfig
</h3>


<p>
OperatingSystemConfig is a specification for a OperatingSystemConfig resource
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
<a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>
</em>
</td>
<td>
<p>Specification of the OperatingSystemConfig.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#operatingsystemconfigstatus">OperatingSystemConfigStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="operatingsystemconfigpurpose">OperatingSystemConfigPurpose
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>)
</p>

<p>
OperatingSystemConfigPurpose is a string alias.
</p>


<h3 id="operatingsystemconfigspec">OperatingSystemConfigSpec
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfig">OperatingSystemConfig</a>)
</p>

<p>
OperatingSystemConfigSpec is the spec for a OperatingSystemConfig resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>criConfig</code></br>
<em>
<a href="#criconfig">CRIConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CRI config is a structure contains configurations of the CRI library</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#operatingsystemconfigpurpose">OperatingSystemConfigPurpose</a>
</em>
</td>
<td>
<p>Purpose describes how the result of this OperatingSystemConfig is used by Gardener. Either it<br />gets sent to the `Worker` extension controller to bootstrap a VM, or it is downloaded by the<br />gardener-node-agent already running on a bootstrapped VM.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>units</code></br>
<em>
<a href="#unit">Unit</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Units is a list of unit for the operating system configuration (usually, a systemd unit).</p>
</td>
</tr>
<tr>
<td>
<code>files</code></br>
<em>
<a href="#file">File</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Files is a list of files that should get written to the host's file system.</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdates</code></br>
<em>
<a href="#inplaceupdates">InPlaceUpdates</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InPlaceUpdates contains the configuration for in-place updates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="operatingsystemconfigstatus">OperatingSystemConfigStatus
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfig">OperatingSystemConfig</a>)
</p>

<p>
OperatingSystemConfigStatus is the status for a OperatingSystemConfig resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>extensionUnits</code></br>
<em>
<a href="#unit">Unit</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExtensionUnits is a list of additional systemd units provided by the extension.</p>
</td>
</tr>
<tr>
<td>
<code>extensionFiles</code></br>
<em>
<a href="#file">File</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExtensionFiles is a list of additional files provided by the extension.</p>
</td>
</tr>
<tr>
<td>
<code>cloudConfig</code></br>
<em>
<a href="#cloudconfig">CloudConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudConfig is a structure for containing the generated output for the given operating system<br />config spec. It contains a reference to a secret as the result may contain confidential data.<br />After Gardener v1.112, this will be only set for OperatingSystemConfigs with purpose 'provision'.</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdates</code></br>
<em>
<a href="#inplaceupdatesstatus">InPlaceUpdatesStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InPlaceUpdates contains the configuration for in-place updates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="pluginconfig">PluginConfig
</h3>


<p>
(<em>Appears on:</em><a href="#containerdconfig">ContainerdConfig</a>)
</p>

<p>
PluginConfig contains configuration values for the containerd plugins section.
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
<code>op</code></br>
<em>
<a href="#pluginpathoperation">PluginPathOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Op is the operation for the given path. Possible values are 'add' and 'remove', defaults to 'add'.</p>
</td>
</tr>
<tr>
<td>
<code>path</code></br>
<em>
string array
</em>
</td>
<td>
<p>Path is a list of elements that construct the path in the plugins section.</p>
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#json-v1-apiextensions-k8s-io">JSON</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Values are the values configured at the given path. If defined, it is expected as json format:<br />- A given json object will be put to the given path.<br />- If not configured, only the table entry to be created.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="pluginpathoperation">PluginPathOperation
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#pluginconfig">PluginConfig</a>)
</p>

<p>
PluginPathOperation is a type alias for operations at containerd's plugin configuration.
</p>


<h3 id="registrycapability">RegistryCapability
</h3>
<p><em>Underlying type: string</em></p>


<p>
RegistryCapability specifies an action a client can perform against a registry.
</p>


<h3 id="registryconfig">RegistryConfig
</h3>


<p>
(<em>Appears on:</em><a href="#containerdconfig">ContainerdConfig</a>)
</p>

<p>
RegistryConfig contains registry configuration options.
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
<code>upstream</code></br>
<em>
string
</em>
</td>
<td>
<p>Upstream is the upstream name of the registry.</p>
</td>
</tr>
<tr>
<td>
<code>server</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Server is the URL to registry server of this upstream.<br />It corresponds to the server field in the `hosts.toml` file, see https://github.com/containerd/containerd/blob/c51463010e0682f76dfdc10edc095e6596e2764b/docs/hosts.md#server-field for more information.</p>
</td>
</tr>
<tr>
<td>
<code>hosts</code></br>
<em>
<a href="#registryhost">RegistryHost</a> array
</em>
</td>
<td>
<p>Hosts are the registry hosts.<br />It corresponds to the host fields in the `hosts.toml` file, see https://github.com/containerd/containerd/blob/c51463010e0682f76dfdc10edc095e6596e2764b/docs/hosts.md#host-fields-in-the-toml-table-format for more information.</p>
</td>
</tr>
<tr>
<td>
<code>readinessProbe</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReadinessProbe determines if host registry endpoints should be probed before they are added to the containerd config.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="registryhost">RegistryHost
</h3>


<p>
(<em>Appears on:</em><a href="#registryconfig">RegistryConfig</a>)
</p>

<p>
RegistryHost contains configuration values for a registry host.
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
<code>url</code></br>
<em>
string
</em>
</td>
<td>
<p>URL is the endpoint address of the registry mirror.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#registrycapability">RegistryCapability</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capabilities determine what operations a host is<br />capable of performing. Defaults to<br /> - pull<br /> - resolve</p>
</td>
</tr>
<tr>
<td>
<code>caCerts</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>CACerts are paths to public key certificates used for TLS.</p>
</td>
</tr>
<tr>
<td>
<code>overridePath</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>OverridePath sets the 'override_path' field to allow defining the API endpoint in the URL.<br />See https://github.com/containerd/containerd/blob/cef8ce2ecb572bc8026323c0c3dfad9953b952f6/docs/hosts.md?override_path#override_path-field for more information.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="selfhostedshootexposure">SelfHostedShootExposure
</h3>


<p>
SelfHostedShootExposure contains the configuration for the exposure of a self-hosted shoot control plane.
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
<a href="#selfhostedshootexposurespec">SelfHostedShootExposureSpec</a>
</em>
</td>
<td>
<p>Specification of the SelfHostedShootExposure.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#selfhostedshootexposurestatus">SelfHostedShootExposureStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="selfhostedshootexposurespec">SelfHostedShootExposureSpec
</h3>


<p>
(<em>Appears on:</em><a href="#selfhostedshootexposure">SelfHostedShootExposure</a>)
</p>

<p>
SelfHostedShootExposureSpec is the spec for an SelfHostedShootExposure resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CredentialsRef is a reference to the cloud provider credentials.<br />It is only set for shoots with managed infrastructure (i.e., if `Shoot.spec.\{credentials,secret\}BindingName` is set).</p>
</td>
</tr>
<tr>
<td>
<code>port</code></br>
<em>
integer
</em>
</td>
<td>
<p>Port is the port number that should be exposed by the exposure mechanism.<br />It is the port where the API server listens on the control plane nodes and the port on which the load balancer (or<br />any other exposure mechanism) should listen on.</p>
</td>
</tr>
<tr>
<td>
<code>endpoints</code></br>
<em>
<a href="#controlplaneendpoint">ControlPlaneEndpoint</a> array
</em>
</td>
<td>
<p>Endpoints contains a list of healthy control plane nodes to expose.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="selfhostedshootexposurestatus">SelfHostedShootExposureStatus
</h3>


<p>
(<em>Appears on:</em><a href="#selfhostedshootexposure">SelfHostedShootExposure</a>)
</p>

<p>
SelfHostedShootExposureStatus is the status for an SelfHostedShootExposure resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#loadbalanceringress-v1-core">LoadBalancerIngress</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ingress is a list of endpoints of the exposure mechanism.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="serviceaccountkeyrotation">ServiceAccountKeyRotation
</h3>


<p>
(<em>Appears on:</em><a href="#credentialsrotation">CredentialsRotation</a>)
</p>

<p>
ServiceAccountKeyRotation contains information about the service account key credential rotation.
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
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationTime is the most recent time when the service account key credential rotation was initiated.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="spec">Spec
</h3>
<p><em>Underlying type: interface{GetExtensionClass() *ExtensionClass; GetExtensionPurpose() *string; GetExtensionType() string; GetProviderConfig() *k8s.io/apimachinery/pkg/runtime.RawExtension}</em></p>


<p>
Spec is the spec section of an Object.
</p>


<h3 id="status">Status
</h3>
<p><em>Underlying type: interface{GetConditions() []github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition; GetLastError() *github.com/gardener/gardener/pkg/apis/core/v1beta1.LastError; GetLastOperation() *github.com/gardener/gardener/pkg/apis/core/v1beta1.LastOperation; GetObservedGeneration() int64; GetProviderStatus() *k8s.io/apimachinery/pkg/runtime.RawExtension; GetResources() []github.com/gardener/gardener/pkg/apis/core/v1beta1.NamedResourceReference; GetState() *k8s.io/apimachinery/pkg/runtime.RawExtension; SetConditions([]github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition); SetLastError(*github.com/gardener/gardener/pkg/apis/core/v1beta1.LastError); SetLastOperation(*github.com/gardener/gardener/pkg/apis/core/v1beta1.LastOperation); SetObservedGeneration(int64); SetResources(namedResourceReferences []github.com/gardener/gardener/pkg/apis/core/v1beta1.NamedResourceReference); SetState(state *k8s.io/apimachinery/pkg/runtime.RawExtension)}</em></p>


<p>
Status is the status of an Object.
</p>


<h3 id="unit">Unit
</h3>


<p>
(<em>Appears on:</em><a href="#operatingsystemconfigspec">OperatingSystemConfigSpec</a>, <a href="#operatingsystemconfigstatus">OperatingSystemConfigStatus</a>)
</p>

<p>
Unit is a unit for the operating system configuration (usually, a systemd unit).
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
<p>Name is the name of a unit.</p>
</td>
</tr>
<tr>
<td>
<code>command</code></br>
<em>
<a href="#unitcommand">UnitCommand</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Command is the unit's command.</p>
</td>
</tr>
<tr>
<td>
<code>enable</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enable describes whether the unit is enabled or not.</p>
</td>
</tr>
<tr>
<td>
<code>content</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Content is the unit's content.</p>
</td>
</tr>
<tr>
<td>
<code>dropIns</code></br>
<em>
<a href="#dropin">DropIn</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DropIns is a list of drop-ins for this unit.</p>
</td>
</tr>
<tr>
<td>
<code>filePaths</code></br>
<em>
string array
</em>
</td>
<td>
<p>FilePaths is a list of files the unit depends on. If any file changes a restart of the dependent unit will be<br />triggered. For each FilePath there must exist a File with matching Path in OperatingSystemConfig.Spec.Files.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="unitcommand">UnitCommand
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#unit">Unit</a>)
</p>

<p>
UnitCommand is a string alias.
</p>


<h3 id="volume">Volume
</h3>


<p>
(<em>Appears on:</em><a href="#workerpool">WorkerPool</a>)
</p>

<p>
Volume contains information about the root disks that should be used for worker pools.
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
<em>(Optional)</em>
<p>Name of the volume to make it referenceable.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type is the type of the volume.</p>
</td>
</tr>
<tr>
<td>
<code>size</code></br>
<em>
string
</em>
</td>
<td>
<p>Size is the of the root volume.</p>
</td>
</tr>
<tr>
<td>
<code>encrypted</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Encrypted determines if the volume should be encrypted.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="worker">Worker
</h3>


<p>
Worker is a specification for a Worker resource.
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
<a href="#workerspec">WorkerSpec</a>
</em>
</td>
<td>
<p>Specification of the Worker.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#workerstatus">WorkerStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerpool">WorkerPool
</h3>


<p>
(<em>Appears on:</em><a href="#workerspec">WorkerSpec</a>)
</p>

<p>
WorkerPool is the definition of a specific worker pool.
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
<code>machineType</code></br>
<em>
string
</em>
</td>
<td>
<p>MachineType contains information about the machine type that should be used for this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>maximum</code></br>
<em>
integer
</em>
</td>
<td>
<p>Maximum is the maximum size of the worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>maxSurge</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<p>MaxSurge is maximum number of VMs that are created during an update.</p>
</td>
</tr>
<tr>
<td>
<code>maxUnavailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#intorstring-intstr-util">IntOrString</a>
</em>
</td>
<td>
<p>MaxUnavailable is the maximum number of VMs that can be unavailable during an update.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations is a map of key/value pairs for annotations for all the `Node` objects in this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels is a map of key/value pairs for labels for all the `Node` objects in this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>taints</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#taint-v1-core">Taint</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Taints is a list of taints for all the `Node` objects in this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>machineImage</code></br>
<em>
<a href="#machineimage">MachineImage</a>
</em>
</td>
<td>
<p>MachineImage contains logical information about the name and the version of the machie image that<br />should be used. The logical information must be mapped to the provider-specific information (e.g.,<br />AMIs, ...) by the provider itself.</p>
</td>
</tr>
<tr>
<td>
<code>minimum</code></br>
<em>
integer
</em>
</td>
<td>
<p>Minimum is the minimum size of the worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>nodeAgentSecretName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeAgentSecretName is uniquely identifying selected aspects of the OperatingSystemConfig. If it changes, then the<br />worker pool must be rolled.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is a provider specific configuration for the worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>userDataSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core">SecretKeySelector</a>
</em>
</td>
<td>
<p>UserDataSecretRef references a Secret and a data key containing the data that is sent to the provider's APIs when<br />a new machine/VM that is part of this worker pool shall be spawned.</p>
</td>
</tr>
<tr>
<td>
<code>volume</code></br>
<em>
<a href="#volume">Volume</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains information about the root disks that should be used for this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumes</code></br>
<em>
<a href="#datavolume">DataVolume</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DataVolumes contains a list of additional worker volumes.</p>
</td>
</tr>
<tr>
<td>
<code>kubeletDataVolumeName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeletDataVolumeName contains the name of a dataVolume that should be used for storing kubelet state.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones contains information about availability zones for this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>machineControllerManager</code></br>
<em>
<a href="#machinecontrollermanagersettings">MachineControllerManagerSettings</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineControllerManagerSettings contains configurations for different worker-pools. Eg. MachineDrainTimeout, MachineHealthTimeout.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetesVersion</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubernetesVersion is the kubernetes version in this worker pool</p>
</td>
</tr>
<tr>
<td>
<code>kubeletConfig</code></br>
<em>
<a href="#kubeletconfig">KubeletConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeletConfig contains the kubelet configuration for the worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>nodeTemplate</code></br>
<em>
<a href="#nodetemplate">NodeTemplate</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the worker pool machines and machine image.</p>
</td>
</tr>
<tr>
<td>
<code>clusterAutoscaler</code></br>
<em>
<a href="#clusterautoscaleroptions">ClusterAutoscalerOptions</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterAutoscaler contains the cluster autoscaler configurations for the worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>priority</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Priority (or weight) is the importance by which this worker pool will be scaled by cluster autoscaling.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#machineupdatestrategy">MachineUpdateStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdateStrategy specifies the machine update strategy for the worker pool.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerspec">WorkerSpec
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
WorkerSpec is the spec for a Worker resource.
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
string
</em>
</td>
<td>
<p>Type contains the instance of the resource's kind.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
<a href="#extensionclass">ExtensionClass</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the extension class used to control the responsibility for multiple provider extensions.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>infrastructureProviderStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InfrastructureProviderStatus is a raw extension field that contains the provider status that has<br />been generated by the controller responsible for the `Infrastructure` resource.</p>
</td>
</tr>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the name of the region where the worker pool should be deployed to. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the cloud provider specific credentials.</p>
</td>
</tr>
<tr>
<td>
<code>sshPublicKey</code></br>
<em>
integer array
</em>
</td>
<td>
<em>(Optional)</em>
<p>SSHPublicKey is the public SSH key that should be used with these workers.</p>
</td>
</tr>
<tr>
<td>
<code>pools</code></br>
<em>
<a href="#workerpool">WorkerPool</a> array
</em>
</td>
<td>
<p>Pools is a list of worker pools.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerstatus">WorkerStatus
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
WorkerStatus is the status for a Worker resource.
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
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains provider-specific status.</p>
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
<p>Conditions represents the latest available observations of a Seed's current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastError</code></br>
<em>
<a href="#lasterror">LastError</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastError holds information about the last occurred error during an operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the resource.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>State can be filled by the operating controller with what ever data it needs.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
NamedResourceReference array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
<tr>
<td>
<code>machineDeployments</code></br>
<em>
<a href="#machinedeployment">MachineDeployment</a> array
</em>
</td>
<td>
<p>MachineDeployments is a list of created machine deployments. It will be used to e.g. configure<br />the cluster-autoscaler properly.</p>
</td>
</tr>
<tr>
<td>
<code>machineDeploymentsLastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineDeploymentsLastUpdateTime is the timestamp when the status.MachineDeployments slice was last updated.</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdates</code></br>
<em>
<a href="#inplaceupdatesworkerstatus">InPlaceUpdatesWorkerStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InPlaceUpdates contains the status for in-place updates.</p>
</td>
</tr>

</tbody>
</table>


