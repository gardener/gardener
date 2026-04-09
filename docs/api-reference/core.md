<p>Packages:</p>
<ul>
<li>
<a href="#core.gardener.cloud%2fv1beta1">core.gardener.cloud/v1beta1</a>
</li>
</ul>

<h2 id="core.gardener.cloud/v1beta1">core.gardener.cloud/v1beta1</h2>
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
<a href="#cloudprofile">CloudProfile</a>
</li>
<li>
<a href="#controllerdeployment">ControllerDeployment</a>
</li>
<li>
<a href="#controllerinstallation">ControllerInstallation</a>
</li>
<li>
<a href="#controllerregistration">ControllerRegistration</a>
</li>
<li>
<a href="#exposureclass">ExposureClass</a>
</li>
<li>
<a href="#internalsecret">InternalSecret</a>
</li>
<li>
<a href="#namespacedcloudprofile">NamespacedCloudProfile</a>
</li>
<li>
<a href="#project">Project</a>
</li>
<li>
<a href="#quota">Quota</a>
</li>
<li>
<a href="#secretbinding">SecretBinding</a>
</li>
<li>
<a href="#seed">Seed</a>
</li>
<li>
<a href="#seedtemplate">SeedTemplate</a>
</li>
<li>
<a href="#shoot">Shoot</a>
</li>
<li>
<a href="#shootstate">ShootState</a>
</li>
<li>
<a href="#shoottemplate">ShootTemplate</a>
</li>
</ul>

<h3 id="apiserverlogging">APIServerLogging
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
APIServerLogging contains configuration for the logs level and http access logs
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
<code>verbosity</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbosity is the kube-apiserver log verbosity level<br />Defaults to 2.</p>
</td>
</tr>
<tr>
<td>
<code>httpAccessVerbosity</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTPAccessVerbosity is the kube-apiserver access logs level</p>
</td>
</tr>

</tbody>
</table>


<h3 id="apiserverrequests">APIServerRequests
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
APIServerRequests contains configuration for request-specific settings for the kube-apiserver.
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
<code>maxNonMutatingInflight</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNonMutatingInflight is the maximum number of non-mutating requests in flight at a given time. When the server<br />exceeds this, it rejects requests.</p>
</td>
</tr>
<tr>
<td>
<code>maxMutatingInflight</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxMutatingInflight is the maximum number of mutating requests in flight at a given time. When the server<br />exceeds this, it rejects requests.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="accessrestriction">AccessRestriction
</h3>


<p>
(<em>Appears on:</em><a href="#accessrestrictionwithoptions">AccessRestrictionWithOptions</a>, <a href="#region">Region</a>, <a href="#seedspec">SeedSpec</a>)
</p>

<p>
AccessRestriction describes an access restriction for a Kubernetes cluster (e.g., EU access-only).
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
<p>Name is the name of the restriction.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="accessrestrictionwithoptions">AccessRestrictionWithOptions
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
AccessRestrictionWithOptions describes an access restriction for a Kubernetes cluster (e.g., EU access-only) and
allows to specify additional options.
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
<p>Name is the name of the restriction.</p>
</td>
</tr>
<tr>
<td>
<code>options</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Options is a map of additional options for the access restriction.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="addon">Addon
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetesdashboard">KubernetesDashboard</a>, <a href="#nginxingress">NginxIngress</a>)
</p>

<p>
Addon allows enabling or disabling a specific addon and is used to derive from.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled indicates whether the addon is enabled or not.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="addons">Addons
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Addons is a collection of configuration for specific addons which are managed by the Gardener.
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
<code>kubernetesDashboard</code></br>
<em>
<a href="#kubernetesdashboard">KubernetesDashboard</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubernetesDashboard holds configuration settings for the kubernetes dashboard addon.</p>
</td>
</tr>
<tr>
<td>
<code>nginxIngress</code></br>
<em>
<a href="#nginxingress">NginxIngress</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NginxIngress holds configuration settings for the nginx-ingress addon.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="admissionplugin">AdmissionPlugin
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.
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
<p>Name is the name of the plugin.</p>
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
<p>Config is the configuration of the plugin.</p>
</td>
</tr>
<tr>
<td>
<code>disabled</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Disabled specifies whether this plugin should be disabled.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfigSecretName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this admission plugin.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="alerting">Alerting
</h3>


<p>
(<em>Appears on:</em><a href="#monitoring">Monitoring</a>)
</p>

<p>
Alerting contains information about how alerting will be done (i.e. who will receive alerts and how).
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
<code>emailReceivers</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MonitoringEmailReceivers is a list of recipients for alerts</p>
</td>
</tr>

</tbody>
</table>


<h3 id="auditconfig">AuditConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
AuditConfig contains settings for audit of the api server
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
<code>auditPolicy</code></br>
<em>
<a href="#auditpolicy">AuditPolicy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuditPolicy contains configuration settings for audit policy of the kube-apiserver.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="auditpolicy">AuditPolicy
</h3>


<p>
(<em>Appears on:</em><a href="#auditconfig">AuditConfig</a>)
</p>

<p>
AuditPolicy contains audit policy for kube-apiserver
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
<code>configMapRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfigMapRef is a reference to a ConfigMap object in the same namespace,<br />which contains the audit policy for the kube-apiserver.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="authorizerkubeconfigreference">AuthorizerKubeconfigReference
</h3>


<p>
(<em>Appears on:</em><a href="#structuredauthorization">StructuredAuthorization</a>)
</p>

<p>
AuthorizerKubeconfigReference is a reference for a kubeconfig for a authorization webhook.
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
<code>authorizerName</code></br>
<em>
string
</em>
</td>
<td>
<p>AuthorizerName is the name of a webhook authorizer.</p>
</td>
</tr>
<tr>
<td>
<code>secretName</code></br>
<em>
string
</em>
</td>
<td>
<p>SecretName is the name of a secret containing the kubeconfig.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="availabilityzone">AvailabilityZone
</h3>


<p>
(<em>Appears on:</em><a href="#region">Region</a>)
</p>

<p>
AvailabilityZone is an availability zone.
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
<p>Name is an availability zone name.</p>
</td>
</tr>
<tr>
<td>
<code>unavailableMachineTypes</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>UnavailableMachineTypes is a list of machine type names that are not availability in this zone.</p>
</td>
</tr>
<tr>
<td>
<code>unavailableVolumeTypes</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>UnavailableVolumeTypes is a list of volume type names that are not availability in this zone.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="backup">Backup
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>, <a href="#workercontrolplane">WorkerControlPlane</a>)
</p>

<p>
Backup contains the object store configuration for backups for shoot (currently only etcd).
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
<code>provider</code></br>
<em>
string
</em>
</td>
<td>
<p>Provider is a provider name. This field is immutable.</p>
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
<p>ProviderConfig is the configuration passed to BackupBucket resource.</p>
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
<p>Region is a region name. This field is immutable.</p>
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
<p>CredentialsRef is reference to a resource holding the credentials used for<br />authentication with the object store service where the backups are stored.<br />Supported referenced resources are v1.Secrets and<br />security.gardener.cloud/v1alpha1.WorkloadIdentity</p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupbucket">BackupBucket
</h3>


<p>
BackupBucket holds details about backup bucket
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
<p>Specification of the Backup Bucket.</p>
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
<p>Most recently observed status of the Backup Bucket.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="backupbucketprovider">BackupBucketProvider
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketspec">BackupBucketSpec</a>)
</p>

<p>
BackupBucketProvider holds the details of cloud provider of the object store.
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
<p>Type is the type of provider.</p>
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
<p>Region is the region of the bucket.</p>
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
BackupBucketSpec is the specification of a Backup Bucket.
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
<code>provider</code></br>
<em>
<a href="#backupbucketprovider">BackupBucketProvider</a>
</em>
</td>
<td>
<p>Provider holds the details of cloud provider of the object store. This field is immutable.</p>
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
<p>ProviderConfig is the configuration passed to BackupBucket resource.</p>
</td>
</tr>
<tr>
<td>
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the Seed this BackupBucket is associated with. Mutually exclusive with ShootRef.<br />This field is immutable.</p>
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
<p>CredentialsRef is reference to a resource holding the credentials used for<br />authentication with the object store service where the backups are stored.<br />Supported referenced resources are v1.Secrets and<br />security.gardener.cloud/v1alpha1.WorkloadIdentity</p>
</td>
</tr>
<tr>
<td>
<code>shootRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootRef is the reference of the Shoot this BackupBucket is associated with. Mutually exclusive with SeedName.<br />This field is immutable.</p>
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
BackupBucketStatus holds the most recently observed status of the Backup Bucket.
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
<p>ProviderStatus is the configuration passed to BackupBucket resource.</p>
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
<p>LastOperation holds information about the last operation on the BackupBucket.</p>
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
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this BackupBucket. It corresponds to the<br />BackupBucket's generation, which is updated on mutation by the API Server.</p>
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
BackupEntry holds details about shoot backup.
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
<em>(Optional)</em>
<p>Spec contains the specification of the Backup Entry.</p>
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
<p>Status contains the most recently observed status of the Backup Entry.</p>
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
BackupEntrySpec is the specification of a Backup Entry.
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
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the Seed this BackupEntry is associated with. Mutually exclusive with ShootRef.</p>
</td>
</tr>
<tr>
<td>
<code>shootRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootRef is the reference of the Shoot this BackupBucket is associated with. Mutually exclusive with SeedName.<br />This field is immutable.</p>
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
BackupEntryStatus holds the most recently observed status of the Backup Entry.
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
<code>lastOperation</code></br>
<em>
<a href="#lastoperation">LastOperation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the BackupEntry.</p>
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
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this BackupEntry. It corresponds to the<br />BackupEntry's generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the seed to which this BackupEntry is currently scheduled. This field is populated<br />at the beginning of a create/reconcile operation. It is used when moving the BackupEntry between seeds.</p>
</td>
</tr>
<tr>
<td>
<code>migrationStartTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MigrationStartTime is the time when a migration to a different seed was initiated.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastion">Bastion
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>)
</p>

<p>
Bastion contains the bastions creation info
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
<code>machineImage</code></br>
<em>
<a href="#bastionmachineimage">BastionMachineImage</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImage contains the bastions machine image properties</p>
</td>
</tr>
<tr>
<td>
<code>machineType</code></br>
<em>
<a href="#bastionmachinetype">BastionMachineType</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineType contains the bastions machine type properties</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastionmachineimage">BastionMachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#bastion">Bastion</a>)
</p>

<p>
BastionMachineImage contains the bastions machine image properties
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
<p>Name of the machine image</p>
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
<em>(Optional)</em>
<p>Version of the machine image</p>
</td>
</tr>

</tbody>
</table>


<h3 id="bastionmachinetype">BastionMachineType
</h3>


<p>
(<em>Appears on:</em><a href="#bastion">Bastion</a>)
</p>

<p>
BastionMachineType contains the bastions machine type properties
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
<p>Name of the machine type</p>
</td>
</tr>

</tbody>
</table>


<h3 id="carotation">CARotation
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentialsrotation">ShootCredentialsRotation</a>)
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
<code>phase</code></br>
<em>
<a href="#credentialsrotationphase">CredentialsRotationPhase</a>
</em>
</td>
<td>
<p>Phase describes the phase of the certificate authority credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the certificate authority credential rotation was successfully<br />completed.</p>
</td>
</tr>
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
<tr>
<td>
<code>lastInitiationFinishedTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationFinishedTime is the recent time when the certificate authority credential rotation initiation was<br />completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTriggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTriggeredTime is the recent time when the certificate authority credential rotation completion was<br />triggered.</p>
</td>
</tr>
<tr>
<td>
<code>pendingWorkersRollouts</code></br>
<em>
<a href="#pendingworkersrollout">PendingWorkersRollout</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkersRollouts contains the name of a worker pool and the initiation time of their last rollout due to<br />credentials rotation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cri">CRI
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>, <a href="#worker">Worker</a>)
</p>

<p>
CRI contains information about the Container Runtimes.
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
<p>The name of the CRI library. Supported values are `containerd`.</p>
</td>
</tr>
<tr>
<td>
<code>containerRuntimes</code></br>
<em>
<a href="#containerruntime">ContainerRuntime</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContainerRuntimes is the list of the required container runtimes supported for a worker pool.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="criname">CRIName
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#cri">CRI</a>)
</p>

<p>
CRIName is a type alias for the CRI name string.
</p>


<h3 id="capabilities">Capabilities
</h3>
<p><em>Underlying type: object (keys:string, values:<a href="#capabilityvalues">CapabilityValues</a>)</em></p>


<p>
(<em>Appears on:</em><a href="#machinetype">MachineType</a>)
</p>

<p>
Capabilities of a machine type or machine image.
</p>


<h3 id="capabilitydefinition">CapabilityDefinition
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>)
</p>

<p>
CapabilityDefinition contains the Name and Values of a capability.
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
<p></p>
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
<a href="#capabilityvalues">CapabilityValues</a>
</em>
</td>
<td>
<p></p>
</td>
</tr>

</tbody>
</table>


<h3 id="capabilityvalues">CapabilityValues
</h3>
<p><em>Underlying type: string array</em></p>


<p>
(<em>Appears on:</em><a href="#capabilities">Capabilities</a>, <a href="#capabilitydefinition">CapabilityDefinition</a>)
</p>

<p>
CapabilityValues contains capability values.
This is a workaround as the Protobuf generator can't handle a map with slice values.
</p>


<h3 id="cloudprofile">CloudProfile
</h3>


<p>
CloudProfile represents certain properties about a provider environment.
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
<a href="#cloudprofilespec">CloudProfileSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the provider environment properties.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#cloudprofilestatus">CloudProfileStatus</a>
</em>
</td>
<td>
<p>Status contains the current status of the cloud profile.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofilemachinecontrollermanagersettings">CloudProfileMachineControllerManagerSettings
</h3>


<p>
(<em>Appears on:</em><a href="#machinetype">MachineType</a>)
</p>

<p>
CloudProfileMachineControllerManagerSettings contains a subset of the MachineControllerManagerSettings which can be defaulted for a machine type in a CloudProfile.
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
<code>machineCreationTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineCreationTimeout is the period after which creation of a machine of this machine type is declared failed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofilereference">CloudProfileReference
</h3>


<p>
(<em>Appears on:</em><a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>, <a href="#shootspec">ShootSpec</a>)
</p>

<p>
CloudProfileReference holds the information about a CloudProfile or a NamespacedCloudProfile.
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
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind contains a CloudProfile kind.</p>
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
<p>Name contains the name of the referenced CloudProfile.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofilespec">CloudProfileSpec
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofile">CloudProfile</a>, <a href="#namespacedcloudprofilestatus">NamespacedCloudProfileStatus</a>)
</p>

<p>
CloudProfileSpec is the specification of a CloudProfile.
It must contain exactly one of its defined keys.
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
<code>caBundle</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#kubernetessettings">KubernetesSettings</a>
</em>
</td>
<td>
<p>Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#machineimage">MachineImage</a> array
</em>
</td>
<td>
<p>MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineTypes</code></br>
<em>
<a href="#machinetype">MachineType</a> array
</em>
</td>
<td>
<p>MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.</p>
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
<p>ProviderConfig contains provider-specific configuration for the profile.</p>
</td>
</tr>
<tr>
<td>
<code>regions</code></br>
<em>
<a href="#region">Region</a> array
</em>
</td>
<td>
<p>Regions contains constraints regarding allowed values for regions and zones.</p>
</td>
</tr>
<tr>
<td>
<code>seedSelector</code></br>
<em>
<a href="#seedselector">SeedSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector contains an optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.<br />An empty list means that all seeds of the same provider type are supported.<br />This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.<br />Optionally a list of possible providers can be added to enable cross-provider scheduling. By default, the provider<br />type of the seed must match the shoot's provider.</p>
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
<p>Type is the name of the provider.</p>
</td>
</tr>
<tr>
<td>
<code>volumeTypes</code></br>
<em>
<a href="#volumetype">VolumeType</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>bastion</code></br>
<em>
<a href="#bastion">Bastion</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bastion contains the machine and image properties</p>
</td>
</tr>
<tr>
<td>
<code>limits</code></br>
<em>
<a href="#limits">Limits</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits configures operational limits for Shoot clusters using this CloudProfile.<br />See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.</p>
</td>
</tr>
<tr>
<td>
<code>machineCapabilities</code></br>
<em>
<a href="#capabilitydefinition">CapabilityDefinition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineCapabilities contains the definition of all possible capabilities in the CloudProfile.<br />Only capabilities and values defined here can be used to describe MachineImages and MachineTypes.<br />The order of values for a given capability is relevant. The most important value is listed first.<br />During maintenance upgrades, the image that matches most capabilities will be selected.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofilestatus">CloudProfileStatus
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofile">CloudProfile</a>)
</p>

<p>
CloudProfileStatus contains the status of the cloud profile.
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
<code>kubernetes</code></br>
<em>
<a href="#kubernetesstatus">KubernetesStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubernetes contains the status information for kubernetes.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#machineimagestatus">MachineImageStatus</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages contains the statuses of the machine image versions.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="clusterautoscaler">ClusterAutoscaler
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
ClusterAutoscaler contains the configuration flags for the Kubernetes cluster autoscaler.
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
<code>scaleDownDelayAfterAdd</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownDelayAfterAdd defines how long after scale up that scale down evaluation resumes (default: 1 hour).</p>
</td>
</tr>
<tr>
<td>
<code>scaleDownDelayAfterDelete</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownDelayAfterDelete how long after node deletion that scale down evaluation resumes, defaults to scanInterval (default: 0 secs).</p>
</td>
</tr>
<tr>
<td>
<code>scaleDownDelayAfterFailure</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownDelayAfterFailure how long after scale down failure that scale down evaluation resumes (default: 3 mins).</p>
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
<p>ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down (default: 30 mins).</p>
</td>
</tr>
<tr>
<td>
<code>scaleDownUtilizationThreshold</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownUtilizationThreshold defines the threshold in fraction (0.0 - 1.0) under which a node is being removed (default: 0.5).</p>
</td>
</tr>
<tr>
<td>
<code>scanInterval</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScanInterval how often cluster is reevaluated for scale up or down (default: 10 secs).</p>
</td>
</tr>
<tr>
<td>
<code>expander</code></br>
<em>
<a href="#expandermode">ExpanderMode</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Expander defines the algorithm to use during scale up (default: least-waste).<br />See: https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#what-are-expanders.</p>
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
<p>MaxNodeProvisionTime defines how long CA waits for node to be provisioned (default: 20 mins).</p>
</td>
</tr>
<tr>
<td>
<code>maxGracefulTerminationSeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxGracefulTerminationSeconds is the number of seconds CA waits for pod termination when trying to scale down a node (default: 600).</p>
</td>
</tr>
<tr>
<td>
<code>ignoreTaints</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>IgnoreTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.<br />Deprecated: Ignore taints are deprecated and treated as startup taints</p>
</td>
</tr>
<tr>
<td>
<code>newPodScaleUpDelay</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NewPodScaleUpDelay specifies how long CA should ignore newly created pods before they have to be considered for scale-up (default: 0s).</p>
</td>
</tr>
<tr>
<td>
<code>maxEmptyBulkDelete</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxEmptyBulkDelete specifies the maximum number of empty nodes that can be deleted at the same time (default: MaxScaleDownParallelism when that is set).<br />Deprecated: This field is deprecated. Setting this field will be forbidden starting from Kubernetes 1.33 and will be removed once gardener drops support for kubernetes v1.32.<br />This cluster-autoscaler field is deprecated upstream, use --max-scale-down-parallelism instead.</p>
</td>
</tr>
<tr>
<td>
<code>ignoreDaemonsetsUtilization</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>IgnoreDaemonsetsUtilization allows CA to ignore DaemonSet pods when calculating resource utilization for scaling down (default: false).</p>
</td>
</tr>
<tr>
<td>
<code>verbosity</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbosity allows CA to modify its log level (default: 2).</p>
</td>
</tr>
<tr>
<td>
<code>startupTaints</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartupTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.<br />Cluster Autoscaler treats nodes tainted with startup taints as unready, but taken into account during scale up logic, assuming they will become ready shortly.</p>
</td>
</tr>
<tr>
<td>
<code>statusTaints</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>StatusTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.<br />Cluster Autoscaler internally treats nodes tainted with status taints as ready, but filtered out during scale up logic.</p>
</td>
</tr>
<tr>
<td>
<code>maxScaleDownParallelism</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxScaleDownParallelism specifies the maximum number of nodes (both empty and needing drain) that can be deleted in parallel.<br />Default: 10 or MaxEmptyBulkDelete when that is set</p>
</td>
</tr>
<tr>
<td>
<code>maxDrainParallelism</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxDrainParallelism specifies the maximum number of nodes needing drain, that can be drained and deleted in parallel.<br />Default: 1</p>
</td>
</tr>
<tr>
<td>
<code>initialNodeGroupBackoffDuration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InitialNodeGroupBackoffDuration is the duration of first backoff after a new node failed to start (default: 5m).</p>
</td>
</tr>
<tr>
<td>
<code>maxNodeGroupBackoffDuration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNodeGroupBackoffDuration is the maximum backoff duration for a NodeGroup after new nodes failed to start (default: 30m).</p>
</td>
</tr>
<tr>
<td>
<code>nodeGroupBackoffResetTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeGroupBackoffResetTimeout is the time after last failed scale-up when the backoff duration is reset (default: 3h).</p>
</td>
</tr>
<tr>
<td>
<code>emitPerNodeGroupMetrics</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>EmitPerNodeGroupMetrics emits additional per node group metrics</p>
</td>
</tr>

</tbody>
</table>


<h3 id="clusterautoscaleroptions">ClusterAutoscalerOptions
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
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
float
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
float
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
<p>MaxNodeProvisionTime defines how long CA waits for node to be provisioned.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="clustertype">ClusterType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#controllerresource">ControllerResource</a>)
</p>

<p>
ClusterType defines the type of cluster.
</p>


<h3 id="condition">Condition
</h3>


<p>
(<em>Appears on:</em><a href="#controllerinstallationstatus">ControllerInstallationStatus</a>, <a href="#projectstatus">ProjectStatus</a>, <a href="#seedstatus">SeedStatus</a>, <a href="#shootstatus">ShootStatus</a>)
</p>

<p>
Condition holds the information about the state of a resource.
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
<a href="#conditiontype">ConditionType</a>
</em>
</td>
<td>
<p>Type of the condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#conditionstatus">ConditionStatus</a>
</em>
</td>
<td>
<p>Status of the condition, one of True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>Last time the condition transitioned from one status to another.</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>Last time the condition was updated.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code></br>
<em>
string
</em>
</td>
<td>
<p>The reason for the condition's last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code></br>
<em>
string
</em>
</td>
<td>
<p>A human readable message indicating details about the transition.</p>
</td>
</tr>
<tr>
<td>
<code>codes</code></br>
<em>
<a href="#errorcode">ErrorCode</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Well-defined error codes in case the condition reports a problem.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="conditionstatus">ConditionStatus
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#condition">Condition</a>)
</p>

<p>
ConditionStatus is the status of a condition.
</p>


<h3 id="conditiontype">ConditionType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#condition">Condition</a>)
</p>

<p>
ConditionType is a string alias.
</p>


<h3 id="containerruntime">ContainerRuntime
</h3>


<p>
(<em>Appears on:</em><a href="#cri">CRI</a>)
</p>

<p>
ContainerRuntime contains information about worker's available container runtime
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
<p>Type is the type of the Container Runtime.</p>
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
<p>ProviderConfig is the configuration passed to container runtime resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplane">ControlPlane
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
ControlPlane holds information about the general settings for the control plane of a shoot.
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
<code>highAvailability</code></br>
<em>
<a href="#highavailability">HighAvailability</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HighAvailability holds the configuration settings for high availability of the<br />control plane of a shoot.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplaneautoscaling">ControlPlaneAutoscaling
</h3>


<p>
(<em>Appears on:</em><a href="#etcdconfig">ETCDConfig</a>, <a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
ControlPlaneAutoscaling contains auto-scaling configuration options for control-plane components.
</p>


<h3 id="controllerdeployment">ControllerDeployment
</h3>


<p>
ControllerDeployment contains information about how this controller is deployed.
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
<code>type</code></br>
<em>
string
</em>
</td>
<td>
<p>Type is the deployment type.</p>
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
<p>ProviderConfig contains type-specific configuration. It contains assets that deploy the controller.</p>
</td>
</tr>
<tr>
<td>
<code>injectGardenKubeconfig</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>InjectGardenKubeconfig controls whether a kubeconfig to the garden cluster should be injected into workload<br />resources.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerdeploymentpolicy">ControllerDeploymentPolicy
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#controllerregistrationdeployment">ControllerRegistrationDeployment</a>)
</p>

<p>
ControllerDeploymentPolicy is a string alias.
</p>


<h3 id="controllerinstallation">ControllerInstallation
</h3>


<p>
ControllerInstallation represents an installation request for an external controller.
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
<a href="#controllerinstallationspec">ControllerInstallationSpec</a>
</em>
</td>
<td>
<p>Spec contains the specification of this installation.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#controllerinstallationstatus">ControllerInstallationStatus</a>
</em>
</td>
<td>
<p>Status contains the status of this installation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerinstallationspec">ControllerInstallationSpec
</h3>


<p>
(<em>Appears on:</em><a href="#controllerinstallation">ControllerInstallation</a>)
</p>

<p>
ControllerInstallationSpec is the specification of a ControllerInstallation.
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
<code>registrationRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<p>RegistrationRef is used to reference a ControllerRegistration resource.<br />The name field of the RegistrationRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>seedRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedRef is used to reference a Seed resource. The name field of the SeedRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>shootRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootRef is used to reference a Shoot resource. The name and namespace fields of the ShootRef are immutable.</p>
</td>
</tr>
<tr>
<td>
<code>deploymentRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeploymentRef is used to reference a ControllerDeployment resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerinstallationstatus">ControllerInstallationStatus
</h3>


<p>
(<em>Appears on:</em><a href="#controllerinstallation">ControllerInstallation</a>)
</p>

<p>
ControllerInstallationStatus is the status of a ControllerInstallation.
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
<a href="#condition">Condition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a ControllerInstallations's current state.</p>
</td>
</tr>
<tr>
<td>
<code>providerStatus</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains type-specific status.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerregistration">ControllerRegistration
</h3>


<p>
ControllerRegistration represents a registration of an external controller.
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
<a href="#controllerregistrationspec">ControllerRegistrationSpec</a>
</em>
</td>
<td>
<p>Spec contains the specification of this registration.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerregistrationdeployment">ControllerRegistrationDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#controllerregistrationspec">ControllerRegistrationSpec</a>)
</p>

<p>
ControllerRegistrationDeployment contains information for how this controller is deployed.
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
<code>policy</code></br>
<em>
<a href="#controllerdeploymentpolicy">ControllerDeploymentPolicy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Policy controls how the controller is deployed. It defaults to 'OnDemand'.</p>
</td>
</tr>
<tr>
<td>
<code>seedSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector contains an optional label selector for seeds. Only if the labels match then this controller will be<br />considered for a deployment.<br />An empty list means that all seeds are selected.</p>
</td>
</tr>
<tr>
<td>
<code>deploymentRefs</code></br>
<em>
<a href="#deploymentref">DeploymentRef</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeploymentRefs holds references to `ControllerDeployments`. Only one element is supported currently.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerregistrationspec">ControllerRegistrationSpec
</h3>


<p>
(<em>Appears on:</em><a href="#controllerregistration">ControllerRegistration</a>)
</p>

<p>
ControllerRegistrationSpec is the specification of a ControllerRegistration.
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
<code>resources</code></br>
<em>
<a href="#controllerresource">ControllerResource</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of combinations of kinds (DNSProvider, Infrastructure, Generic, ...) and their actual types<br />(aws-route53, gcp, auditlog, ...).</p>
</td>
</tr>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#controllerregistrationdeployment">ControllerRegistrationDeployment</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment contains information for how this controller is deployed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerresource">ControllerResource
</h3>


<p>
(<em>Appears on:</em><a href="#controllerregistrationspec">ControllerRegistrationSpec</a>)
</p>

<p>
ControllerResource is a combination of a kind (DNSProvider, Infrastructure, Generic, ...) and the actual type for this
kind (aws-route53, gcp, auditlog, ...).
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
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind is the resource kind, for example "OperatingSystemConfig".</p>
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
<p>Type is the resource type, for example "coreos" or "ubuntu".</p>
</td>
</tr>
<tr>
<td>
<code>reconcileTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.<br />This field is defaulted to 3m0s when kind is "Extension".</p>
</td>
</tr>
<tr>
<td>
<code>primary</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Primary determines if the controller backed by this ControllerRegistration is responsible for the extension<br />resource's lifecycle. This field defaults to true. There must be exactly one primary controller for this kind/type<br />combination. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code></br>
<em>
<a href="#controllerresourcelifecycle">ControllerResourceLifecycle</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lifecycle defines a strategy that determines when different operations on a ControllerResource should be performed.<br />This field is defaulted in the following way when kind is "Extension".<br /> Reconcile: "AfterKubeAPIServer"<br /> Delete: "BeforeKubeAPIServer"<br /> Migrate: "BeforeKubeAPIServer"</p>
</td>
</tr>
<tr>
<td>
<code>workerlessSupported</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkerlessSupported specifies whether this ControllerResource supports Workerless Shoot clusters.<br />This field is only relevant when kind is "Extension".</p>
</td>
</tr>
<tr>
<td>
<code>autoEnable</code></br>
<em>
<a href="#clustertype">ClusterType</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoEnable determines if this resource is automatically enabled for shoot or seed clusters, or both.<br />This field can only be set for resources of kind "Extension".</p>
</td>
</tr>
<tr>
<td>
<code>clusterCompatibility</code></br>
<em>
<a href="#clustertype">ClusterType</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterCompatibility defines the compatibility of this resource with different cluster types.<br />If compatibility is not specified, it will be defaulted to 'shoot'.<br />This field can only be set for resources of kind "Extension".</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerresourcelifecycle">ControllerResourceLifecycle
</h3>


<p>
(<em>Appears on:</em><a href="#controllerresource">ControllerResource</a>)
</p>

<p>
ControllerResourceLifecycle defines the lifecycle of a controller resource.
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
<code>reconcile</code></br>
<em>
<a href="#controllerresourcelifecyclestrategy">ControllerResourceLifecycleStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Reconcile defines the strategy during reconciliation.</p>
</td>
</tr>
<tr>
<td>
<code>delete</code></br>
<em>
<a href="#controllerresourcelifecyclestrategy">ControllerResourceLifecycleStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Delete defines the strategy during deletion.</p>
</td>
</tr>
<tr>
<td>
<code>migrate</code></br>
<em>
<a href="#controllerresourcelifecyclestrategy">ControllerResourceLifecycleStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Migrate defines the strategy during migration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controllerresourcelifecyclestrategy">ControllerResourceLifecycleStrategy
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#controllerresourcelifecycle">ControllerResourceLifecycle</a>)
</p>

<p>
ControllerResourceLifecycleStrategy is a string alias.
</p>


<h3 id="coredns">CoreDNS
</h3>


<p>
(<em>Appears on:</em><a href="#systemcomponents">SystemComponents</a>)
</p>

<p>
CoreDNS contains the settings of the Core DNS components running in the data plane of the Shoot cluster.
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
<code>autoscaling</code></br>
<em>
<a href="#corednsautoscaling">CoreDNSAutoscaling</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains the settings related to autoscaling of the Core DNS components running in the data plane of the Shoot cluster.</p>
</td>
</tr>
<tr>
<td>
<code>rewriting</code></br>
<em>
<a href="#corednsrewriting">CoreDNSRewriting</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Rewriting contains the setting related to rewriting of requests, which are obviously incorrect due to the unnecessary application of the search path.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="corednsautoscaling">CoreDNSAutoscaling
</h3>


<p>
(<em>Appears on:</em><a href="#coredns">CoreDNS</a>)
</p>

<p>
CoreDNSAutoscaling contains the settings related to autoscaling of the Core DNS components running in the data plane of the Shoot cluster.
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
<code>mode</code></br>
<em>
<a href="#corednsautoscalingmode">CoreDNSAutoscalingMode</a>
</em>
</td>
<td>
<p>The mode of the autoscaling to be used for the Core DNS components running in the data plane of the Shoot cluster.<br />Supported values are `horizontal` and `cluster-proportional`.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="corednsautoscalingmode">CoreDNSAutoscalingMode
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#corednsautoscaling">CoreDNSAutoscaling</a>)
</p>

<p>
CoreDNSAutoscalingMode is a type alias for the Core DNS autoscaling mode string.
</p>


<h3 id="corednsrewriting">CoreDNSRewriting
</h3>


<p>
(<em>Appears on:</em><a href="#coredns">CoreDNS</a>)
</p>

<p>
CoreDNSRewriting contains the setting related to rewriting requests, which are obviously incorrect due to the unnecessary application of the search path.
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
<code>commonSuffixes</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>CommonSuffixes are expected to be the suffix of a fully qualified domain name. Each suffix should contain at least one or two dots ('.') to prevent accidental clashes.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="credentialsrotationphase">CredentialsRotationPhase
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#carotation">CARotation</a>, <a href="#etcdencryptionkeyrotation">ETCDEncryptionKeyRotation</a>, <a href="#serviceaccountkeyrotation">ServiceAccountKeyRotation</a>)
</p>

<p>
CredentialsRotationPhase is a string alias.
</p>


<h3 id="dns">DNS
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
DNS holds information about the provider, the hosted zone id and the domain.
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
<code>domain</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Domain is the external available domain of the Shoot cluster. This domain will be written into the<br />kubeconfig that is handed out to end-users. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>providers</code></br>
<em>
<a href="#dnsprovider">DNSProvider</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Providers is a list of DNS providers that shall be enabled for this shoot cluster. Only relevant if<br />not a default domain is used.<br />Deprecated: Configuring multiple DNS providers is deprecated and will be forbidden in a future release.<br />Please use the DNS extension provider config (e.g. shoot-dns-service) for additional providers.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsexposure">DNSExposure
</h3>
<p><em>Underlying type: <a href="#struct{}">struct{}</a></em></p>


<p>
(<em>Appears on:</em><a href="#exposure">Exposure</a>)
</p>

<p>
DNSExposure specifies that this shoot will be exposed by DNS.
There is no specific configuration currently, for future extendability.
</p>


<h3 id="dnsincludeexclude">DNSIncludeExclude
</h3>


<p>
(<em>Appears on:</em><a href="#dnsprovider">DNSProvider</a>)
</p>

<p>
DNSIncludeExclude contains information about which domains shall be included/excluded.
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
<code>include</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Include is a list of domains that shall be included.</p>
</td>
</tr>
<tr>
<td>
<code>exclude</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Exclude is a list of domains that shall be excluded.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsprovider">DNSProvider
</h3>


<p>
(<em>Appears on:</em><a href="#dns">DNS</a>)
</p>

<p>
DNSProvider contains information about a DNS provider.
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
<code>domains</code></br>
<em>
<a href="#dnsincludeexclude">DNSIncludeExclude</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Domains contains information about which domains shall be included/excluded for this provider.<br />Deprecated: This field is deprecated and will be removed in a future release.<br />Please use the DNS extension provider config (e.g. shoot-dns-service) for additional configuration.</p>
</td>
</tr>
<tr>
<td>
<code>primary</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Primary indicates that this DNSProvider is used for shoot related domains.<br />Deprecated: This field is deprecated and will be removed in a future release.<br />Please use the DNS extension provider config (e.g. shoot-dns-service) for additional and non-primary providers.</p>
</td>
</tr>
<tr>
<td>
<code>secretName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretName is a name of a secret containing credentials for the stated domain and the<br />provider. When not specified, the Gardener will use the cloud provider credentials referenced<br />by the Shoot and try to find respective credentials there (primary provider only). Specifying this field may override<br />this behavior, i.e. forcing the Gardener to only look into the given secret.<br />Deprecated: This field is deprecated and will be forbidden starting from Kubernetes 1.35. Please use `CredentialsRef` instead.<br />Until removed, this field is synced with the `CredentialsRef` field when it refers to a secret.</p>
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
<p>Type is the DNS provider type.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#dnsincludeexclude">DNSIncludeExclude</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones contains information about which hosted zones shall be included/excluded for this provider.<br />Deprecated: This field is deprecated and will be removed in a future release.<br />Please use the DNS extension provider config (e.g. shoot-dns-service) for additional configuration.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#crossversionobjectreference-v1-autoscaling">CrossVersionObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CredentialsRef is a reference to a resource providing credentials for the DNS provider.<br />Supported resources are Secret and WorkloadIdentity.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="datavolume">DataVolume
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
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
<p>VolumeSize is the size of the volume.</p>
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


<h3 id="deploymentref">DeploymentRef
</h3>


<p>
(<em>Appears on:</em><a href="#controllerregistrationdeployment">ControllerRegistrationDeployment</a>)
</p>

<p>
DeploymentRef contains information about `ControllerDeployment` references.
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
<p>Name is the name of the `ControllerDeployment` that is being referred to.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dualapprovalfordeletion">DualApprovalForDeletion
</h3>


<p>
(<em>Appears on:</em><a href="#projectspec">ProjectSpec</a>)
</p>

<p>
DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.
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
<code>resource</code></br>
<em>
string
</em>
</td>
<td>
<p>Resource is the name of the resource this applies to.</p>
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
<p>Selector is the label selector for the resources.</p>
</td>
</tr>
<tr>
<td>
<code>includeServiceAccounts</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeServiceAccounts specifies whether the concept also applies when deletion is triggered by ServiceAccounts.<br />Defaults to true.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcd">ETCD
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
ETCD contains configuration for etcds of the shoot cluster.
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
<code>main</code></br>
<em>
<a href="#etcdconfig">ETCDConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Main contains configuration for the main etcd.</p>
</td>
</tr>
<tr>
<td>
<code>events</code></br>
<em>
<a href="#etcdconfig">ETCDConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Events contains configuration for the events etcd.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcdconfig">ETCDConfig
</h3>


<p>
(<em>Appears on:</em><a href="#etcd">ETCD</a>)
</p>

<p>
ETCDConfig contains etcd configuration.
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
<code>autoscaling</code></br>
<em>
<a href="#controlplaneautoscaling">ControlPlaneAutoscaling</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains auto-scaling configuration options for etcd.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcdencryptionkeyrotation">ETCDEncryptionKeyRotation
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentialsrotation">ShootCredentialsRotation</a>)
</p>

<p>
ETCDEncryptionKeyRotation contains information about the ETCD encryption key credential rotation.
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
<code>phase</code></br>
<em>
<a href="#credentialsrotationphase">CredentialsRotationPhase</a>
</em>
</td>
<td>
<p>Phase describes the phase of the ETCD encryption key credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the ETCD encryption key credential rotation was successfully<br />completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationTime is the most recent time when the ETCD encryption key credential rotation was initiated.</p>
</td>
</tr>
<tr>
<td>
<code>lastInitiationFinishedTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationFinishedTime is the recent time when the ETCD encryption key credential rotation initiation was<br />completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTriggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTriggeredTime is the recent time when the ETCD encryption key credential rotation completion was<br />triggered.</p>
</td>
</tr>
<tr>
<td>
<code>autoCompleteAfterPrepared</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoCompleteAfterPrepared indicates whether the current ETCD encryption key rotation should be auto completed after the preparation phase has finished.<br />Such rotation can be triggered by the `rotate-etcd-encryption-key` annotation.<br />This field is needed while we support two types of key rotations: two-operation and single operation rotation.<br />Deprecated: This field will be removed in a future release. The field will be no longer needed with<br />the removal `rotate-etcd-encryption-key-start` & `rotate-etcd-encryption-key-complete` annotations.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="encryptionatrest">EncryptionAtRest
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentials">ShootCredentials</a>)
</p>

<p>
EncryptionAtRest contains information about Shoot data encryption at rest.
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
<code>resources</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is the list of resources in the Shoot which are currently encrypted.<br />Secrets are encrypted by default and are not part of the list.<br />See https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md for more details.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
<a href="#encryptionproviderstatus">EncryptionProviderStatus</a>
</em>
</td>
<td>
<p>Provider contains information about Shoot encryption provider.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="encryptionconfig">EncryptionConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
EncryptionConfig contains customizable encryption configuration of the API server.
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
<code>resources</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources contains the list of resources that shall be encrypted in addition to secrets.<br />Each item is a Kubernetes resource name in plural (resource or resource.group) that should be encrypted.<br />Wildcards are not supported for now.<br />See https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md for more details.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
<a href="#encryptionprovider">EncryptionProvider</a>
</em>
</td>
<td>
<p>Provider contains information about the encryption provider.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="encryptionprovider">EncryptionProvider
</h3>


<p>
(<em>Appears on:</em><a href="#encryptionconfig">EncryptionConfig</a>)
</p>

<p>
EncryptionProvider contains information about the encryption provider.
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
<a href="#encryptionprovidertype">EncryptionProviderType</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type contains the type of the encryption provider.<br />Supported types:<br />  - "aescbc"<br />Defaults to aescbc.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="encryptionproviderstatus">EncryptionProviderStatus
</h3>


<p>
(<em>Appears on:</em><a href="#encryptionatrest">EncryptionAtRest</a>)
</p>

<p>
EncryptionProviderStatus contains information about Shoot encryption provider.
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
<a href="#encryptionprovidertype">EncryptionProviderType</a>
</em>
</td>
<td>
<p>Type is the used encryption provider type.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="encryptionprovidertype">EncryptionProviderType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#encryptionprovider">EncryptionProvider</a>, <a href="#encryptionproviderstatus">EncryptionProviderStatus</a>)
</p>

<p>
EncryptionProviderType is a type alias for the encryption provider type string.
</p>


<h3 id="errorcode">ErrorCode
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#condition">Condition</a>, <a href="#lasterror">LastError</a>)
</p>

<p>
ErrorCode is a string alias.
</p>


<h3 id="expandermode">ExpanderMode
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#clusterautoscaler">ClusterAutoscaler</a>)
</p>

<p>
ExpanderMode is type used for Expander values
</p>


<h3 id="expirableversion">ExpirableVersion
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetessettings">KubernetesSettings</a>, <a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
ExpirableVersion contains a version with associated lifecycle information.
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version identifier.</p>
</td>
</tr>
<tr>
<td>
<code>expirationDate</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationDate defines the time at which this version expires.<br />Deprecated: Is replaced by Lifecycle; mutually exclusive with it.</p>
</td>
</tr>
<tr>
<td>
<code>classification</code></br>
<em>
<a href="#versionclassification">VersionClassification</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Classification defines the state of a version (preview, supported, deprecated).<br />Deprecated: Is replaced by Lifecycle. mutually exclusive with it.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code></br>
<em>
<a href="#lifecyclestage">LifecycleStage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lifecycle defines the lifecycle stages for this version.<br />Mutually exclusive with Classification and ExpirationDate.<br />This can only be used when the VersionClassificationLifecycle feature gate is enabled.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="expirableversionstatus">ExpirableVersionStatus
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetesstatus">KubernetesStatus</a>, <a href="#machineimagestatus">MachineImageStatus</a>)
</p>

<p>
ExpirableVersionStatus defines the current status of an expirable version.
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version identifier.</p>
</td>
</tr>
<tr>
<td>
<code>classification</code></br>
<em>
<a href="#versionclassification">VersionClassification</a>
</em>
</td>
<td>
<p>Classification reflects the current state in the classification lifecycle.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="exposure">Exposure
</h3>


<p>
(<em>Appears on:</em><a href="#workercontrolplane">WorkerControlPlane</a>)
</p>

<p>
Exposure holds the exposure configuration for the shoot (either `extension` or `dns` or omitted/empty).
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
<code>extension</code></br>
<em>
<a href="#extensionexposure">ExtensionExposure</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extension holds the type and provider config of the exposure extension.<br />Mutually exclusive with DNS.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#dnsexposure">DNSExposure</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNS specifies that this shoot will be exposed by DNS.<br />Mutually exclusive with Extension.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="exposureclass">ExposureClass
</h3>


<p>
ExposureClass represents a control plane endpoint exposure strategy.
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
<code>handler</code></br>
<em>
string
</em>
</td>
<td>
<p>Handler is the name of the handler which applies the control plane endpoint exposure strategy.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>scheduling</code></br>
<em>
<a href="#exposureclassscheduling">ExposureClassScheduling</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Scheduling holds information how to select applicable Seed's for ExposureClass usage.<br />This field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="exposureclassscheduling">ExposureClassScheduling
</h3>


<p>
(<em>Appears on:</em><a href="#exposureclass">ExposureClass</a>)
</p>

<p>
ExposureClassScheduling holds information to select applicable Seed's for ExposureClass usage.
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
<code>seedSelector</code></br>
<em>
<a href="#seedselector">SeedSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector is an optional label selector for Seed's which are suitable to use the ExposureClass.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="#toleration">Toleration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations contains the tolerations for taints on Seed clusters.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="extension">Extension
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>, <a href="#shootspec">ShootSpec</a>)
</p>

<p>
Extension contains type and provider information for extensions.
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
<p>Type is the type of the extension resource.</p>
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
<p>ProviderConfig is the configuration passed to extension resource.</p>
</td>
</tr>
<tr>
<td>
<code>disabled</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Disabled allows to disable extensions that were marked as 'automatically enabled' by Gardener administrators.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="extensionexposure">ExtensionExposure
</h3>
<p><em>Underlying type: <a href="#struct{type-*string-"json:\"type,omitempty\"-protobuf:\"bytes,1,opt,name=type\"";-providerconfig-*k8sioapimachinerypkgruntimerawextension-"json:\"providerconfig,omitempty\"-protobuf:\"bytes,2,opt,name=providerconfig\""}">struct{Type *string "json:\"type,omitempty\" protobuf:\"bytes,1,opt,name=type\""; ProviderConfig *k8s.io/apimachinery/pkg/runtime.RawExtension "json:\"providerConfig,omitempty\" protobuf:\"bytes,2,opt,name=providerConfig\""}</a></em></p>


<p>
(<em>Appears on:</em><a href="#exposure">Exposure</a>)
</p>

<p>
ExtensionExposure holds the type and provider config of the exposure extension.
</p>


<h3 id="extensionresourcestate">ExtensionResourceState
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatespec">ShootStateSpec</a>)
</p>

<p>
ExtensionResourceState contains the kind of the extension custom resource and its last observed state in the Shoot's
namespace on the Seed cluster.
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
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind (type) of the extension custom resource</p>
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
<em>(Optional)</em>
<p>Name of the extension custom resource</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Purpose of the extension custom resource</p>
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
<p>State of the extension resource</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#namedresourcereference">NamedResourceReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="failuretolerance">FailureTolerance
</h3>


<p>
(<em>Appears on:</em><a href="#highavailability">HighAvailability</a>)
</p>

<p>
FailureTolerance describes information about failure tolerance level of a highly available resource.
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
<a href="#failuretolerancetype">FailureToleranceType</a>
</em>
</td>
<td>
<p>Type specifies the type of failure that the highly available resource can tolerate</p>
</td>
</tr>

</tbody>
</table>


<h3 id="failuretolerancetype">FailureToleranceType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#failuretolerance">FailureTolerance</a>)
</p>

<p>
FailureToleranceType specifies the type of failure that a highly available
shoot control plane that can tolerate.
</p>


<h3 id="gardener">Gardener
</h3>


<p>
(<em>Appears on:</em><a href="#seedstatus">SeedStatus</a>, <a href="#shootstatus">ShootStatus</a>)
</p>

<p>
Gardener holds the information about the Gardener version that operated a resource.
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
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the container id of the Gardener which last acted on a resource.</p>
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
<p>Name is the hostname (pod name) of the Gardener which last acted on a resource.</p>
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
<p>Version is the version of the Gardener which last acted on a resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenerresourcedata">GardenerResourceData
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatespec">ShootStateSpec</a>)
</p>

<p>
GardenerResourceData holds the data which is used to generate resources, deployed in the Shoot's control plane.
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
<p>Name of the object required to generate resources</p>
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
<p>Type of the object</p>
</td>
</tr>
<tr>
<td>
<code>data</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>Data contains the payload required to generate resources</p>
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
<p>Labels are labels of the object</p>
</td>
</tr>

</tbody>
</table>


<h3 id="helmcontrollerdeployment">HelmControllerDeployment
</h3>


<p>
HelmControllerDeployment configures how an extension controller is deployed using helm.
This is the legacy structure that used to be defined in gardenlet's ControllerInstallation controller for
ControllerDeployment's with type=helm.
While this is not a proper API type, we need to define the structure in the API package so that we can convert it
to the internal API version in the new representation.
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
<code>chart</code></br>
<em>
integer array
</em>
</td>
<td>
<p>Chart is a Helm chart tarball.</p>
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
<p>Values is a map of values for the given chart.</p>
</td>
</tr>
<tr>
<td>
<code>ociRepository</code></br>
<em>
<a href="#ocirepository">OCIRepository</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OCIRepository defines where to pull the chart.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="hibernation">Hibernation
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Hibernation contains information whether the Shoot is suspended or not.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enabled specifies whether the Shoot needs to be hibernated or not. If it is true, the Shoot's desired state is to be hibernated.<br />If it is false or nil, the Shoot's desired state is to be awakened.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code></br>
<em>
<a href="#hibernationschedule">HibernationSchedule</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Schedules determine the hibernation schedules.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="hibernationschedule">HibernationSchedule
</h3>


<p>
(<em>Appears on:</em><a href="#hibernation">Hibernation</a>)
</p>

<p>
HibernationSchedule determines the hibernation schedule of a Shoot.
A Shoot will be regularly hibernated at each start time and will be woken up at each end time.
Start or End can be omitted, though at least one of each has to be specified.
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
<code>start</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Start is a Cron spec at which time a Shoot will be hibernated.</p>
</td>
</tr>
<tr>
<td>
<code>end</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>End is a Cron spec at which time a Shoot will be woken up.</p>
</td>
</tr>
<tr>
<td>
<code>location</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Location is the time location in which both start and shall be evaluated.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="highavailability">HighAvailability
</h3>


<p>
(<em>Appears on:</em><a href="#controlplane">ControlPlane</a>)
</p>

<p>
HighAvailability specifies the configuration settings for high availability for a resource. Typical
usages could be to configure HA for shoot control plane or for seed system components.
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
<code>failureTolerance</code></br>
<em>
<a href="#failuretolerance">FailureTolerance</a>
</em>
</td>
<td>
<p>FailureTolerance holds information about failure tolerance level of a highly available resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="horizontalpodautoscalerconfig">HorizontalPodAutoscalerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubecontrollermanagerconfig">KubeControllerManagerConfig</a>)
</p>

<p>
HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
Note: Descriptions were taken from the Kubernetes documentation.
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
<code>cpuInitializationPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The period after which a ready pod transition is considered to be the first.</p>
</td>
</tr>
<tr>
<td>
<code>downscaleStabilization</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The configurable window at which the controller will choose the highest recommendation for autoscaling.</p>
</td>
</tr>
<tr>
<td>
<code>initialReadinessDelay</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The configurable period at which the horizontal pod autoscaler considers a Pod “not yet ready” given that it’s unready and it has  transitioned to unready during that time.</p>
</td>
</tr>
<tr>
<td>
<code>syncPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The period for syncing the number of pods in horizontal pod autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>tolerance</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ipfamily">IPFamily
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#networking">Networking</a>, <a href="#seednetworks">SeedNetworks</a>)
</p>

<p>
IPFamily is a type for specifying an IP protocol version to use in Gardener clusters.
</p>


<h3 id="inplaceupdates">InPlaceUpdates
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
InPlaceUpdates contains the configuration for in-place updates for a machine image version.
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
<code>supported</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Supported indicates whether in-place updates are supported for this machine image version.</p>
</td>
</tr>
<tr>
<td>
<code>minVersionForUpdate</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinVersionForInPlaceUpdate specifies the minimum supported version from which an in-place update to this machine image version can be performed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="inplaceupdatesstatus">InPlaceUpdatesStatus
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatus">ShootStatus</a>)
</p>

<p>
InPlaceUpdatesStatus contains information about in-place updates for the Shoot workers.
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
<code>pendingWorkerUpdates</code></br>
<em>
<a href="#pendingworkerupdates">PendingWorkerUpdates</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkerUpdates contains information about worker pools pending in-place updates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ingress">Ingress
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
Ingress configures the Ingress specific settings of the cluster
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
<code>domain</code></br>
<em>
string
</em>
</td>
<td>
<p>Domain specifies the IngressDomain of the cluster pointing to the ingress controller endpoint. It will be used<br />to construct ingress URLs for system applications running in Shoot/Garden clusters. Once set this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>controller</code></br>
<em>
<a href="#ingresscontroller">IngressController</a>
</em>
</td>
<td>
<p>Controller configures a Gardener managed Ingress Controller listening on the ingressDomain</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ingresscontroller">IngressController
</h3>


<p>
(<em>Appears on:</em><a href="#ingress">Ingress</a>)
</p>

<p>
IngressController enables a Gardener managed Ingress Controller listening on the ingressDomain
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
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind defines which kind of IngressController to use. At the moment only `nginx` is supported</p>
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
<p>ProviderConfig specifies infrastructure specific configuration for the ingressController</p>
</td>
</tr>

</tbody>
</table>


<h3 id="internalsecret">InternalSecret
</h3>


<p>
InternalSecret holds secret data of a certain type. The total bytes of the values in
the Data field must be less than MaxSecretSize bytes.
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
<code>immutable</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Immutable, if set to true, ensures that data stored in the Secret cannot<br />be updated (only object metadata can be modified).<br />If not set to true, the field can be modified at any time.<br />Defaulted to nil.</p>
</td>
</tr>
<tr>
<td>
<code>data</code></br>
<em>
object (keys:string, values:integer array)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Data contains the secret data. Each key must consist of alphanumeric<br />characters, '-', '_' or '.'. The serialized form of the secret data is a<br />base64 encoded string, representing the arbitrary (possibly non-string)<br />data value here. Described in https://tools.ietf.org/html/rfc4648#section-4</p>
</td>
</tr>
<tr>
<td>
<code>stringData</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>stringData allows specifying non-binary secret data in string form.<br />It is provided as a write-only input field for convenience.<br />All keys and values are merged into the data field on write, overwriting any existing values.<br />The stringData field is never output when reading from the API.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secrettype-v1-core">SecretType</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used to facilitate programmatic handling of secret data.<br />More info: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeapiserverconfig">KubeAPIServerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
KubeAPIServerConfig contains configuration settings for the kube-apiserver.
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
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
<tr>
<td>
<code>admissionPlugins</code></br>
<em>
<a href="#admissionplugin">AdmissionPlugin</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding<br />configuration.</p>
</td>
</tr>
<tr>
<td>
<code>apiAudiences</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIAudiences are the identifiers of the API. The service account token authenticator will<br />validate that tokens used against the API are bound to at least one of these audiences.<br />Defaults to ["kubernetes"].</p>
</td>
</tr>
<tr>
<td>
<code>auditConfig</code></br>
<em>
<a href="#auditconfig">AuditConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuditConfig contains configuration settings for the audit of the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>oidcConfig</code></br>
<em>
<a href="#oidcconfig">OIDCConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OIDCConfig contains configuration settings for the OIDC provider.<br />Deprecated: This field is deprecated and will be forbidden starting from Kubernetes 1.32.<br />Please configure and use structured authentication instead of oidc flags.<br />For more information check https://github.com/gardener/gardener/issues/9858</p>
</td>
</tr>
<tr>
<td>
<code>runtimeConfig</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>RuntimeConfig contains information about enabled or disabled APIs.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountConfig</code></br>
<em>
<a href="#serviceaccountconfig">ServiceAccountConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountConfig contains configuration settings for the service account handling<br />of the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>watchCacheSizes</code></br>
<em>
<a href="#watchcachesizes">WatchCacheSizes</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WatchCacheSizes contains configuration of the API server's watch cache sizes.<br />Configuring these flags might be useful for large-scale Shoot clusters with a lot of parallel update requests<br />and a lot of watching controllers (e.g. large ManagedSeed clusters). When the API server's watch cache's<br />capacity is too small to cope with the amount of update requests and watchers for a particular resource, it<br />might happen that controller watches are permanently stopped with `too old resource version` errors.<br />Starting from kubernetes v1.19, the API server's watch cache size is adapted dynamically and setting the watch<br />cache size flags will have no effect, except when setting it to 0 (which disables the watch cache).</p>
</td>
</tr>
<tr>
<td>
<code>requests</code></br>
<em>
<a href="#apiserverrequests">APIServerRequests</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Requests contains configuration for request-specific settings for the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>enableAnonymousAuthentication</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableAnonymousAuthentication defines whether anonymous requests to the secure port<br />of the API server should be allowed (flag `--anonymous-auth`).<br />See: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/<br />Deprecated: This field is deprecated and will be removed after support for Kubernetes v1.34 is dropped.<br />This field is forbidden for clusters with Kubernetes version >= 1.35.<br />Please use anonymous authentication configuration instead.</p>
</td>
</tr>
<tr>
<td>
<code>eventTTL</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EventTTL controls the amount of time to retain events.<br />Defaults to 1h.</p>
</td>
</tr>
<tr>
<td>
<code>logging</code></br>
<em>
<a href="#apiserverlogging">APIServerLogging</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Logging contains configuration for the log level and HTTP access logs.</p>
</td>
</tr>
<tr>
<td>
<code>defaultNotReadyTolerationSeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultNotReadyTolerationSeconds indicates the tolerationSeconds of the toleration for notReady:NoExecute<br />that is added by default to every pod that does not already have such a toleration (flag `--default-not-ready-toleration-seconds`).<br />The field has effect only when the `DefaultTolerationSeconds` admission plugin is enabled.<br />Defaults to 300.</p>
</td>
</tr>
<tr>
<td>
<code>defaultUnreachableTolerationSeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultUnreachableTolerationSeconds indicates the tolerationSeconds of the toleration for unreachable:NoExecute<br />that is added by default to every pod that does not already have such a toleration (flag `--default-unreachable-toleration-seconds`).<br />The field has effect only when the `DefaultTolerationSeconds` admission plugin is enabled.<br />Defaults to 300.</p>
</td>
</tr>
<tr>
<td>
<code>encryptionConfig</code></br>
<em>
<a href="#encryptionconfig">EncryptionConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EncryptionConfig contains customizable encryption configuration of the Kube API server.</p>
</td>
</tr>
<tr>
<td>
<code>structuredAuthentication</code></br>
<em>
<a href="#structuredauthentication">StructuredAuthentication</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StructuredAuthentication contains configuration settings for structured authentication for the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>structuredAuthorization</code></br>
<em>
<a href="#structuredauthorization">StructuredAuthorization</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StructuredAuthorization contains configuration settings for structured authorization for the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>autoscaling</code></br>
<em>
<a href="#controlplaneautoscaling">ControlPlaneAutoscaling</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains auto-scaling configuration options for the kube-apiserver.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubecontrollermanagerconfig">KubeControllerManagerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.
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
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
<tr>
<td>
<code>horizontalPodAutoscaler</code></br>
<em>
<a href="#horizontalpodautoscalerconfig">HorizontalPodAutoscalerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>nodeCIDRMaskSize</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeCIDRMaskSize defines the mask size for node cidr in cluster (default is 24). This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>podEvictionTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodEvictionTimeout defines the grace period for deleting pods on failed nodes. Defaults to 2m.<br />Deprecated: The corresponding kube-controller-manager flag `--pod-eviction-timeout` is deprecated<br />in favor of the kube-apiserver flags `--default-not-ready-toleration-seconds` and `--default-unreachable-toleration-seconds`.<br />The `--pod-eviction-timeout` flag does not have effect when the taint based eviction is enabled. The taint<br />based eviction is beta (enabled by default) since Kubernetes 1.13 and GA since Kubernetes 1.18. Hence,<br />instead of setting this field, set the `spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds` and<br />`spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds`. Setting this field is forbidden starting<br />from Kubernetes 1.33.</p>
</td>
</tr>
<tr>
<td>
<code>nodeMonitorGracePeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeMonitorGracePeriod defines the grace period before an unresponsive node is marked unhealthy.</p>
</td>
</tr>
<tr>
<td>
<code>nodeCIDRMaskSizeIPv6</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeCIDRMaskSizeIPv6 defines the mask size for node cidr in cluster (default is 64). This field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeproxyconfig">KubeProxyConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
KubeProxyConfig contains configuration settings for the kube-proxy.
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
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
<tr>
<td>
<code>mode</code></br>
<em>
<a href="#proxymode">ProxyMode</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mode specifies which proxy mode to use.<br />defaults to IPTables.</p>
</td>
</tr>
<tr>
<td>
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enabled indicates whether kube-proxy should be deployed or not.<br />Depending on the networking extensions switching kube-proxy off might be rejected. Consulting the respective documentation of the used networking extension is recommended before using this field.<br />defaults to true if not specified.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeschedulerconfig">KubeSchedulerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
KubeSchedulerConfig contains configuration settings for the kube-scheduler.
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
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
<tr>
<td>
<code>kubeMaxPDVols</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeMaxPDVols is not respected anymore by kube-scheduler.<br />The maximum number of attached volumes is configured by the CSI driver.<br />More information can be found at https://kubernetes.io/docs/concepts/storage/storage-limits/#custom-limits.<br />Deprecated: This field is deprecated. Using this field will be forbidden starting from Kubernetes 1.35.</p>
</td>
</tr>
<tr>
<td>
<code>profile</code></br>
<em>
<a href="#schedulingprofile">SchedulingProfile</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Profile configures the scheduling profile for the cluster.<br />If not specified, the used profile is "balanced" (provides the default kube-scheduler behavior).</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeletconfig">KubeletConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>, <a href="#workerkubernetes">WorkerKubernetes</a>)
</p>

<p>
KubeletConfig contains configuration settings for the kubelet.
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
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
<tr>
<td>
<code>cpuCFSQuota</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>CPUCFSQuota allows you to disable/enable CPU throttling for Pods.</p>
</td>
</tr>
<tr>
<td>
<code>cpuManagerPolicy</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CPUManagerPolicy allows to set alternative CPU management policies (default: none).</p>
</td>
</tr>
<tr>
<td>
<code>evictionHard</code></br>
<em>
<a href="#kubeletconfigeviction">KubeletConfigEviction</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionHard describes a set of eviction thresholds (e.g. memory.available<1Gi) that if met would trigger a Pod eviction.<br />Default:<br />  memory.available:   "100Mi/1Gi/5%"<br />  nodefs.available:   "5%"<br />  nodefs.inodesFree:  "5%"<br />  imagefs.available:  "5%"<br />  imagefs.inodesFree: "5%"</p>
</td>
</tr>
<tr>
<td>
<code>evictionMaxPodGracePeriod</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionMaxPodGracePeriod describes the maximum allowed grace period (in seconds) to use when terminating pods in response to a soft eviction threshold being met.<br />Default: 90</p>
</td>
</tr>
<tr>
<td>
<code>evictionMinimumReclaim</code></br>
<em>
<a href="#kubeletconfigevictionminimumreclaim">KubeletConfigEvictionMinimumReclaim</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionMinimumReclaim configures the amount of resources below the configured eviction threshold that the kubelet attempts to reclaim whenever the kubelet observes resource pressure.<br />Default: 0 for each resource</p>
</td>
</tr>
<tr>
<td>
<code>evictionPressureTransitionPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionPressureTransitionPeriod is the duration for which the kubelet has to wait before transitioning out of an eviction pressure condition.<br />Default: 4m0s</p>
</td>
</tr>
<tr>
<td>
<code>evictionSoft</code></br>
<em>
<a href="#kubeletconfigeviction">KubeletConfigEviction</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionSoft describes a set of eviction thresholds (e.g. memory.available<1.5Gi) that if met over a corresponding grace period would trigger a Pod eviction.<br />Default:<br />  memory.available:   "200Mi/1.5Gi/10%"<br />  nodefs.available:   "10%"<br />  nodefs.inodesFree:  "10%"<br />  imagefs.available:  "10%"<br />  imagefs.inodesFree: "10%"</p>
</td>
</tr>
<tr>
<td>
<code>evictionSoftGracePeriod</code></br>
<em>
<a href="#kubeletconfigevictionsoftgraceperiod">KubeletConfigEvictionSoftGracePeriod</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionSoftGracePeriod describes a set of eviction grace periods (e.g. memory.available=1m30s) that correspond to how long a soft eviction threshold must hold before triggering a Pod eviction.<br />Default:<br />  memory.available:   1m30s<br />  nodefs.available:   1m30s<br />  nodefs.inodesFree:  1m30s<br />  imagefs.available:  1m30s<br />  imagefs.inodesFree: 1m30s</p>
</td>
</tr>
<tr>
<td>
<code>maxPods</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxPods is the maximum number of Pods that are allowed by the Kubelet.<br />Default: 110</p>
</td>
</tr>
<tr>
<td>
<code>podPidsLimit</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodPIDsLimit is the maximum number of process IDs per pod allowed by the kubelet.</p>
</td>
</tr>
<tr>
<td>
<code>failSwapOn</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailSwapOn makes the Kubelet fail to start if swap is enabled on the node. (default true).</p>
</td>
</tr>
<tr>
<td>
<code>kubeReserved</code></br>
<em>
<a href="#kubeletconfigreserved">KubeletConfigReserved</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeReserved is the configuration for resources reserved for kubernetes node components (mainly kubelet and container runtime).<br />When updating these values, be aware that cgroup resizes may not succeed on active worker nodes. Look for the NodeAllocatableEnforced event to determine if the configuration was applied.<br />Default: cpu=80m,memory=1Gi,pid=20k</p>
</td>
</tr>
<tr>
<td>
<code>imageGCHighThresholdPercent</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageGCHighThresholdPercent describes the percent of the disk usage which triggers image garbage collection.<br />Default: 50</p>
</td>
</tr>
<tr>
<td>
<code>imageGCLowThresholdPercent</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageGCLowThresholdPercent describes the percent of the disk to which garbage collection attempts to free.<br />Default: 40</p>
</td>
</tr>
<tr>
<td>
<code>serializeImagePulls</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>SerializeImagePulls describes whether the images are pulled one at a time.<br />Default: true</p>
</td>
</tr>
<tr>
<td>
<code>registryPullQPS</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>RegistryPullQPS is the limit of registry pulls per second. The value must not be a negative number.<br />Setting it to 0 means no limit.<br />Default: 5</p>
</td>
</tr>
<tr>
<td>
<code>registryBurst</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>RegistryBurst is the maximum size of bursty pulls, temporarily allows pulls to burst to this number,<br />while still not exceeding registryPullQPS. The value must not be a negative number.<br />Only used if registryPullQPS is greater than 0.<br />Default: 10</p>
</td>
</tr>
<tr>
<td>
<code>seccompDefault</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeccompDefault enables the use of `RuntimeDefault` as the default seccomp profile for all workloads.</p>
</td>
</tr>
<tr>
<td>
<code>containerLogMaxSize</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A quantity defines the maximum size of the container log file before it is rotated. For example: "5Mi" or "256Ki".<br />Default: 100Mi</p>
</td>
</tr>
<tr>
<td>
<code>containerLogMaxFiles</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maximum number of container log files that can be present for a container.</p>
</td>
</tr>
<tr>
<td>
<code>protectKernelDefaults</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProtectKernelDefaults ensures that the kernel tunables are equal to the kubelet defaults.<br />Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>streamingConnectionIdleTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StreamingConnectionIdleTimeout is the maximum time a streaming connection can be idle before the connection is automatically closed.<br />This field cannot be set lower than "30s" or greater than "4h".<br />Default: "5m".</p>
</td>
</tr>
<tr>
<td>
<code>memorySwap</code></br>
<em>
<a href="#memoryswapconfiguration">MemorySwapConfiguration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemorySwap configures swap memory available to container workloads.</p>
</td>
</tr>
<tr>
<td>
<code>maxParallelImagePulls</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxParallelImagePulls describes the maximum number of image pulls in parallel. The value must be a positive number.<br />This field cannot be set if SerializeImagePulls (pull one image at a time) is set to true.<br />Setting it to nil means no limit.<br />Default: nil</p>
</td>
</tr>
<tr>
<td>
<code>imageMinimumGCAge</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageMinimumGCAge is the minimum age of an unused image before it can be garbage collected.<br />Default: 2m0s</p>
</td>
</tr>
<tr>
<td>
<code>imageMaximumGCAge</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageMaximumGCAge is the maximum age of an unused image before it can be garbage collected.<br />Default: 0s</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeletconfigeviction">KubeletConfigEviction
</h3>


<p>
(<em>Appears on:</em><a href="#kubeletconfig">KubeletConfig</a>)
</p>

<p>
KubeletConfigEviction contains kubelet eviction thresholds supporting either a resource.Quantity or a percentage based value.
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
<code>memoryAvailable</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAvailable is the threshold for the free memory on the host server.</p>
</td>
</tr>
<tr>
<td>
<code>imageFSAvailable</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageFSAvailable is the threshold for the free disk space in the imagefs filesystem (docker images and container writable layers).</p>
</td>
</tr>
<tr>
<td>
<code>imageFSInodesFree</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageFSInodesFree is the threshold for the available inodes in the imagefs filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>nodeFSAvailable</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSAvailable is the threshold for the free disk space in the nodefs filesystem (docker volumes, logs, etc).</p>
</td>
</tr>
<tr>
<td>
<code>nodeFSInodesFree</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSInodesFree is the threshold for the available inodes in the nodefs filesystem.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeletconfigevictionminimumreclaim">KubeletConfigEvictionMinimumReclaim
</h3>


<p>
(<em>Appears on:</em><a href="#kubeletconfig">KubeletConfig</a>)
</p>

<p>
KubeletConfigEvictionMinimumReclaim contains configuration for the kubelet eviction minimum reclaim.
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
<code>memoryAvailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAvailable is the threshold for the memory reclaim on the host server.</p>
</td>
</tr>
<tr>
<td>
<code>imageFSAvailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageFSAvailable is the threshold for the disk space reclaim in the imagefs filesystem (docker images and container writable layers).</p>
</td>
</tr>
<tr>
<td>
<code>imageFSInodesFree</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageFSInodesFree is the threshold for the inodes reclaim in the imagefs filesystem.</p>
</td>
</tr>
<tr>
<td>
<code>nodeFSAvailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSAvailable is the threshold for the disk space reclaim in the nodefs filesystem (docker volumes, logs, etc).</p>
</td>
</tr>
<tr>
<td>
<code>nodeFSInodesFree</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSInodesFree is the threshold for the inodes reclaim in the nodefs filesystem.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeletconfigevictionsoftgraceperiod">KubeletConfigEvictionSoftGracePeriod
</h3>


<p>
(<em>Appears on:</em><a href="#kubeletconfig">KubeletConfig</a>)
</p>

<p>
KubeletConfigEvictionSoftGracePeriod contains grace periods for kubelet eviction thresholds.
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
<code>memoryAvailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAvailable is the grace period for the MemoryAvailable eviction threshold.</p>
</td>
</tr>
<tr>
<td>
<code>imageFSAvailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageFSAvailable is the grace period for the ImageFSAvailable eviction threshold.</p>
</td>
</tr>
<tr>
<td>
<code>imageFSInodesFree</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageFSInodesFree is the grace period for the ImageFSInodesFree eviction threshold.</p>
</td>
</tr>
<tr>
<td>
<code>nodeFSAvailable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSAvailable is the grace period for the NodeFSAvailable eviction threshold.</p>
</td>
</tr>
<tr>
<td>
<code>nodeFSInodesFree</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSInodesFree is the grace period for the NodeFSInodesFree eviction threshold.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeletconfigreserved">KubeletConfigReserved
</h3>


<p>
(<em>Appears on:</em><a href="#kubeletconfig">KubeletConfig</a>)
</p>

<p>
KubeletConfigReserved contains reserved resources for daemons
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
<code>cpu</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CPU is the reserved cpu.</p>
</td>
</tr>
<tr>
<td>
<code>memory</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Memory is the reserved memory.</p>
</td>
</tr>
<tr>
<td>
<code>ephemeralStorage</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EphemeralStorage is the reserved ephemeral-storage.</p>
</td>
</tr>
<tr>
<td>
<code>pid</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PID is the reserved process-ids.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubernetes">Kubernetes
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Kubernetes contains the version and configuration variables for the Shoot control plane.
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
<code>clusterAutoscaler</code></br>
<em>
<a href="#clusterautoscaler">ClusterAutoscaler</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterAutoscaler contains the configuration flags for the Kubernetes cluster autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>kubeAPIServer</code></br>
<em>
<a href="#kubeapiserverconfig">KubeAPIServerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeAPIServer contains configuration settings for the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>kubeControllerManager</code></br>
<em>
<a href="#kubecontrollermanagerconfig">KubeControllerManagerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeControllerManager contains configuration settings for the kube-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>kubeScheduler</code></br>
<em>
<a href="#kubeschedulerconfig">KubeSchedulerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeScheduler contains configuration settings for the kube-scheduler.</p>
</td>
</tr>
<tr>
<td>
<code>kubeProxy</code></br>
<em>
<a href="#kubeproxyconfig">KubeProxyConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeProxy contains configuration settings for the kube-proxy.</p>
</td>
</tr>
<tr>
<td>
<code>kubelet</code></br>
<em>
<a href="#kubeletconfig">KubeletConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubelet contains configuration settings for the kubelet.</p>
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
<em>(Optional)</em>
<p>Version is the semantic Kubernetes version to use for the Shoot cluster.<br />Defaults to the highest supported minor and patch version given in the referenced cloud profile.<br />The version can be omitted completely or partially specified, e.g. `<major>.<minor>`.</p>
</td>
</tr>
<tr>
<td>
<code>verticalPodAutoscaler</code></br>
<em>
<a href="#verticalpodautoscaler">VerticalPodAutoscaler</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#etcd">ETCD</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCD contains configuration for etcds of the shoot cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubernetesconfig">KubernetesConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>, <a href="#kubecontrollermanagerconfig">KubeControllerManagerConfig</a>, <a href="#kubeproxyconfig">KubeProxyConfig</a>, <a href="#kubeschedulerconfig">KubeSchedulerConfig</a>, <a href="#kubeletconfig">KubeletConfig</a>)
</p>

<p>
KubernetesConfig contains common configuration fields for the control plane components.

This is a legacy type that should not be used in new API fields or resources.
Instead of embedding this type, consider using inline map for feature gates definitions.
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
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubernetesdashboard">KubernetesDashboard
</h3>


<p>
(<em>Appears on:</em><a href="#addons">Addons</a>)
</p>

<p>
KubernetesDashboard describes configuration values for the kubernetes-dashboard addon.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled indicates whether the addon is enabled or not.</p>
</td>
</tr>
<tr>
<td>
<code>authenticationMode</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuthenticationMode defines the authentication mode for the kubernetes-dashboard.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubernetessettings">KubernetesSettings
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>, <a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>)
</p>

<p>
KubernetesSettings contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
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
<code>versions</code></br>
<em>
<a href="#expirableversion">ExpirableVersion</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubernetesstatus">KubernetesStatus
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilestatus">CloudProfileStatus</a>)
</p>

<p>
KubernetesStatus contains the status information for kubernetes.
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
<code>versions</code></br>
<em>
<a href="#expirableversionstatus">ExpirableVersionStatus</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions contains the statuses of the kubernetes versions.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="lasterror">LastError
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketstatus">BackupBucketStatus</a>, <a href="#backupentrystatus">BackupEntryStatus</a>, <a href="#shootstatus">ShootStatus</a>)
</p>

<p>
LastError indicates the last occurred error for an operation on a resource.
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
<code>description</code></br>
<em>
string
</em>
</td>
<td>
<p>A human readable message indicating details about the last error.</p>
</td>
</tr>
<tr>
<td>
<code>taskID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ID of the task which caused this last error</p>
</td>
</tr>
<tr>
<td>
<code>codes</code></br>
<em>
<a href="#errorcode">ErrorCode</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Well-defined error codes of the last error(s).</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Last time the error was reported</p>
</td>
</tr>

</tbody>
</table>


<h3 id="lastmaintenance">LastMaintenance
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatus">ShootStatus</a>)
</p>

<p>
LastMaintenance holds information about a maintenance operation on the Shoot.
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
<code>description</code></br>
<em>
string
</em>
</td>
<td>
<p>A human-readable message containing details about the operations performed in the last maintenance.</p>
</td>
</tr>
<tr>
<td>
<code>triggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>TriggeredTime is the time when maintenance was triggered.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="#lastoperationstate">LastOperationState</a>
</em>
</td>
<td>
<p>Status of the last maintenance operation, one of Processing, Succeeded, Error.</p>
</td>
</tr>
<tr>
<td>
<code>failureReason</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailureReason holds the information about the last maintenance operation failure reason.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="lastoperation">LastOperation
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketstatus">BackupBucketStatus</a>, <a href="#backupentrystatus">BackupEntryStatus</a>, <a href="#seedstatus">SeedStatus</a>, <a href="#shootstatus">ShootStatus</a>)
</p>

<p>
LastOperation indicates the type and the state of the last operation, along with a description
message and a progress indicator.
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
<code>description</code></br>
<em>
string
</em>
</td>
<td>
<p>A human readable message indicating details about the last operation.</p>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>Last time the operation state transitioned from one to another.</p>
</td>
</tr>
<tr>
<td>
<code>progress</code></br>
<em>
integer
</em>
</td>
<td>
<p>The progress in percentage (0-100) of the last operation.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="#lastoperationstate">LastOperationState</a>
</em>
</td>
<td>
<p>Status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#lastoperationtype">LastOperationType</a>
</em>
</td>
<td>
<p>Type of the last operation, one of Create, Reconcile, Delete, Migrate, Restore.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="lastoperationstate">LastOperationState
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#lastmaintenance">LastMaintenance</a>, <a href="#lastoperation">LastOperation</a>)
</p>

<p>
LastOperationState is a string alias.
</p>


<h3 id="lastoperationtype">LastOperationType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#lastoperation">LastOperation</a>)
</p>

<p>
LastOperationType is a string alias.
</p>


<h3 id="lifecyclestage">LifecycleStage
</h3>


<p>
(<em>Appears on:</em><a href="#expirableversion">ExpirableVersion</a>, <a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
LifecycleStage describes a stage in the versions lifecycle.
Each stage defines the classification of the version (e.g. unavailable, preview, supported, deprecated, expired)
and the time at which this classification becomes effective.
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
<code>classification</code></br>
<em>
<a href="#versionclassification">VersionClassification</a>
</em>
</td>
<td>
<p>Classification is the category of this lifecycle stage (unavailable, preview, supported, deprecated, expired).</p>
</td>
</tr>
<tr>
<td>
<code>startTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartTime defines when this lifecycle stage becomes active.<br />StartTime can be omitted for the first lifecycle stage, implying a start time in the past.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="limits">Limits
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>, <a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>)
</p>

<p>
Limits configures operational limits for Shoot clusters using this CloudProfile.
See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
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
<code>maxNodesTotal</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNodesTotal configures the maximum node count a Shoot cluster can have during runtime.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="loadbalancerservicesproxyprotocol">LoadBalancerServicesProxyProtocol
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettingloadbalancerservices">SeedSettingLoadBalancerServices</a>, <a href="#seedsettingloadbalancerserviceszones">SeedSettingLoadBalancerServicesZones</a>)
</p>

<p>
LoadBalancerServicesProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
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
<code>allowed</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Allowed controls whether the ProxyProtocol is optionally allowed for the load balancer services.<br />This should only be enabled if the load balancer services are already using ProxyProtocol or will be reconfigured to use it soon.<br />Until the load balancers are configured with ProxyProtocol, enabling this setting may allow clients to spoof their source IP addresses.<br />The option allows a migration from non-ProxyProtocol to ProxyProtocol without downtime (depending on the infrastructure).<br />Defaults to false.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machine">Machine
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
Machine contains information about the machine type and image.
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
<p>Type is the machine type of the worker group.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
<a href="#shootmachineimage">ShootMachineImage</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image holds information about the machine image to use for all nodes of this pool. It will default to the<br />latest version of the first image stated in the referenced CloudProfile if no value has been provided.</p>
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
<p>Architecture is CPU architecture of machines in this worker pool.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinecontrollermanagersettings">MachineControllerManagerSettings
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
MachineControllerManagerSettings contains configurations for different worker-pools. Eg. MachineDrainTimeout, MachineHealthTimeout.
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
<code>machineDrainTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineDrainTimeout is the period after which machine is forcefully deleted.</p>
</td>
</tr>
<tr>
<td>
<code>machineHealthTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineHealthTimeout is the period after which machine is declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>machineCreationTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineCreationTimeout is the period after which creation of the machine is declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>maxEvictRetries</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxEvictRetries are the number of eviction retries on a pod after which drain is declared failed, and forceful deletion is triggered.</p>
</td>
</tr>
<tr>
<td>
<code>nodeConditions</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeConditions are the set of conditions if set to true for the period of MachineHealthTimeout, machine will be declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdateTimeout</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineInPlaceUpdateTimeout is the timeout after which in-place update is declared failed.</p>
</td>
</tr>
<tr>
<td>
<code>disableHealthTimeout</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableHealthTimeout if set to true, health timeout will be ignored. Leading to machine never being declared failed.<br />This is intended to be used only for in-place updates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimage">MachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>, <a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>)
</p>

<p>
MachineImage defines the name and multiple versions of the machine image in any environment.
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
<p>Name is the name of the image.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#machineimageversion">MachineImageVersion</a> array
</em>
</td>
<td>
<p>Versions contains versions, expiration dates and container runtimes of the machine image</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#machineimageupdatestrategy">MachineImageUpdateStrategy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdateStrategy is the update strategy to use for the machine image. Possible values are:<br /> - patch: update to the latest patch version of the current minor version.<br /> - minor: update to the latest minor and patch version.<br /> - major: always update to the overall latest version (default).</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageflavor">MachineImageFlavor
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
MachineImageFlavor is a wrapper for Capabilities.
This is a workaround as the Protobuf generator can't handle a slice of maps.
</p>


<h3 id="machineimagestatus">MachineImageStatus
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilestatus">CloudProfileStatus</a>)
</p>

<p>
MachineImageStatus contains the status of a machine image and its version classifications.
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
<p>Name matches the name of the MachineImage the status is represented of.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#expirableversionstatus">ExpirableVersionStatus</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions contains the statuses of the machine image versions.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageupdatestrategy">MachineImageUpdateStrategy
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#machineimage">MachineImage</a>)
</p>

<p>
MachineImageUpdateStrategy is the update strategy to use for a machine image
</p>


<h3 id="machineimageversion">MachineImageVersion
</h3>


<p>
(<em>Appears on:</em><a href="#machineimage">MachineImage</a>)
</p>

<p>
MachineImageVersion is an expirable version with list of supported container runtimes and interfaces
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version identifier.</p>
</td>
</tr>
<tr>
<td>
<code>expirationDate</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationDate defines the time at which this version expires.<br />Deprecated: Is replaced by Lifecycle; mutually exclusive with it.</p>
</td>
</tr>
<tr>
<td>
<code>classification</code></br>
<em>
<a href="#versionclassification">VersionClassification</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Classification defines the state of a version (preview, supported, deprecated).<br />Deprecated: Is replaced by Lifecycle. mutually exclusive with it.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code></br>
<em>
<a href="#lifecyclestage">LifecycleStage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lifecycle defines the lifecycle stages for this version.<br />Mutually exclusive with Classification and ExpirationDate.<br />This can only be used when the VersionClassificationLifecycle feature gate is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>cri</code></br>
<em>
<a href="#cri">CRI</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>CRI list of supported container runtime and interfaces supported by this version</p>
</td>
</tr>
<tr>
<td>
<code>architectures</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architectures is the list of CPU architectures of the machine image in this version.</p>
</td>
</tr>
<tr>
<td>
<code>kubeletVersionConstraint</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeletVersionConstraint is a constraint describing the supported kubelet versions by the machine image in this version.<br />If the field is not specified, it is assumed that the machine image in this version supports all kubelet versions.<br />Examples:<br />- '>= 1.26' - supports only kubelet versions greater than or equal to 1.26<br />- '< 1.26' - supports only kubelet versions less than 1.26</p>
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
<p>InPlaceUpdates contains the configuration for in-place updates for this machine image version.</p>
</td>
</tr>
<tr>
<td>
<code>capabilityFlavors</code></br>
<em>
<a href="#machineimageflavor">MachineImageFlavor</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>CapabilityFlavors is an array of MachineImageFlavor. Each entry represents a combination of capabilities that is provided by<br />the machine image version.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinetype">MachineType
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>, <a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>)
</p>

<p>
MachineType contains certain properties of a machine type.
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
<code>cpu</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<p>CPU is the number of CPUs for this machine type.</p>
</td>
</tr>
<tr>
<td>
<code>gpu</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<p>GPU is the number of GPUs for this machine type.</p>
</td>
</tr>
<tr>
<td>
<code>memory</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<p>Memory is the amount of memory for this machine type.</p>
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
<p>Name is the name of the machine type.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#machinetypestorage">MachineTypeStorage</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage is the amount of storage associated with the root volume of this machine type.</p>
</td>
</tr>
<tr>
<td>
<code>usable</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Usable defines if the machine type can be used for shoot clusters.</p>
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
<p>Architecture is the CPU architecture of this machine type.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capabilities contains the machine type capabilities.</p>
</td>
</tr>
<tr>
<td>
<code>machineControllerManager</code></br>
<em>
<a href="#cloudprofilemachinecontrollermanagersettings">CloudProfileMachineControllerManagerSettings</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineControllerManagerSettings contains a subset of the MachineControllerManagerSettings which can be defaulted for a machine type in a CloudProfile.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinetypestorage">MachineTypeStorage
</h3>


<p>
(<em>Appears on:</em><a href="#machinetype">MachineType</a>)
</p>

<p>
MachineTypeStorage is the amount of storage associated with the root volume of this machine type.
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
<code>class</code></br>
<em>
string
</em>
</td>
<td>
<p>Class is the class of the storage type.</p>
</td>
</tr>
<tr>
<td>
<code>size</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StorageSize is the storage size.</p>
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
<p>Type is the type of the storage.</p>
</td>
</tr>
<tr>
<td>
<code>minSize</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinSize is the minimal supported storage size.<br />This overrides any other common minimum size configuration from `spec.volumeTypes[*].minSize`.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineupdatestrategy">MachineUpdateStrategy
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
MachineUpdateStrategy specifies the machine update strategy for the worker pool.
</p>


<h3 id="maintenance">Maintenance
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Maintenance contains information about the time window for maintenance operations and which
operations should be performed.
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
<code>autoUpdate</code></br>
<em>
<a href="#maintenanceautoupdate">MaintenanceAutoUpdate</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoUpdate contains information about which constraints should be automatically updated.</p>
</td>
</tr>
<tr>
<td>
<code>timeWindow</code></br>
<em>
<a href="#maintenancetimewindow">MaintenanceTimeWindow</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TimeWindow contains information about the time window for maintenance operations.</p>
</td>
</tr>
<tr>
<td>
<code>confineSpecUpdateRollout</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfineSpecUpdateRollout prevents that changes/updates to the shoot specification will be rolled out immediately.<br />Instead, they are rolled out during the shoot's maintenance time window. There is one exception that will trigger<br />an immediate roll out which is changes to the Spec.Hibernation.Enabled field.</p>
</td>
</tr>
<tr>
<td>
<code>autoRotation</code></br>
<em>
<a href="#maintenanceautorotation">MaintenanceAutoRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoRotation contains information about which rotations should be automatically performed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="maintenanceautorotation">MaintenanceAutoRotation
</h3>


<p>
(<em>Appears on:</em><a href="#maintenance">Maintenance</a>)
</p>

<p>
MaintenanceAutoRotation contains information about which rotations should be automatically performed.
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
<code>credentials</code></br>
<em>
<a href="#maintenancecredentialsautorotation">MaintenanceCredentialsAutoRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credentials contains information about which credentials should be automatically rotated.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="maintenanceautoupdate">MaintenanceAutoUpdate
</h3>


<p>
(<em>Appears on:</em><a href="#maintenance">Maintenance</a>)
</p>

<p>
MaintenanceAutoUpdate contains information about which constraints should be automatically updated.
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
<code>kubernetesVersion</code></br>
<em>
boolean
</em>
</td>
<td>
<p>KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated (default: true).</p>
</td>
</tr>
<tr>
<td>
<code>machineImageVersion</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImageVersion indicates whether the machine image version may be automatically updated (default: true).</p>
</td>
</tr>

</tbody>
</table>


<h3 id="maintenancecredentialsautorotation">MaintenanceCredentialsAutoRotation
</h3>


<p>
(<em>Appears on:</em><a href="#maintenanceautorotation">MaintenanceAutoRotation</a>)
</p>

<p>
MaintenanceCredentialsAutoRotation contains information about which credentials should be automatically rotated.
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
<code>observability</code></br>
<em>
<a href="#maintenancerotationconfig">MaintenanceRotationConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Observability configures the automatic rotation for the observability credentials.</p>
</td>
</tr>
<tr>
<td>
<code>sshKeypair</code></br>
<em>
<a href="#maintenancerotationconfig">MaintenanceRotationConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SSHKeypair configures the automatic rotation for the ssh keypair for worker nodes.</p>
</td>
</tr>
<tr>
<td>
<code>etcdEncryptionKey</code></br>
<em>
<a href="#maintenancerotationconfig">MaintenanceRotationConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCDEncryptionKey configures the automatic rotation for the etcd encryption key.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="maintenancerotationconfig">MaintenanceRotationConfig
</h3>


<p>
(<em>Appears on:</em><a href="#maintenancecredentialsautorotation">MaintenanceCredentialsAutoRotation</a>)
</p>

<p>
MaintenanceRotationConfig contains configuration for automatic rotation.
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
<code>rotationPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RotationPeriod is the period between a completed rotation and the start of a new rotation (default: 7d).<br />The allowed rotation period is between 30m and 90d. When set to 0, rotation is disabled.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="maintenancetimewindow">MaintenanceTimeWindow
</h3>


<p>
(<em>Appears on:</em><a href="#maintenance">Maintenance</a>)
</p>

<p>
MaintenanceTimeWindow contains information about the time window for maintenance operations.
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
<code>begin</code></br>
<em>
string
</em>
</td>
<td>
<p>Begin is the beginning of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".<br />If not present, a random value will be computed.</p>
</td>
</tr>
<tr>
<td>
<code>end</code></br>
<em>
string
</em>
</td>
<td>
<p>End is the end of the time window in the format HHMMSS+ZONE, e.g. "220000+0100".<br />If not present, the value will be computed based on the "Begin" value.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="manualworkerpoolrollout">ManualWorkerPoolRollout
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatus">ShootStatus</a>)
</p>

<p>
ManualWorkerPoolRollout contains information about the worker pool rollout progress that has been initiated via the gardener.cloud/operation=rollout-workers annotation.
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
<code>pendingWorkersRollouts</code></br>
<em>
<a href="#pendingworkersrollout">PendingWorkersRollout</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkersRollouts contains the names of the worker pools that are still pending rollout.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="memoryswapconfiguration">MemorySwapConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#kubeletconfig">KubeletConfig</a>)
</p>

<p>
MemorySwapConfiguration contains kubelet swap configuration
For more information, please see KEP: 2400-node-swap
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
<code>swapBehavior</code></br>
<em>
<a href="#swapbehavior">SwapBehavior</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SwapBehavior configures swap memory available to container workloads. May be one of \{"NoSwap", "LimitedSwap"\}<br />defaults to: LimitedSwap</p>
</td>
</tr>

</tbody>
</table>


<h3 id="monitoring">Monitoring
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Monitoring contains information about the monitoring configuration for the shoot.
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
<code>alerting</code></br>
<em>
<a href="#alerting">Alerting</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alerting contains information about the alerting configuration for the shoot cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="namedresourcereference">NamedResourceReference
</h3>


<p>
(<em>Appears on:</em><a href="#extensionresourcestate">ExtensionResourceState</a>, <a href="#seedspec">SeedSpec</a>, <a href="#shootspec">ShootSpec</a>)
</p>

<p>
NamedResourceReference is a named reference to a resource.
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
<p>Name of the resource reference.</p>
</td>
</tr>
<tr>
<td>
<code>resourceRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#crossversionobjectreference-v1-autoscaling">CrossVersionObjectReference</a>
</em>
</td>
<td>
<p>ResourceRef is a reference to a resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="namespacedcloudprofile">NamespacedCloudProfile
</h3>


<p>
NamespacedCloudProfile represents certain properties about a provider environment.
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
<a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>
</em>
</td>
<td>
<p>Spec defines the provider environment properties.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#namespacedcloudprofilestatus">NamespacedCloudProfileStatus</a>
</em>
</td>
<td>
<p>Most recently observed status of the NamespacedCloudProfile.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="namespacedcloudprofilespec">NamespacedCloudProfileSpec
</h3>


<p>
(<em>Appears on:</em><a href="#namespacedcloudprofile">NamespacedCloudProfile</a>)
</p>

<p>
NamespacedCloudProfileSpec is the specification of a NamespacedCloudProfile.
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
<code>caBundle</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#kubernetessettings">KubernetesSettings</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#machineimage">MachineImage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineTypes</code></br>
<em>
<a href="#machinetype">MachineType</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>volumeTypes</code></br>
<em>
<a href="#volumetype">VolumeType</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>parent</code></br>
<em>
<a href="#cloudprofilereference">CloudProfileReference</a>
</em>
</td>
<td>
<p>Parent contains a reference to a CloudProfile it inherits from.</p>
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
<p>ProviderConfig contains provider-specific configuration for the profile.</p>
</td>
</tr>
<tr>
<td>
<code>limits</code></br>
<em>
<a href="#limits">Limits</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits configures operational limits for Shoot clusters using this NamespacedCloudProfile.<br />Any limits specified here override those set in the parent CloudProfile.<br />See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="namespacedcloudprofilestatus">NamespacedCloudProfileStatus
</h3>


<p>
(<em>Appears on:</em><a href="#namespacedcloudprofile">NamespacedCloudProfile</a>)
</p>

<p>
NamespacedCloudProfileStatus holds the most recently observed status of the NamespacedCloudProfile.
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
<code>cloudProfileSpec</code></br>
<em>
<a href="#cloudprofilespec">CloudProfileSpec</a>
</em>
</td>
<td>
<p>CloudProfile is the most recently generated CloudProfile of the NamespacedCloudProfile.</p>
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
<p>ObservedGeneration is the most recent generation observed for this NamespacedCloudProfile.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networking">Networking
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Networking defines networking parameters for the shoot cluster.
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
<em>(Optional)</em>
<p>Type identifies the type of the networking plugin. This field is immutable.</p>
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
<p>ProviderConfig is the configuration passed to network resource.</p>
</td>
</tr>
<tr>
<td>
<code>pods</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Pods is the CIDR of the pod network. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>nodes</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes is the CIDR of the entire node network.<br />This field is mutable.</p>
</td>
</tr>
<tr>
<td>
<code>services</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Services is the CIDR of the service network. This field is immutable.</p>
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
<p>IPFamilies specifies the IP protocol versions to use for shoot networking.<br />See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md.<br />Defaults to ["IPv4"].</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networkingstatus">NetworkingStatus
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatus">ShootStatus</a>)
</p>

<p>
NetworkingStatus contains information about cluster networking such as CIDRs.
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
<tr>
<td>
<code>egressCIDRs</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>EgressCIDRs is a list of CIDRs used by the shoot as the source IP for egress traffic as reported by the used<br />Infrastructure extension controller. For certain environments the egress IPs may not be stable in which case the<br />extension controller may opt to not populate this field.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="nginxingress">NginxIngress
</h3>


<p>
(<em>Appears on:</em><a href="#addons">Addons</a>)
</p>

<p>
NginxIngress describes configuration values for the nginx-ingress addon.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled indicates whether the addon is enabled or not.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerSourceRanges</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerSourceRanges is list of allowed IP sources for NginxIngress</p>
</td>
</tr>
<tr>
<td>
<code>config</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config contains custom configuration for the nginx-ingress-controller configuration.<br />See https://github.com/kubernetes/ingress-nginx/blob/master/docs/user-guide/nginx-configuration/configmap.md#configuration-options</p>
</td>
</tr>
<tr>
<td>
<code>externalTrafficPolicy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#serviceexternaltrafficpolicy-v1-core">ServiceExternalTrafficPolicy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExternalTrafficPolicy controls the `.spec.externalTrafficPolicy` value of the load balancer `Service`<br />exposing the nginx-ingress. Defaults to `Cluster`.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="nodelocaldns">NodeLocalDNS
</h3>


<p>
(<em>Appears on:</em><a href="#systemcomponents">SystemComponents</a>)
</p>

<p>
NodeLocalDNS contains the settings of the node local DNS components running in the data plane of the Shoot cluster.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled indicates whether node local DNS is enabled or not.</p>
</td>
</tr>
<tr>
<td>
<code>forceTCPToClusterDNS</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceTCPToClusterDNS indicates whether the connection from the node local DNS to the cluster DNS (Core DNS) will be forced to TCP or not.<br />Default, if unspecified, is to enforce TCP.</p>
</td>
</tr>
<tr>
<td>
<code>forceTCPToUpstreamDNS</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceTCPToUpstreamDNS indicates whether the connection from the node local DNS to the upstream DNS (infrastructure DNS) will be forced to TCP or not.<br />Default, if unspecified, is to enforce TCP.</p>
</td>
</tr>
<tr>
<td>
<code>disableForwardToUpstreamDNS</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableForwardToUpstreamDNS indicates whether requests from node local DNS to upstream DNS should be disabled.<br />Default, if unspecified, is to forward requests for external domains to upstream DNS</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ocirepository">OCIRepository
</h3>


<p>
(<em>Appears on:</em><a href="#helmcontrollerdeployment">HelmControllerDeployment</a>)
</p>

<p>
OCIRepository configures where to pull an OCI Artifact, that could contain for example a Helm Chart.
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
<code>ref</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ref is the full artifact Ref and takes precedence over all other fields.</p>
</td>
</tr>
<tr>
<td>
<code>repository</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Repository is a reference to an OCI artifact repository.</p>
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
<p>Tag is the image tag to pull.</p>
</td>
</tr>
<tr>
<td>
<code>digest</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Digest of the image to pull, takes precedence over tag.</p>
</td>
</tr>
<tr>
<td>
<code>pullSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PullSecretRef is a reference to a secret containing the pull secret.<br />The secret must be of type `kubernetes.io/dockerconfigjson` and must be located in the `garden` namespace.</p>
</td>
</tr>
<tr>
<td>
<code>caBundleSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CABundleSecretRef is a reference to a secret containing a PEM-encoded certificate authority bundle.<br />The CA bundle is used to verify the TLS certificate of the OCI registry.<br />The secret must have a data key `bundle.crt` and must be located in the `garden` namespace.<br />For usage in the gardenlet, the secret must have the label `gardener.cloud/role=oci-ca-bundle`.<br />If not provided, the system's default certificate pool is used.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="oidcconfig">OIDCConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
OIDCConfig contains configuration settings for the OIDC provider.
Note: Descriptions were taken from the Kubernetes documentation.
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
<code>caBundle</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.</p>
</td>
</tr>
<tr>
<td>
<code>clientID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The client ID for the OpenID Connect client, must be set.</p>
</td>
</tr>
<tr>
<td>
<code>groupsClaim</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.</p>
</td>
</tr>
<tr>
<td>
<code>groupsPrefix</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.</p>
</td>
</tr>
<tr>
<td>
<code>issuerURL</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The URL of the OpenID issuer, only HTTPS scheme will be accepted. Used to verify the OIDC JSON Web Token (JWT).</p>
</td>
</tr>
<tr>
<td>
<code>requiredClaims</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.</p>
</td>
</tr>
<tr>
<td>
<code>signingAlgs</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1</p>
</td>
</tr>
<tr>
<td>
<code>usernameClaim</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")</p>
</td>
</tr>
<tr>
<td>
<code>usernamePrefix</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="observabilityrotation">ObservabilityRotation
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentialsrotation">ShootCredentialsRotation</a>)
</p>

<p>
ObservabilityRotation contains information about the observability credential rotation.
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
<p>LastInitiationTime is the most recent time when the observability credential rotation was initiated.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the observability credential rotation was successfully completed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="openidconnectclientauthentication">OpenIDConnectClientAuthentication
</h3>


<p>
OpenIDConnectClientAuthentication contains configuration for OIDC clients.
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
<code>extraConfig</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extra configuration added to kubeconfig's auth-provider.<br />Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token</p>
</td>
</tr>
<tr>
<td>
<code>secret</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The client Secret for the OpenID Connect client.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="pendingworkerupdates">PendingWorkerUpdates
</h3>


<p>
(<em>Appears on:</em><a href="#inplaceupdatesstatus">InPlaceUpdatesStatus</a>)
</p>

<p>
PendingWorkerUpdates contains information about worker pools pending in-place update.
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
<code>autoInPlaceUpdate</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AutoInPlaceUpdate contains the names of the pending worker pools with strategy AutoInPlaceUpdate.</p>
</td>
</tr>
<tr>
<td>
<code>manualInPlaceUpdate</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManualInPlaceUpdate contains the names of the pending worker pools with strategy ManualInPlaceUpdate.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="pendingworkersrollout">PendingWorkersRollout
</h3>


<p>
(<em>Appears on:</em><a href="#carotation">CARotation</a>, <a href="#manualworkerpoolrollout">ManualWorkerPoolRollout</a>, <a href="#serviceaccountkeyrotation">ServiceAccountKeyRotation</a>)
</p>

<p>
PendingWorkersRollout contains the name of a worker pool and the initiation time of their last rollout.
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
<p>Name is the name of a worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationTime is the most recent time when the worker rollout was initiated.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="project">Project
</h3>


<p>
Project holds certain properties about a Gardener project.
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
<a href="#projectspec">ProjectSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the project properties.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#projectstatus">ProjectStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Project.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="projectmember">ProjectMember
</h3>


<p>
(<em>Appears on:</em><a href="#projectspec">ProjectSpec</a>)
</p>

<p>
ProjectMember is a member of a project.
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
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind of object being referenced. Values defined by this API group are "User", "Group", and "ServiceAccount".<br />If the Authorizer does not recognized the kind value, the Authorizer should report an error.</p>
</td>
</tr>
<tr>
<td>
<code>apiGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIGroup holds the API group of the referenced subject.<br />Defaults to "" for ServiceAccount subjects.<br />Defaults to "rbac.authorization.k8s.io" for User and Group subjects.</p>
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
<p>Name of the object being referenced.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Namespace of the referenced object.  If the object kind is non-namespace, such as "User" or "Group", and this value is not empty<br />the Authorizer should report an error.</p>
</td>
</tr>
<tr>
<td>
<code>role</code></br>
<em>
string
</em>
</td>
<td>
<p>Role represents the role of this member.<br />IMPORTANT: Be aware that this field will be removed in the `v1` version of this API in favor of the `roles`<br />list.</p>
</td>
</tr>
<tr>
<td>
<code>roles</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles represents the list of roles of this member.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="projectphase">ProjectPhase
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#projectstatus">ProjectStatus</a>)
</p>

<p>
ProjectPhase is a label for the condition of a project at the current time.
</p>


<h3 id="projectspec">ProjectSpec
</h3>


<p>
(<em>Appears on:</em><a href="#project">Project</a>)
</p>

<p>
ProjectSpec is the specification of a Project.
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
<code>createdBy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#subject-v1-rbac">Subject</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CreatedBy is a subject representing a user name, an email address, or any other identifier of a user<br />who created the project. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>description</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description is a human-readable description of what the project is used for.<br />Only letters, digits and certain punctuation characters are allowed for this field.</p>
</td>
</tr>
<tr>
<td>
<code>owner</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#subject-v1-rbac">Subject</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Owner is a subject representing a user name, an email address, or any other identifier of a user owning<br />the project.<br />IMPORTANT: Be aware that this field will be removed in the `v1` version of this API in favor of the `owner`<br />role. The only way to change the owner will be by moving the `owner` role. In this API version the only way<br />to change the owner is to use this field.</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Purpose is a human-readable explanation of the project's purpose.<br />Only letters, digits and certain punctuation characters are allowed for this field.</p>
</td>
</tr>
<tr>
<td>
<code>members</code></br>
<em>
<a href="#projectmember">ProjectMember</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Members is a list of subjects representing a user name, an email address, or any other identifier of a user,<br />group, or service account that has a certain role.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Namespace is the name of the namespace that has been created for the Project object.<br />A nil value means that Gardener will determine the name of the namespace.<br />If set, its value must be prefixed with `garden-`.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="#projecttolerations">ProjectTolerations</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations contains the tolerations for taints on seed clusters.</p>
</td>
</tr>
<tr>
<td>
<code>dualApprovalForDeletion</code></br>
<em>
<a href="#dualapprovalfordeletion">DualApprovalForDeletion</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="projectstatus">ProjectStatus
</h3>


<p>
(<em>Appears on:</em><a href="#project">Project</a>)
</p>

<p>
ProjectStatus holds the most recently observed status of the project.
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
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this project.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code></br>
<em>
<a href="#projectphase">ProjectPhase</a>
</em>
</td>
<td>
<p>Phase is the current phase of the project.</p>
</td>
</tr>
<tr>
<td>
<code>staleSinceTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StaleSinceTimestamp contains the timestamp when the project was first discovered to be stale/unused.</p>
</td>
</tr>
<tr>
<td>
<code>staleAutoDeleteTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StaleAutoDeleteTimestamp contains the timestamp when the project will be garbage-collected/automatically deleted<br />because it's stale/unused.</p>
</td>
</tr>
<tr>
<td>
<code>lastActivityTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastActivityTimestamp contains the timestamp from the last activity performed in this project.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="#condition">Condition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Project's current state.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="projecttolerations">ProjectTolerations
</h3>


<p>
(<em>Appears on:</em><a href="#projectspec">ProjectSpec</a>)
</p>

<p>
ProjectTolerations contains the tolerations for taints on seed clusters.
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
<code>defaults</code></br>
<em>
<a href="#toleration">Toleration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defaults contains a list of tolerations that are added to the shoots in this project by default.</p>
</td>
</tr>
<tr>
<td>
<code>whitelist</code></br>
<em>
<a href="#toleration">Toleration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whitelist contains a list of tolerations that are allowed to be added to the shoots in this project. Please note<br />that this list may only be added by users having the `spec-tolerations-whitelist` verb for project resources.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="provider">Provider
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
Provider contains provider-specific information that are handed-over to the provider-specific
extension controller.
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
<p>Type is the type of the provider. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>controlPlaneConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControlPlaneConfig contains the provider-specific control plane config blob. Please look up the concrete<br />definition in the documentation of your provider extension.</p>
</td>
</tr>
<tr>
<td>
<code>infrastructureConfig</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InfrastructureConfig contains the provider-specific infrastructure config blob. Please look up the concrete<br />definition in the documentation of your provider extension.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#worker">Worker</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workers is a list of worker groups.</p>
</td>
</tr>
<tr>
<td>
<code>workersSettings</code></br>
<em>
<a href="#workerssettings">WorkersSettings</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkersSettings contains settings for all workers.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="proxymode">ProxyMode
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#kubeproxyconfig">KubeProxyConfig</a>)
</p>

<p>
ProxyMode available in Linux platform: 'userspace' (older, going to be EOL), 'iptables'
(newer, faster), 'nftables', and 'ipvs' (deprecated starting with Kubernetes 1.35).
As of now only 'iptables', 'nftables' and 'ipvs' (deprecated starting with Kubernetes 1.35) is supported by Gardener.
In Linux platform, if the iptables proxy is selected, regardless of how, but the system's kernel or iptables versions are
insufficient, this always falls back to the userspace proxy.
</p>


<h3 id="quota">Quota
</h3>


<p>
Quota represents a quota on resources consumed by shoot clusters either per project or per provider secret.
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
<a href="#quotaspec">QuotaSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the Quota constraints.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="quotaspec">QuotaSpec
</h3>


<p>
(<em>Appears on:</em><a href="#quota">Quota</a>)
</p>

<p>
QuotaSpec is the specification of a Quota.
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
<code>clusterLifetimeDays</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.</p>
</td>
</tr>
<tr>
<td>
<code>scope</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a>
</em>
</td>
<td>
<p>Scope is the scope of the Quota object, either 'project', 'secret' or 'workloadidentity'. This field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="region">Region
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>)
</p>

<p>
Region contains certain properties of a region.
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
<p>Name is a region name.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#availabilityzone">AvailabilityZone</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones is a list of availability zones in this region.</p>
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
<p>Labels is an optional set of key-value pairs that contain certain administrator-controlled labels for this region.<br />It can be used by Gardener administrators/operators to provide additional information about a region, e.g. wrt<br />quality, reliability, etc.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#accessrestriction">AccessRestriction</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions that can be used for Shoots using this region.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="resourcedata">ResourceData
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatespec">ShootStateSpec</a>)
</p>

<p>
ResourceData holds the data of a resource referred to by an extension controller state.
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
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>kind is the kind of the referent; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds</p>
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
<p>name is the name of the referent; More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names</p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>apiVersion is the API version of the referent</p>
</td>
</tr>
<tr>
<td>
<code>data</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<p>Data of the resource</p>
</td>
</tr>

</tbody>
</table>


<h3 id="resourcewatchcachesize">ResourceWatchCacheSize
</h3>


<p>
(<em>Appears on:</em><a href="#watchcachesizes">WatchCacheSizes</a>)
</p>

<p>
ResourceWatchCacheSize contains configuration of the API server's watch cache size for one specific resource.
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
<code>apiGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIGroup is the API group of the resource for which the watch cache size should be configured.<br />An unset value is used to specify the legacy core API (e.g. for `secrets`).</p>
</td>
</tr>
<tr>
<td>
<code>resource</code></br>
<em>
string
</em>
</td>
<td>
<p>Resource is the name of the resource for which the watch cache size should be configured<br />(in lowercase plural form, e.g. `secrets`).</p>
</td>
</tr>
<tr>
<td>
<code>size</code></br>
<em>
integer
</em>
</td>
<td>
<p>CacheSize specifies the watch cache size that should be configured for the specified resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="sshaccess">SSHAccess
</h3>


<p>
(<em>Appears on:</em><a href="#workerssettings">WorkersSettings</a>)
</p>

<p>
SSHAccess contains settings regarding ssh access to the worker nodes.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled indicates whether the SSH access to the worker nodes is ensured to be enabled or disabled in systemd.<br />Defaults to true.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="schedulingprofile">SchedulingProfile
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#kubeschedulerconfig">KubeSchedulerConfig</a>)
</p>

<p>
SchedulingProfile is a string alias used for scheduling profile values.
</p>


<h3 id="secretbinding">SecretBinding
</h3>


<p>
SecretBinding represents a binding to a secret in the same or another namespace.

Deprecated: Use CredentialsBinding instead. See https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md for migration instructions.
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
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretreference-v1-core">SecretReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret object in the same or another namespace.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>quotas</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectreference-v1-core">ObjectReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Quotas is a list of references to Quota objects in the same or another namespace.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
<a href="#secretbindingprovider">SecretBindingProvider</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider defines the provider type of the SecretBinding.<br />This field is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="secretbindingprovider">SecretBindingProvider
</h3>


<p>
(<em>Appears on:</em><a href="#secretbinding">SecretBinding</a>)
</p>

<p>
SecretBindingProvider defines the provider type of the SecretBinding.

Deprecated: Use CredentialsBindingProvider instead. See https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md for migration instructions.
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
<p>Type is the type of the provider.<br />For backwards compatibility, the field can contain multiple providers separated by a comma.<br />However the usage of single SecretBinding (hence Secret) for different cloud providers is strongly discouraged.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seed">Seed
</h3>


<p>
Seed represents an installation request for an external controller.
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
<a href="#seedspec">SeedSpec</a>
</em>
</td>
<td>
<p>Spec contains the specification of this installation.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#seedstatus">SeedStatus</a>
</em>
</td>
<td>
<p>Status contains the status of this installation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seeddns">SeedDNS
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
SeedDNS contains DNS-relevant information about this seed cluster.
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
<code>provider</code></br>
<em>
<a href="#seeddnsprovider">SeedDNSProvider</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider configures a DNSProvider</p>
</td>
</tr>
<tr>
<td>
<code>internal</code></br>
<em>
<a href="#seeddnsproviderconfig">SeedDNSProviderConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Internal configures DNS settings related to seed internal domain.</p>
</td>
</tr>
<tr>
<td>
<code>defaults</code></br>
<em>
<a href="#seeddnsproviderconfig">SeedDNSProviderConfig</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Defaults configures DNS settings related to seed default domains.<br />When determining the DNS settings for a Shoot, the first matching entry in this list will take precedence.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seeddnsprovider">SeedDNSProvider
</h3>


<p>
(<em>Appears on:</em><a href="#seeddns">SeedDNS</a>)
</p>

<p>
SeedDNSProvider configures a DNSProvider for Seeds
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
<p>Type describes the type of the dns-provider, for example `aws-route53`</p>
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
<p>CredentialsRef is a reference to a resource holding the credentials used for<br />authentication with the DNS provider.<br />Supported referenced resources are v1.Secrets and<br />security.gardener.cloud/v1alpha1.WorkloadIdentity</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seeddnsproviderconfig">SeedDNSProviderConfig
</h3>


<p>
(<em>Appears on:</em><a href="#seeddns">SeedDNS</a>)
</p>

<p>
SeedDNSProviderConfig configures a DNS provider.
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
<p>Type is the type of the DNS provider.</p>
</td>
</tr>
<tr>
<td>
<code>domain</code></br>
<em>
string
</em>
</td>
<td>
<p>Domain is the domain name to be used by the DNS provider.</p>
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
<p>Zone is the zone where the DNS records are managed.</p>
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
<p>CredentialsRef is a reference to a resource holding the credentials used for<br />authentication with the DNS provider.<br />Supported referenced resources are v1.Secrets and<br />security.gardener.cloud/v1alpha1.WorkloadIdentity</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seednetworks">SeedNetworks
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
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
<code>nodes</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes is the CIDR of the node network. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>pods</code></br>
<em>
string
</em>
</td>
<td>
<p>Pods is the CIDR of the pod network. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>services</code></br>
<em>
string
</em>
</td>
<td>
<p>Services is the CIDR of the service network. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>shootDefaults</code></br>
<em>
<a href="#shootnetworks">ShootNetworks</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootDefaults contains the default networks CIDRs for shoots.</p>
</td>
</tr>
<tr>
<td>
<code>blockCIDRs</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>BlockCIDRs is a list of network addresses that should be blocked for shoot control plane components running<br />in the seed cluster.</p>
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
<p>IPFamilies specifies the IP protocol versions to use for seed networking. This field is immutable.<br />See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md.<br />Defaults to ["IPv4"].</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedprovider">SeedProvider
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
SeedProvider defines the provider-specific information of this Seed cluster.
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
<p>Type is the name of the provider.</p>
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
<p>ProviderConfig is the configuration passed to Seed resource.</p>
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
<p>Region is a name of a region.</p>
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
<p>Zones is the list of availability zones the seed cluster is deployed to.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedselector">SeedSelector
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>, <a href="#exposureclassscheduling">ExposureClassScheduling</a>, <a href="#shootspec">ShootSpec</a>)
</p>

<p>
SeedSelector contains constraints for selecting seed to be usable for shoots using a profile
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
<code>matchLabels</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>matchLabels is a map of \{key,value\} pairs. A single \{key,value\} in the matchLabels<br />map is equivalent to an element of matchExpressions, whose key field is "key", the<br />operator is "In", and the values array contains only "value". The requirements are ANDed.</p>
</td>
</tr>
<tr>
<td>
<code>matchExpressions</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselectorrequirement-v1-meta">LabelSelectorRequirement</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>matchExpressions is a list of label selector requirements. The requirements are ANDed.</p>
</td>
</tr>
<tr>
<td>
<code>providerTypes</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Providers is optional and can be used by restricting seeds by their provider type. '*' can be used to enable seeds regardless of their provider type.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingdependencywatchdog">SeedSettingDependencyWatchdog
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingDependencyWatchdog controls the dependency-watchdog settings for the seed.
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
<code>weeder</code></br>
<em>
<a href="#seedsettingdependencywatchdogweeder">SeedSettingDependencyWatchdogWeeder</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Weeder controls the weeder settings for the dependency-watchdog for the seed.</p>
</td>
</tr>
<tr>
<td>
<code>prober</code></br>
<em>
<a href="#seedsettingdependencywatchdogprober">SeedSettingDependencyWatchdogProber</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Prober controls the prober settings for the dependency-watchdog for the seed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingdependencywatchdogprober">SeedSettingDependencyWatchdogProber
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettingdependencywatchdog">SeedSettingDependencyWatchdog</a>)
</p>

<p>
SeedSettingDependencyWatchdogProber controls the prober settings for the dependency-watchdog for the seed.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled controls whether the probe controller(prober) of the dependency-watchdog should be enabled. This controller<br />scales down the kube-controller-manager, machine-controller-manager and cluster-autoscaler of shoot clusters in case their respective kube-apiserver is not<br />reachable via its external ingress in order to avoid melt-down situations.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingdependencywatchdogweeder">SeedSettingDependencyWatchdogWeeder
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettingdependencywatchdog">SeedSettingDependencyWatchdog</a>)
</p>

<p>
SeedSettingDependencyWatchdogWeeder controls the weeder settings for the dependency-watchdog for the seed.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled controls whether the endpoint controller(weeder) of the dependency-watchdog should be enabled. This controller<br />helps to alleviate the delay where control plane components remain unavailable by finding the respective pods in<br />CrashLoopBackoff status and restarting them once their dependants become ready and available again.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingexcesscapacityreservation">SeedSettingExcessCapacityReservation
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enabled controls whether the default excess capacity reservation should be enabled. When not specified, the functionality is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>configs</code></br>
<em>
<a href="#seedsettingexcesscapacityreservationconfig">SeedSettingExcessCapacityReservationConfig</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configs configures excess capacity reservation deployments for shoot control planes in the seed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingexcesscapacityreservationconfig">SeedSettingExcessCapacityReservationConfig
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettingexcesscapacityreservation">SeedSettingExcessCapacityReservation</a>)
</p>

<p>
SeedSettingExcessCapacityReservationConfig configures excess capacity reservation deployments for shoot control planes in the seed.
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
<code>nodeSelector</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeSelector specifies the node where the excess-capacity-reservation pod should run.</p>
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
<p>Tolerations specify the tolerations for the the excess-capacity-reservation pod.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingloadbalancerservices">SeedSettingLoadBalancerServices
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
seed.
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
<code>annotations</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations is a map of annotations that will be injected/merged into every load balancer service object.</p>
</td>
</tr>
<tr>
<td>
<code>externalTrafficPolicy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#serviceexternaltrafficpolicy-v1-core">ServiceExternalTrafficPolicy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExternalTrafficPolicy describes how nodes distribute service traffic they<br />receive on one of the service's "externally-facing" addresses.<br />Defaults to "Cluster".</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#seedsettingloadbalancerserviceszones">SeedSettingLoadBalancerServicesZones</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones controls settings, which are specific to the single-zone load balancers in a multi-zonal setup.<br />Can be empty for single-zone seeds. Each specified zone has to relate to one of the zones in seed.spec.provider.zones.</p>
</td>
</tr>
<tr>
<td>
<code>proxyProtocol</code></br>
<em>
<a href="#loadbalancerservicesproxyprotocol">LoadBalancerServicesProxyProtocol</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.<br />Defaults to nil, which is equivalent to not allowing ProxyProtocol.</p>
</td>
</tr>
<tr>
<td>
<code>zonalIngress</code></br>
<em>
<a href="#seedsettingloadbalancerserviceszonalingress">SeedSettingLoadBalancerServicesZonalIngress</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ZonalIngress controls whether ingress gateways are deployed per availability zone.<br />Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>class</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class configures the Service.spec.loadBalancerClass field for the load balancer services on the seed.<br />Note that changing the loadBalancerClass of existing LoadBalancer services is denied by Kubernetes.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingloadbalancerserviceszonalingress">SeedSettingLoadBalancerServicesZonalIngress
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettingloadbalancerservices">SeedSettingLoadBalancerServices</a>)
</p>

<p>
SeedSettingLoadBalancerServicesZonalIngress controls the deployment of ingress gateways per availability zone.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enabled controls whether seed ingress gateways are deployed in each availability zone.<br />Defaults to true, which provisions an ingress gateway load balancer for each availability zone.<br />When disabled, only a single ingress gateway is deployed.<br />See https://github.com/gardener/gardener/blob/master/docs/operations/seed_settings.md#zonal-ingress.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingloadbalancerserviceszones">SeedSettingLoadBalancerServicesZones
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettingloadbalancerservices">SeedSettingLoadBalancerServices</a>)
</p>

<p>
SeedSettingLoadBalancerServicesZones controls settings, which are specific to the single-zone load balancers in a
multi-zonal setup.
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
<p>Name is the name of the zone as specified in seed.spec.provider.zones.</p>
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
<p>Annotations is a map of annotations that will be injected/merged into the zone-specific load balancer service object.</p>
</td>
</tr>
<tr>
<td>
<code>externalTrafficPolicy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#serviceexternaltrafficpolicy-v1-core">ServiceExternalTrafficPolicy</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExternalTrafficPolicy describes how nodes distribute service traffic they<br />receive on one of the service's "externally-facing" addresses.<br />Defaults to "Cluster".</p>
</td>
</tr>
<tr>
<td>
<code>proxyProtocol</code></br>
<em>
<a href="#loadbalancerservicesproxyprotocol">LoadBalancerServicesProxyProtocol</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.<br />Defaults to nil, which is equivalent to not allowing ProxyProtocol.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingscheduling">SeedSettingScheduling
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingScheduling controls settings for scheduling decisions for the seed.
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
<code>visible</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Visible controls whether the gardener-scheduler shall consider this seed when scheduling shoots. Invisible seeds<br />are not considered by the scheduler.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingtopologyawarerouting">SeedSettingTopologyAwareRouting
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled controls whether certain Services deployed in the seed cluster should be topology-aware.<br />These Services are etcd-main-client, etcd-events-client, kube-apiserver, gardener-resource-manager and vpa-webhook.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingverticalpodautoscaler">SeedSettingVerticalPodAutoscaler
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
seed.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled controls whether the VPA components shall be deployed into the garden namespace in the seed cluster. It<br />is enabled by default because Gardener heavily relies on a VPA being deployed. You should only disable this if<br />your seed cluster already has another, manually/custom managed VPA deployment.</p>
</td>
</tr>
<tr>
<td>
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettingzoneselection">SeedSettingZoneSelection
</h3>


<p>
(<em>Appears on:</em><a href="#seedsettings">SeedSettings</a>)
</p>

<p>
SeedSettingZoneSelection controls whether shoot control plane zone placement is derived
from the shoot's worker pool zones rather than randomly selected from seed zones.
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
<code>mode</code></br>
<em>
<a href="#zoneselectionmode">ZoneSelectionMode</a>
</em>
</td>
<td>
<p>Mode controls the zone selection behavior.<br />"Prefer" tries to match worker pool zones to seed zones, falling back to random selection on mismatch.<br />"Enforce" requires worker pool zones to be present in the seed's zone list; scheduling fails otherwise.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedsettings">SeedSettings
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
SeedSettings contains certain settings for this seed cluster.
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
<code>excessCapacityReservation</code></br>
<em>
<a href="#seedsettingexcesscapacityreservation">SeedSettingExcessCapacityReservation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.</p>
</td>
</tr>
<tr>
<td>
<code>scheduling</code></br>
<em>
<a href="#seedsettingscheduling">SeedSettingScheduling</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Scheduling controls settings for scheduling decisions for the seed.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerServices</code></br>
<em>
<a href="#seedsettingloadbalancerservices">SeedSettingLoadBalancerServices</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerServices controls certain settings for services of type load balancer that are created in the seed.</p>
</td>
</tr>
<tr>
<td>
<code>verticalPodAutoscaler</code></br>
<em>
<a href="#seedsettingverticalpodautoscaler">SeedSettingVerticalPodAutoscaler</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the seed.</p>
</td>
</tr>
<tr>
<td>
<code>dependencyWatchdog</code></br>
<em>
<a href="#seedsettingdependencywatchdog">SeedSettingDependencyWatchdog</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DependencyWatchdog controls certain settings for the dependency-watchdog components deployed in the seed.</p>
</td>
</tr>
<tr>
<td>
<code>topologyAwareRouting</code></br>
<em>
<a href="#seedsettingtopologyawarerouting">SeedSettingTopologyAwareRouting</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.<br />See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.</p>
</td>
</tr>
<tr>
<td>
<code>zoneSelection</code></br>
<em>
<a href="#seedsettingzoneselection">SeedSettingZoneSelection</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ZoneSelection controls whether shoot control plane zone placement is derived from the shoot's worker pool zones<br />rather than randomly selected from seed zones.<br />See https://github.com/gardener/gardener/blob/master/docs/operations/seed_settings.md#zone-selection.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedspec">SeedSpec
</h3>


<p>
(<em>Appears on:</em><a href="#seed">Seed</a>, <a href="#seedtemplate">SeedTemplate</a>)
</p>

<p>
SeedSpec is the specification of a Seed.
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
<code>backup</code></br>
<em>
<a href="#backup">Backup</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup holds the object store configuration for the backups of shoot (currently only etcd).<br />If it is not specified, then there won't be any backups taken for shoots associated with this seed.<br />If backup field is present in seed, then backups of the etcd from shoot control plane will be stored<br />under the configured object store.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#seeddns">SeedDNS</a>
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
<a href="#seednetworks">SeedNetworks</a>
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
<a href="#seedprovider">SeedProvider</a>
</em>
</td>
<td>
<p>Provider defines the provider type and region for this Seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>taints</code></br>
<em>
<a href="#seedtaint">SeedTaint</a> array
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
<a href="#seedvolume">SeedVolume</a>
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
<a href="#seedsettings">SeedSettings</a>
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
<a href="#ingress">Ingress</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ingress configures Ingress specific settings of the Seed cluster. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#accessrestriction">AccessRestriction</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions for this seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>extensions</code></br>
<em>
<a href="#extension">Extension</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extensions contain type and provider information for Seed extensions.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#namedresourcereference">NamedResourceReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in extension configs by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedstatus">SeedStatus
</h3>


<p>
(<em>Appears on:</em><a href="#seed">Seed</a>)
</p>

<p>
SeedStatus is the status of a Seed.
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
<code>gardener</code></br>
<em>
<a href="#gardener">Gardener</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardener holds information about the Gardener which last acted on the Shoot.</p>
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
<p>KubernetesVersion is the Kubernetes version of the seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="#condition">Condition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Seed's current state.</p>
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
<p>ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the<br />Seed's generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>clusterIdentity</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterIdentity is the identity of the Seed cluster. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>clientCertificateExpirationTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientCertificateExpirationTimestamp is the timestamp at which gardenlet's client certificate expires.</p>
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
<p>LastOperation holds information about the last operation on the Seed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedtaint">SeedTaint
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
SeedTaint describes a taint on a seed.
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
<code>key</code></br>
<em>
string
</em>
</td>
<td>
<p>Key is the taint key to be applied to a seed.</p>
</td>
</tr>
<tr>
<td>
<code>value</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Value is the taint value corresponding to the taint key.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedtemplate">SeedTemplate
</h3>


<p>
SeedTemplate is a template for creating a Seed object.
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
<a href="#seedspec">SeedSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the Seed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedvolume">SeedVolume
</h3>


<p>
(<em>Appears on:</em><a href="#seedspec">SeedSpec</a>)
</p>

<p>
SeedVolume contains settings for persistentvolumes created in the seed cluster.
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
<code>minimumSize</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinimumSize defines the minimum size that should be used for PVCs in the seed.</p>
</td>
</tr>
<tr>
<td>
<code>providers</code></br>
<em>
<a href="#seedvolumeprovider">SeedVolumeProvider</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Providers is a list of storage class provisioner types for the seed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="seedvolumeprovider">SeedVolumeProvider
</h3>


<p>
(<em>Appears on:</em><a href="#seedvolume">SeedVolume</a>)
</p>

<p>
SeedVolumeProvider is a storage class provisioner type.
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
<code>purpose</code></br>
<em>
string
</em>
</td>
<td>
<p>Purpose is the purpose of this provider.</p>
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
<p>Name is the name of the storage class provisioner type.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="serviceaccountconfig">ServiceAccountConfig
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
ServiceAccountConfig is the kube-apiserver configuration for service accounts.
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
<code>issuer</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Issuer is the identifier of the service account token issuer. The issuer will assert this<br />identifier in "iss" claim of issued tokens. This value is used to generate new service account tokens.<br />This value is a string or URI. Defaults to URI of the API server.</p>
</td>
</tr>
<tr>
<td>
<code>extendTokenExpiration</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExtendTokenExpiration turns on projected service account expiration extension during token generation, which<br />helps safe transition from legacy token to bound service account token feature. If this flag is enabled,<br />admission injected tokens would be extended up to 1 year to prevent unexpected failure during transition,<br />ignoring value of service-account-max-token-expiration.</p>
</td>
</tr>
<tr>
<td>
<code>maxTokenExpiration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxTokenExpiration is the maximum validity duration of a token created by the service account token issuer. If an<br />otherwise valid TokenRequest with a validity duration larger than this value is requested, a token will be issued<br />with a validity duration of this value.<br />This field must be within [30d,90d].</p>
</td>
</tr>
<tr>
<td>
<code>acceptedIssuers</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AcceptedIssuers is an additional set of issuers that are used to determine which service account tokens are accepted.<br />These values are not used to generate new service account tokens. Only useful when service account tokens are also<br />issued by another external system or a change of the current issuer that is used for generating tokens is being performed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="serviceaccountkeyrotation">ServiceAccountKeyRotation
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentialsrotation">ShootCredentialsRotation</a>)
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
<code>phase</code></br>
<em>
<a href="#credentialsrotationphase">CredentialsRotationPhase</a>
</em>
</td>
<td>
<p>Phase describes the phase of the service account key credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the service account key credential rotation was successfully<br />completed.</p>
</td>
</tr>
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
<tr>
<td>
<code>lastInitiationFinishedTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationFinishedTime is the recent time when the service account key credential rotation initiation was<br />completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTriggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTriggeredTime is the recent time when the service account key credential rotation completion was<br />triggered.</p>
</td>
</tr>
<tr>
<td>
<code>pendingWorkersRollouts</code></br>
<em>
<a href="#pendingworkersrollout">PendingWorkersRollout</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkersRollouts contains the name of a worker pool and the initiation time of their last rollout due to<br />credentials rotation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shoot">Shoot
</h3>


<p>
Shoot represents a Shoot cluster created and managed by Gardener.
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
<a href="#shootspec">ShootSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the Shoot cluster.<br />If the object's deletion timestamp is set, this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#shootstatus">ShootStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Shoot cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootadvertisedaddress">ShootAdvertisedAddress
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatus">ShootStatus</a>)
</p>

<p>
ShootAdvertisedAddress contains information for the shoot's Kube API server.
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
<p>Name of the advertised address. e.g. external</p>
</td>
</tr>
<tr>
<td>
<code>url</code></br>
<em>
string
</em>
</td>
<td>
<p>The URL of the API Server. e.g. https://api.foo.bar or https://1.2.3.4</p>
</td>
</tr>
<tr>
<td>
<code>application</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Application is the name of the application this address belongs to. Used by UI clients.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootcredentials">ShootCredentials
</h3>


<p>
(<em>Appears on:</em><a href="#shootstatus">ShootStatus</a>)
</p>

<p>
ShootCredentials contains information about the shoot credentials.
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
<code>rotation</code></br>
<em>
<a href="#shootcredentialsrotation">ShootCredentialsRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Rotation contains information about the credential rotations.</p>
</td>
</tr>
<tr>
<td>
<code>encryptionAtRest</code></br>
<em>
<a href="#encryptionatrest">EncryptionAtRest</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EncryptionAtRest contains information about Shoot data encryption at rest.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootcredentialsrotation">ShootCredentialsRotation
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentials">ShootCredentials</a>)
</p>

<p>
ShootCredentialsRotation contains information about the rotation of credentials.
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
<code>sshKeypair</code></br>
<em>
<a href="#shootsshkeypairrotation">ShootSSHKeypairRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SSHKeypair contains information about the ssh-keypair credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>observability</code></br>
<em>
<a href="#observabilityrotation">ObservabilityRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Observability contains information about the observability credential rotation.</p>
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
<tr>
<td>
<code>etcdEncryptionKey</code></br>
<em>
<a href="#etcdencryptionkeyrotation">ETCDEncryptionKeyRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCDEncryptionKey contains information about the ETCD encryption key credential rotation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootkubeconfigrotation">ShootKubeconfigRotation
</h3>


<p>
ShootKubeconfigRotation contains information about the kubeconfig credential rotation.
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
<p>LastInitiationTime is the most recent time when the kubeconfig credential rotation was initiated.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the kubeconfig credential rotation was successfully completed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootmachineimage">ShootMachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#machine">Machine</a>)
</p>

<p>
ShootMachineImage defines the name and the version of the shoot's machine image in any environment. Has to be
defined in the respective CloudProfile.
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
<p>Name is the name of the image.</p>
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
<p>ProviderConfig is the shoot's individual configuration passed to an extension resource.</p>
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
<em>(Optional)</em>
<p>Version is the version of the shoot's image.<br />If version is not provided, it will be defaulted to the latest version from the CloudProfile.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootnetworks">ShootNetworks
</h3>


<p>
(<em>Appears on:</em><a href="#seednetworks">SeedNetworks</a>)
</p>

<p>
ShootNetworks contains the default networks CIDRs for shoots.
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
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Pods is the CIDR of the pod network.</p>
</td>
</tr>
<tr>
<td>
<code>services</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Services is the CIDR of the service network.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootpurpose">ShootPurpose
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
ShootPurpose is a type alias for string.
</p>


<h3 id="shootsshkeypairrotation">ShootSSHKeypairRotation
</h3>


<p>
(<em>Appears on:</em><a href="#shootcredentialsrotation">ShootCredentialsRotation</a>)
</p>

<p>
ShootSSHKeypairRotation contains information about the ssh-keypair credential rotation.
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
<p>LastInitiationTime is the most recent time when the ssh-keypair credential rotation was initiated.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the ssh-keypair credential rotation was successfully completed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootspec">ShootSpec
</h3>


<p>
(<em>Appears on:</em><a href="#shoot">Shoot</a>, <a href="#shoottemplate">ShootTemplate</a>)
</p>

<p>
ShootSpec is the specification of a Shoot.
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
<code>addons</code></br>
<em>
<a href="#addons">Addons</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Addons contains information about enabled/disabled addons and their configuration.<br />Deprecated: This field is deprecated. Enabling addons will be forbidden starting from Kubernetes 1.35.</p>
</td>
</tr>
<tr>
<td>
<code>cloudProfileName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudProfileName is a name of a CloudProfile object.<br />Deprecated: This field will be removed in a future version of Gardener. Use `CloudProfile` instead.<br />Until Kubernetes v1.33, this field is synced with the `CloudProfile` field.<br />Starting with Kubernetes v1.34, this field is set to empty string and must not be provided anymore.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#dns">DNS</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNS contains information about the DNS settings of the Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>extensions</code></br>
<em>
<a href="#extension">Extension</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extensions contain type and provider information for Shoot extensions.</p>
</td>
</tr>
<tr>
<td>
<code>hibernation</code></br>
<em>
<a href="#hibernation">Hibernation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Hibernation contains information whether the Shoot is suspended or not.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#kubernetes">Kubernetes</a>
</em>
</td>
<td>
<p>Kubernetes contains the version and configuration settings of the control plane components.</p>
</td>
</tr>
<tr>
<td>
<code>networking</code></br>
<em>
<a href="#networking">Networking</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Networking contains information about cluster networking such as CNI Plugin type, CIDRs, ...etc.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#maintenance">Maintenance</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maintenance contains information about the time window for maintenance operations and which<br />operations should be performed.</p>
</td>
</tr>
<tr>
<td>
<code>monitoring</code></br>
<em>
<a href="#monitoring">Monitoring</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Monitoring contains information about custom monitoring configurations for the shoot.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
<a href="#provider">Provider</a>
</em>
</td>
<td>
<p>Provider contains all provider-specific and provider-relevant information.</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#shootpurpose">ShootPurpose</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Purpose is the purpose class for this cluster.</p>
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
<p>Region is a name of a region. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>secretBindingName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretBindingName is the name of a SecretBinding that has a reference to the provider secret.<br />The credentials inside the provider secret will be used to create the shoot in the respective account.<br />The field is mutually exclusive with CredentialsBindingName.<br />This field is immutable.<br />Deprecated: Use CredentialsBindingName instead. See https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md for migration instructions.</p>
</td>
</tr>
<tr>
<td>
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the seed cluster that runs the control plane of the Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>seedSelector</code></br>
<em>
<a href="#seedselector">SeedSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector is an optional selector which must match a seed's labels for the shoot to be scheduled on that seed.<br />Once the shoot is assigned to a seed, the selector can only be changed later if the new one still matches the assigned seed.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#namedresourcereference">NamedResourceReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in extension configs by their names.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="#toleration">Toleration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations contains the tolerations for taints on seed clusters.</p>
</td>
</tr>
<tr>
<td>
<code>exposureClassName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExposureClassName is the optional name of an exposure class to apply a control plane endpoint exposure strategy.</p>
</td>
</tr>
<tr>
<td>
<code>systemComponents</code></br>
<em>
<a href="#systemcomponents">SystemComponents</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SystemComponents contains the settings of system components in the control or data plane of the Shoot cluster.</p>
</td>
</tr>
<tr>
<td>
<code>controlPlane</code></br>
<em>
<a href="#controlplane">ControlPlane</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControlPlane contains general settings for the control plane of the shoot.</p>
</td>
</tr>
<tr>
<td>
<code>schedulerName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SchedulerName is the name of the responsible scheduler which schedules the shoot.<br />If not specified, the default scheduler takes over.<br />This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>cloudProfile</code></br>
<em>
<a href="#cloudprofilereference">CloudProfileReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudProfile contains a reference to a CloudProfile or a NamespacedCloudProfile.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsBindingName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CredentialsBindingName is the name of a CredentialsBinding that has a reference to the provider credentials.<br />The credentials will be used to create the shoot in the respective account. The field is mutually exclusive with SecretBindingName.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#accessrestrictionwithoptions">AccessRestrictionWithOptions</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions for this shoot cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootstate">ShootState
</h3>


<p>
ShootState contains a snapshot of the Shoot's state required to migrate the Shoot's control plane to a new Seed.
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
<a href="#shootstatespec">ShootStateSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the ShootState.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootstatespec">ShootStateSpec
</h3>


<p>
(<em>Appears on:</em><a href="#shootstate">ShootState</a>)
</p>

<p>
ShootStateSpec is the specification of the ShootState.
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
<code>gardener</code></br>
<em>
<a href="#gardenerresourcedata">GardenerResourceData</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardener holds the data required to generate resources deployed by the gardenlet</p>
</td>
</tr>
<tr>
<td>
<code>extensions</code></br>
<em>
<a href="#extensionresourcestate">ExtensionResourceState</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extensions holds the state of custom resources reconciled by extension controllers in the seed</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#resourcedata">ResourceData</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds the data of resources referred to by extension controller states</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shootstatus">ShootStatus
</h3>


<p>
(<em>Appears on:</em><a href="#shoot">Shoot</a>)
</p>

<p>
ShootStatus holds the most recently observed status of the Shoot cluster.
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
<a href="#condition">Condition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Shoots's current state.</p>
</td>
</tr>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#condition">Condition</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Constraints represents conditions of a Shoot's current state that constraint some operations on it.</p>
</td>
</tr>
<tr>
<td>
<code>gardener</code></br>
<em>
<a href="#gardener">Gardener</a>
</em>
</td>
<td>
<p>Gardener holds information about the Gardener which last acted on the Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>hibernated</code></br>
<em>
boolean
</em>
</td>
<td>
<p>IsHibernated indicates whether the Shoot is currently hibernated.</p>
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
<p>LastOperation holds information about the last operation on the Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>lastErrors</code></br>
<em>
<a href="#lasterror">LastError</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastErrors holds information about the last occurred error(s) during an operation.</p>
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
<p>ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the<br />Shoot's generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>retryCycleStartTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation<br />must be retried until we give up).</p>
</td>
</tr>
<tr>
<td>
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the seed cluster that runs the control plane of the Shoot. This value is only written<br />after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.</p>
</td>
</tr>
<tr>
<td>
<code>technicalID</code></br>
<em>
string
</em>
</td>
<td>
<p>TechnicalID is a unique technical ID for this Shoot. It is used for the infrastructure resources, and<br />basically everything that is related to this particular Shoot. For regular shoot clusters, this is also the name<br />of the namespace in the seed cluster running the shoot's control plane. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>uid</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#uid-types-pkg">UID</a>
</em>
</td>
<td>
<p>UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.<br />It is used to compute unique hashes. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>clusterIdentity</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterIdentity is the identity of the Shoot cluster. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>advertisedAddresses</code></br>
<em>
<a href="#shootadvertisedaddress">ShootAdvertisedAddress</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of addresses that are relevant to the shoot.<br />These include the Kube API server address and also the service account issuer.</p>
</td>
</tr>
<tr>
<td>
<code>migrationStartTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MigrationStartTime is the time when a migration to a different seed was initiated.</p>
</td>
</tr>
<tr>
<td>
<code>credentials</code></br>
<em>
<a href="#shootcredentials">ShootCredentials</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credentials contains information about the shoot credentials.</p>
</td>
</tr>
<tr>
<td>
<code>lastHibernationTriggerTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastHibernationTriggerTime indicates the last time when the hibernation controller<br />managed to change the hibernation settings of the cluster</p>
</td>
</tr>
<tr>
<td>
<code>lastMaintenance</code></br>
<em>
<a href="#lastmaintenance">LastMaintenance</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastMaintenance holds information about the last maintenance operations on the Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>networking</code></br>
<em>
<a href="#networkingstatus">NetworkingStatus</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Networking contains information about cluster networking such as CIDRs.</p>
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
<p>InPlaceUpdates contains information about in-place updates for the Shoot workers.</p>
</td>
</tr>
<tr>
<td>
<code>manualWorkerPoolRollout</code></br>
<em>
<a href="#manualworkerpoolrollout">ManualWorkerPoolRollout</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManualWorkerPoolRollout contains information about the worker pool rollout progress.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="shoottemplate">ShootTemplate
</h3>


<p>
ShootTemplate is a template for creating a Shoot object.
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
<a href="#shootspec">ShootSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the Shoot.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="structuredauthentication">StructuredAuthentication
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
StructuredAuthentication contains authentication config for kube-apiserver.
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
<code>configMapName</code></br>
<em>
string
</em>
</td>
<td>
<p>ConfigMapName is the name of the ConfigMap in the project namespace which contains AuthenticationConfiguration<br />for the kube-apiserver.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="structuredauthorization">StructuredAuthorization
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
StructuredAuthorization contains authorization config for kube-apiserver.
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
<code>configMapName</code></br>
<em>
string
</em>
</td>
<td>
<p>ConfigMapName is the name of the ConfigMap in the project namespace which contains AuthorizationConfiguration for<br />the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfigs</code></br>
<em>
<a href="#authorizerkubeconfigreference">AuthorizerKubeconfigReference</a> array
</em>
</td>
<td>
<p>Kubeconfigs is a list of references for kubeconfigs for the authorization webhooks.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="swapbehavior">SwapBehavior
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#memoryswapconfiguration">MemorySwapConfiguration</a>)
</p>

<p>
SwapBehavior configures swap memory available to container workloads
</p>


<h3 id="systemcomponents">SystemComponents
</h3>


<p>
(<em>Appears on:</em><a href="#shootspec">ShootSpec</a>)
</p>

<p>
SystemComponents contains the settings of system components in the control or data plane of the Shoot cluster.
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
<code>coreDNS</code></br>
<em>
<a href="#coredns">CoreDNS</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CoreDNS contains the settings of the Core DNS components running in the data plane of the Shoot cluster.</p>
</td>
</tr>
<tr>
<td>
<code>nodeLocalDNS</code></br>
<em>
<a href="#nodelocaldns">NodeLocalDNS</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeLocalDNS contains the settings of the node local DNS components running in the data plane of the Shoot cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="toleration">Toleration
</h3>


<p>
(<em>Appears on:</em><a href="#exposureclassscheduling">ExposureClassScheduling</a>, <a href="#projecttolerations">ProjectTolerations</a>, <a href="#shootspec">ShootSpec</a>)
</p>

<p>
Toleration is a toleration for a seed taint.
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
<code>key</code></br>
<em>
string
</em>
</td>
<td>
<p>Key is the toleration key to be applied to a project or shoot.</p>
</td>
</tr>
<tr>
<td>
<code>value</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Value is the toleration value corresponding to the toleration key.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="versionclassification">VersionClassification
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#expirableversion">ExpirableVersion</a>, <a href="#expirableversionstatus">ExpirableVersionStatus</a>, <a href="#lifecyclestage">LifecycleStage</a>, <a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
VersionClassification is the logical state of a version.
</p>


<h3 id="verticalpodautoscaler">VerticalPodAutoscaler
</h3>


<p>
(<em>Appears on:</em><a href="#kubernetes">Kubernetes</a>)
</p>

<p>
VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.
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
<code>enabled</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Enabled specifies whether the Kubernetes VPA shall be enabled for the shoot cluster.</p>
</td>
</tr>
<tr>
<td>
<code>evictAfterOOMThreshold</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictAfterOOMThreshold defines the threshold that will lead to pod eviction in case it OOMed in less than the given<br />threshold since its start and if it has only one container (default: 10m0s).</p>
</td>
</tr>
<tr>
<td>
<code>evictionRateBurst</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionRateBurst defines the burst of pods that can be evicted (default: 1)</p>
</td>
</tr>
<tr>
<td>
<code>evictionRateLimit</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionRateLimit defines the number of pods that can be evicted per second. A rate limit set to 0 or -1 will<br />disable the rate limiter (default: -1).</p>
</td>
</tr>
<tr>
<td>
<code>evictionTolerance</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionTolerance defines the fraction of replica count that can be evicted for update in case more than one<br />pod can be evicted (default: 0.5).</p>
</td>
</tr>
<tr>
<td>
<code>recommendationMarginFraction</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationMarginFraction is the fraction of usage added as the safety margin to the recommended request<br />(default: 0.15).</p>
</td>
</tr>
<tr>
<td>
<code>updaterInterval</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdaterInterval is the interval how often the updater should run (default: 1m0s).</p>
</td>
</tr>
<tr>
<td>
<code>recommenderInterval</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommenderInterval is the interval how often metrics should be fetched (default: 1m0s).</p>
</td>
</tr>
<tr>
<td>
<code>targetCPUPercentile</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetCPUPercentile is the usage percentile that will be used as a base for CPU target recommendation.<br />Doesn't affect CPU lower bound, CPU upper bound nor memory recommendations.<br />(default: 0.9)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationLowerBoundCPUPercentile</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationLowerBoundCPUPercentile is the usage percentile that will be used for the lower bound on CPU recommendation.<br />(default: 0.5)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationUpperBoundCPUPercentile</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationUpperBoundCPUPercentile is the usage percentile that will be used for the upper bound on CPU recommendation.<br />(default: 0.95)</p>
</td>
</tr>
<tr>
<td>
<code>targetMemoryPercentile</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetMemoryPercentile is the usage percentile that will be used as a base for memory target recommendation.<br />Doesn't affect memory lower bound nor memory upper bound.<br />(default: 0.9)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationLowerBoundMemoryPercentile</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationLowerBoundMemoryPercentile is the usage percentile that will be used for the lower bound on memory recommendation.<br />(default: 0.5)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationUpperBoundMemoryPercentile</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationUpperBoundMemoryPercentile is the usage percentile that will be used for the upper bound on memory recommendation.<br />(default: 0.95)</p>
</td>
</tr>
<tr>
<td>
<code>cpuHistogramDecayHalfLife</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CPUHistogramDecayHalfLife is the amount of time it takes a historical CPU usage sample to lose half of its weight.<br />(default: 24h)</p>
</td>
</tr>
<tr>
<td>
<code>memoryHistogramDecayHalfLife</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryHistogramDecayHalfLife is the amount of time it takes a historical memory usage sample to lose half of its weight.<br />(default: 24h)</p>
</td>
</tr>
<tr>
<td>
<code>memoryAggregationInterval</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAggregationInterval is the length of a single interval, for which the peak memory usage is computed.<br />(default: 24h)</p>
</td>
</tr>
<tr>
<td>
<code>memoryAggregationIntervalCount</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAggregationIntervalCount is the number of consecutive memory-aggregation-intervals which make up the<br />MemoryAggregationWindowLength which in turn is the period for memory usage aggregation by VPA. In other words,<br />`MemoryAggregationWindowLength = memory-aggregation-interval * memory-aggregation-interval-count`.<br />(default: 8)</p>
</td>
</tr>
<tr>
<td>
<code>featureGates</code></br>
<em>
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
<tr>
<td>
<code>recommenderUpdateWorkerCount</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommenderUpdateWorkerCount is the number of workers used in the vpa-recommender for updating VPAs and VPACheckpoints in parallel.<br />(default: 10)</p>
</td>
</tr>

</tbody>
</table>


<h3 id="volume">Volume
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
Volume contains information about the volume type, size, and encryption.
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
<p>VolumeSize is the size of the volume.</p>
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


<h3 id="volumetype">VolumeType
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofilespec">CloudProfileSpec</a>, <a href="#namespacedcloudprofilespec">NamespacedCloudProfileSpec</a>)
</p>

<p>
VolumeType contains certain properties of a volume type.
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
<code>class</code></br>
<em>
string
</em>
</td>
<td>
<p>Class is the class of the volume type.</p>
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
<p>Name is the name of the volume type.</p>
</td>
</tr>
<tr>
<td>
<code>usable</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Usable defines if the volume type can be used for shoot clusters.</p>
</td>
</tr>
<tr>
<td>
<code>minSize</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinSize is the minimal supported storage size.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="watchcachesizes">WatchCacheSizes
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
WatchCacheSizes contains configuration of the API server's watch cache sizes.
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
<code>default</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default is not respected anymore by kube-apiserver.<br />The cache is sized automatically.<br />Deprecated: This field is deprecated. Setting the default cache size will be forbidden starting from Kubernetes 1.35.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#resourcewatchcachesize">ResourceWatchCacheSize</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources configures the watch cache size of the kube-apiserver per resource<br />(flag `--watch-cache-sizes`).<br />See: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/</p>
</td>
</tr>

</tbody>
</table>


<h3 id="worker">Worker
</h3>


<p>
(<em>Appears on:</em><a href="#provider">Provider</a>)
</p>

<p>
Worker is the base definition of a worker group.
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
<code>caBundle</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CABundle is a certificate bundle which will be installed onto every machine of this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>cri</code></br>
<em>
<a href="#cri">CRI</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CRI contains configurations of CRI support of every machine in the worker pool.<br />Defaults to a CRI with name `containerd`.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#workerkubernetes">WorkerKubernetes</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubernetes contains configuration for Kubernetes components related to this worker pool.</p>
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
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the worker group.</p>
</td>
</tr>
<tr>
<td>
<code>machine</code></br>
<em>
<a href="#machine">Machine</a>
</em>
</td>
<td>
<p>Machine contains information about the machine type and image.</p>
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
<p>Maximum is the maximum number of machines to create.<br />This value is divided by the number of configured zones for a fair distribution.</p>
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
<p>Minimum is the minimum number of machines to create.<br />This value is divided by the number of configured zones for a fair distribution.</p>
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
<em>(Optional)</em>
<p>MaxSurge is maximum number of machines that are created during an update.<br />This value is divided by the number of configured zones for a fair distribution.<br />Defaults to 0 in case of an in-place update.<br />Defaults to 1 in case of a rolling update.</p>
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
<em>(Optional)</em>
<p>MaxUnavailable is the maximum number of machines that can be unavailable during an update.<br />This value is divided by the number of configured zones for a fair distribution.<br />Defaults to 1 in case of an in-place update.<br />Defaults to 0 in case of a rolling update.</p>
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
<p>ProviderConfig is the provider-specific configuration for this worker pool.</p>
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
<code>volume</code></br>
<em>
<a href="#volume">Volume</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains information about the volume type and size.</p>
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
<p>Zones is a list of availability zones that are used to evenly distribute this worker pool. Optional<br />as not every provider may support availability zones.</p>
</td>
</tr>
<tr>
<td>
<code>systemComponents</code></br>
<em>
<a href="#workersystemcomponents">WorkerSystemComponents</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SystemComponents contains configuration for system components related to this worker pool</p>
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
<code>sysctls</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Sysctls is a map of kernel settings to apply on all machines in this worker pool.</p>
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
<p>Priority (or weight) is the importance by which this worker group will be scaled by cluster autoscaling.</p>
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
<tr>
<td>
<code>controlPlane</code></br>
<em>
<a href="#workercontrolplane">WorkerControlPlane</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControlPlane specifies that the shoot cluster control plane components should be running in this worker pool.<br />This is only relevant for self-hosted shoot clusters.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workercontrolplane">WorkerControlPlane
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
WorkerControlPlane specifies that the shoot cluster control plane components should be running in this worker pool.
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
<code>backup</code></br>
<em>
<a href="#backup">Backup</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup holds the object store configuration for the backups of shoot (currently only etcd).<br />If it is not specified, then there won't be any backups taken.</p>
</td>
</tr>
<tr>
<td>
<code>exposure</code></br>
<em>
<a href="#exposure">Exposure</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Exposure holds the exposure configuration for the shoot (either `extension` or `dns` or omitted/empty).</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerkubernetes">WorkerKubernetes
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
WorkerKubernetes contains configuration for Kubernetes components related to this worker pool.
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
<code>kubelet</code></br>
<em>
<a href="#kubeletconfig">KubeletConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubelet contains configuration settings for all kubelets of this worker pool.<br />If set, all `spec.kubernetes.kubelet` settings will be overwritten for this worker pool (no merge of settings).</p>
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
<em>(Optional)</em>
<p>Version is the semantic Kubernetes version to use for the Kubelet in this Worker Group.<br />If not specified the kubelet version is derived from the global shoot cluster kubernetes version.<br />version must be equal or lower than the version of the shoot kubernetes version.<br />Only one minor version difference to other worker groups and global kubernetes version is allowed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workersystemcomponents">WorkerSystemComponents
</h3>


<p>
(<em>Appears on:</em><a href="#worker">Worker</a>)
</p>

<p>
WorkerSystemComponents contains configuration for system components related to this worker pool
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
<code>allow</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Allow determines whether the pool should be allowed to host system components or not (defaults to true)</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerssettings">WorkersSettings
</h3>


<p>
(<em>Appears on:</em><a href="#provider">Provider</a>)
</p>

<p>
WorkersSettings contains settings for all workers.
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
<code>sshAccess</code></br>
<em>
<a href="#sshaccess">SSHAccess</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SSHAccess contains settings regarding ssh access to the worker nodes.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="zoneselectionmode">ZoneSelectionMode
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#seedsettingzoneselection">SeedSettingZoneSelection</a>)
</p>

<p>
ZoneSelectionMode is the mode for zone selection.
</p>


