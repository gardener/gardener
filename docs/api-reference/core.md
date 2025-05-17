<p>Packages:</p>
<ul>
<li>
<a href="#core.gardener.cloud%2fv1beta1">core.gardener.cloud/v1beta1</a>
</li>
</ul>
<h2 id="core.gardener.cloud/v1beta1">core.gardener.cloud/v1beta1</h2>
<p>
<p>Package v1beta1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#core.gardener.cloud/v1beta1.BackupBucket">BackupBucket</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.BackupEntry">BackupEntry</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.CloudProfile">CloudProfile</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.ControllerDeployment">ControllerDeployment</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.ControllerInstallation">ControllerInstallation</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistration">ControllerRegistration</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.ExposureClass">ExposureClass</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.InternalSecret">InternalSecret</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfile">NamespacedCloudProfile</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.Project">Project</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.Quota">Quota</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.SecretBinding">SecretBinding</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.Seed">Seed</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.Shoot">Shoot</a>
</li><li>
<a href="#core.gardener.cloud/v1beta1.ShootState">ShootState</a>
</li></ul>
<h3 id="core.gardener.cloud/v1beta1.BackupBucket">BackupBucket
</h3>
<p>
<p>BackupBucket holds details about backup bucket</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>BackupBucket</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.BackupBucketSpec">
BackupBucketSpec
</a>
</em>
</td>
<td>
<p>Specification of the Backup Bucket.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>provider</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.BackupBucketProvider">
BackupBucketProvider
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the configuration passed to BackupBucket resource.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the credentials to access object store.</p>
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
<p>SeedName holds the name of the seed allocated to BackupBucket for running controller.
This field is immutable.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.BackupBucketStatus">
BackupBucketStatus
</a>
</em>
</td>
<td>
<p>Most recently observed status of the Backup Bucket.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.BackupEntry">BackupEntry
</h3>
<p>
<p>BackupEntry holds details about shoot backup.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>BackupEntry</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.BackupEntrySpec">
BackupEntrySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec contains the specification of the Backup Entry.</p>
<br/>
<br/>
<table>
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
<p>SeedName holds the name of the seed to which this BackupEntry is scheduled</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.BackupEntryStatus">
BackupEntryStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status contains the most recently observed status of the Backup Entry.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CloudProfile">CloudProfile
</h3>
<p>
<p>CloudProfile represents certain properties about a provider environment.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>CloudProfile</code></td>
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
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">
CloudProfileSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the provider environment properties.</p>
<br/>
<br/>
<table>
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
<a href="#core.gardener.cloud/v1beta1.KubernetesSettings">
KubernetesSettings
</a>
</em>
</td>
<td>
<p>Kubernetes contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.MachineImage">
[]MachineImage
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineType">
[]MachineType
</a>
</em>
</td>
<td>
<p>MachineTypes contains constraints regarding allowed values for machine types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Region">
[]Region
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSelector">
SeedSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector contains an optional list of labels on <code>Seed</code> resources that marks those seeds whose shoots may use this provider profile.
An empty list means that all seeds of the same provider type are supported.
This is useful for environments that are of the same type (like openstack) but may have different &ldquo;instances&rdquo;/landscapes.
Optionally a list of possible providers can be added to enable cross-provider scheduling. By default, the provider
type of the seed must match the shoot&rsquo;s provider.</p>
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
<a href="#core.gardener.cloud/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>bastion</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Bastion">
Bastion
</a>
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
<a href="#core.gardener.cloud/v1beta1.Limits">
Limits
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits configures operational limits for Shoot clusters using this CloudProfile.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md">https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md</a>.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CapabilityDefinition">
[]CapabilityDefinition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capabilities contains the definition of all possible capabilities in the CloudProfile.
Only capabilities and values defined here can be used to describe MachineImages and MachineTypes.
The order of values for a given capability is relevant. The most important value is listed first.
During maintenance upgrades, the image that matches most capabilities will be selected.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerDeployment">ControllerDeployment
</h3>
<p>
<p>ControllerDeployment contains information about how this controller is deployed.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControllerDeployment</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>InjectGardenKubeconfig controls whether a kubeconfig to the garden cluster should be injected into workload
resources.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerInstallation">ControllerInstallation
</h3>
<p>
<p>ControllerInstallation represents an installation request for an external controller.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControllerInstallation</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerInstallationSpec">
ControllerInstallationSpec
</a>
</em>
</td>
<td>
<p>Spec contains the specification of this installation.
If the object&rsquo;s deletion timestamp is set, this field is immutable.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>registrationRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<p>RegistrationRef is used to reference a ControllerRegistration resource.
The name field of the RegistrationRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>seedRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<p>SeedRef is used to reference a Seed resource. The name field of the SeedRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>deploymentRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeploymentRef is used to reference a ControllerDeployment resource.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerInstallationStatus">
ControllerInstallationStatus
</a>
</em>
</td>
<td>
<p>Status contains the status of this installation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerRegistration">ControllerRegistration
</h3>
<p>
<p>ControllerRegistration represents a registration of an external controller.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControllerRegistration</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationSpec">
ControllerRegistrationSpec
</a>
</em>
</td>
<td>
<p>Spec contains the specification of this registration.
If the object&rsquo;s deletion timestamp is set, this field is immutable.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerResource">
[]ControllerResource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of combinations of kinds (DNSProvider, Infrastructure, Generic, &hellip;) and their actual types
(aws-route53, gcp, auditlog, &hellip;).</p>
</td>
</tr>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationDeployment">
ControllerRegistrationDeployment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment contains information for how this controller is deployed.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ExposureClass">ExposureClass
</h3>
<p>
<p>ExposureClass represents a control plane endpoint exposure strategy.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ExposureClass</code></td>
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
<code>handler</code></br>
<em>
string
</em>
</td>
<td>
<p>Handler is the name of the handler which applies the control plane endpoint exposure strategy.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>scheduling</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ExposureClassScheduling">
ExposureClassScheduling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Scheduling holds information how to select applicable Seed&rsquo;s for ExposureClass usage.
This field is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.InternalSecret">InternalSecret
</h3>
<p>
<p>InternalSecret holds secret data of a certain type. The total bytes of the values in
the Data field must be less than MaxSecretSize bytes.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>InternalSecret</code></td>
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
<p>Standard object&rsquo;s metadata.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata</a></p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>immutable</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Immutable, if set to true, ensures that data stored in the Secret cannot
be updated (only object metadata can be modified).
If not set to true, the field can be modified at any time.
Defaulted to nil.</p>
</td>
</tr>
<tr>
<td>
<code>data</code></br>
<em>
map[string][]byte
</em>
</td>
<td>
<em>(Optional)</em>
<p>Data contains the secret data. Each key must consist of alphanumeric
characters, &lsquo;-&rsquo;, &lsquo;_&rsquo; or &lsquo;.&rsquo;. The serialized form of the secret data is a
base64 encoded string, representing the arbitrary (possibly non-string)
data value here. Described in <a href="https://tools.ietf.org/html/rfc4648#section-4">https://tools.ietf.org/html/rfc4648#section-4</a></p>
</td>
</tr>
<tr>
<td>
<code>stringData</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>stringData allows specifying non-binary secret data in string form.
It is provided as a write-only input field for convenience.
All keys and values are merged into the data field on write, overwriting any existing values.
The stringData field is never output when reading from the API.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secrettype-v1-core">
Kubernetes core/v1.SecretType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Used to facilitate programmatic handling of secret data.
More info: <a href="https://kubernetes.io/docs/concepts/configuration/secret/#secret-types">https://kubernetes.io/docs/concepts/configuration/secret/#secret-types</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NamespacedCloudProfile">NamespacedCloudProfile
</h3>
<p>
<p>NamespacedCloudProfile represents certain properties about a provider environment.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>NamespacedCloudProfile</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">
NamespacedCloudProfileSpec
</a>
</em>
</td>
<td>
<p>Spec defines the provider environment properties.</p>
<br/>
<br/>
<table>
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
<a href="#core.gardener.cloud/v1beta1.KubernetesSettings">
KubernetesSettings
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubernetes contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.MachineImage">
[]MachineImage
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineType">
[]MachineType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineTypes contains constraints regarding allowed values for machine types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>volumeTypes</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>parent</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileReference">
CloudProfileReference
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Limits">
Limits
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits configures operational limits for Shoot clusters using this NamespacedCloudProfile.
Any limits specified here override those set in the parent CloudProfile.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md">https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md</a>.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileStatus">
NamespacedCloudProfileStatus
</a>
</em>
</td>
<td>
<p>Most recently observed status of the NamespacedCloudProfile.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Project">Project
</h3>
<p>
<p>Project holds certain properties about a Gardener project.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Project</code></td>
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
<a href="#core.gardener.cloud/v1beta1.ProjectSpec">
ProjectSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the project properties.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>createdBy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
who created the project. This field is immutable.</p>
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
<p>Description is a human-readable description of what the project is used for.</p>
</td>
</tr>
<tr>
<td>
<code>owner</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Owner is a subject representing a user name, an email address, or any other identifier of a user owning
the project.
IMPORTANT: Be aware that this field will be removed in the <code>v1</code> version of this API in favor of the <code>owner</code>
role. The only way to change the owner will be by moving the <code>owner</code> role. In this API version the only way
to change the owner is to use this field.
TODO: Remove this field in favor of the <code>owner</code> role in <code>v1</code>.</p>
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
<p>Purpose is a human-readable explanation of the project&rsquo;s purpose.</p>
</td>
</tr>
<tr>
<td>
<code>members</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProjectMember">
[]ProjectMember
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Members is a list of subjects representing a user name, an email address, or any other identifier of a user,
group, or service account that has a certain role.</p>
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
<p>Namespace is the name of the namespace that has been created for the Project object.
A nil value means that Gardener will determine the name of the namespace.
If set, its value must be prefixed with <code>garden-</code>.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProjectTolerations">
ProjectTolerations
</a>
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
<a href="#core.gardener.cloud/v1beta1.DualApprovalForDeletion">
[]DualApprovalForDeletion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProjectStatus">
ProjectStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Project.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Quota">Quota
</h3>
<p>
<p>Quota represents a quota on resources consumed by shoot clusters either per project or per provider secret.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Quota</code></td>
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
<a href="#core.gardener.cloud/v1beta1.QuotaSpec">
QuotaSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the Quota constraints.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>clusterLifetimeDays</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.</p>
</td>
</tr>
<tr>
<td>
<code>metrics</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>Metrics is a list of resources which will be put under constraints.</p>
</td>
</tr>
<tr>
<td>
<code>scope</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<p>Scope is the scope of the Quota object, either &lsquo;project&rsquo;, &lsquo;secret&rsquo; or &lsquo;workloadidentity&rsquo;. This field is immutable.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SecretBinding">SecretBinding
</h3>
<p>
<p>SecretBinding represents a binding to a secret in the same or another namespace.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>SecretBinding</code></td>
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
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret object in the same or another namespace.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>quotas</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
[]Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Quotas is a list of references to Quota objects in the same or another namespace.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SecretBindingProvider">
SecretBindingProvider
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider defines the provider type of the SecretBinding.
This field is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Seed">Seed
</h3>
<p>
<p>Seed represents an installation request for an external controller.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Seed</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">
SeedSpec
</a>
</em>
</td>
<td>
<p>Spec contains the specification of this installation.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SeedBackup">
SeedBackup
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedDNS">
SeedDNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedNetworks">
SeedNetworks
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedProvider">
SeedProvider
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedTaint">
[]SeedTaint
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedVolume">
SeedVolume
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettings">
SeedSettings
</a>
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
<a href="#core.gardener.cloud/v1beta1.Ingress">
Ingress
</a>
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
<a href="#core.gardener.cloud/v1beta1.AccessRestriction">
[]AccessRestriction
</a>
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
<a href="#core.gardener.cloud/v1beta1.Extension">
[]Extension
</a>
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
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in extension configs by their names.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SeedStatus">
SeedStatus
</a>
</em>
</td>
<td>
<p>Status contains the status of this installation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Shoot">Shoot
</h3>
<p>
<p>Shoot represents a Shoot cluster created and managed by Gardener.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Shoot</code></td>
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
<a href="#core.gardener.cloud/v1beta1.ShootSpec">
ShootSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the Shoot cluster.
If the object&rsquo;s deletion timestamp is set, this field is immutable.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>addons</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Addons">
Addons
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Addons contains information about enabled/disabled addons and their configuration.</p>
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
<p>CloudProfileName is a name of a CloudProfile object.
Deprecated: This field will be removed in a future version of Gardener. Use <code>CloudProfile</code> instead.
Until removed, this field is synced with the <code>CloudProfile</code> field.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.DNS">
DNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.Extension">
[]Extension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Hibernation">
Hibernation
</a>
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
<a href="#core.gardener.cloud/v1beta1.Kubernetes">
Kubernetes
</a>
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
<a href="#core.gardener.cloud/v1beta1.Networking">
Networking
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Networking contains information about cluster networking such as CNI Plugin type, CIDRs, &hellip;etc.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Maintenance">
Maintenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maintenance contains information about the time window for maintenance operations and which
operations should be performed.</p>
</td>
</tr>
<tr>
<td>
<code>monitoring</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Monitoring">
Monitoring
</a>
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
<a href="#core.gardener.cloud/v1beta1.Provider">
Provider
</a>
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
<a href="#core.gardener.cloud/v1beta1.ShootPurpose">
ShootPurpose
</a>
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
<p>SecretBindingName is the name of a SecretBinding that has a reference to the provider secret.
The credentials inside the provider secret will be used to create the shoot in the respective account.
The field is mutually exclusive with CredentialsBindingName.
This field is immutable.</p>
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
<a href="#core.gardener.cloud/v1beta1.SeedSelector">
SeedSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector is an optional selector which must match a seed&rsquo;s labels for the shoot to be scheduled on that seed.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
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
<a href="#core.gardener.cloud/v1beta1.Toleration">
[]Toleration
</a>
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
<p>ExposureClassName is the optional name of an exposure class to apply a control plane endpoint exposure strategy.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>systemComponents</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SystemComponents">
SystemComponents
</a>
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
<a href="#core.gardener.cloud/v1beta1.ControlPlane">
ControlPlane
</a>
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
<p>SchedulerName is the name of the responsible scheduler which schedules the shoot.
If not specified, the default scheduler takes over.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>cloudProfile</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileReference">
CloudProfileReference
</a>
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
<p>CredentialsBindingName is the name of a CredentialsBinding that has a reference to the provider credentials.
The credentials will be used to create the shoot in the respective account. The field is mutually exclusive with SecretBindingName.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AccessRestrictionWithOptions">
[]AccessRestrictionWithOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions for this shoot cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ShootStatus">
ShootStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootState">ShootState
</h3>
<p>
<p>ShootState contains a snapshot of the Shoot&rsquo;s state required to migrate the Shoot&rsquo;s control plane to a new Seed.</p>
</p>
<table>
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
core.gardener.cloud/v1beta1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ShootState</code></td>
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
<a href="#core.gardener.cloud/v1beta1.ShootStateSpec">
ShootStateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the ShootState.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>gardener</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.GardenerResourceData">
[]GardenerResourceData
</a>
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
<a href="#core.gardener.cloud/v1beta1.ExtensionResourceState">
[]ExtensionResourceState
</a>
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
<a href="#core.gardener.cloud/v1beta1.ResourceData">
[]ResourceData
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds the data of resources referred to by extension controller states</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.APIServerLogging">APIServerLogging
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>APIServerLogging contains configuration for the logs level and http access logs</p>
</p>
<table>
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Verbosity is the kube-apiserver log verbosity level
Defaults to 2.</p>
</td>
</tr>
<tr>
<td>
<code>httpAccessVerbosity</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTPAccessVerbosity is the kube-apiserver access logs level</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.APIServerRequests">APIServerRequests
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>APIServerRequests contains configuration for request-specific settings for the kube-apiserver.</p>
</p>
<table>
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNonMutatingInflight is the maximum number of non-mutating requests in flight at a given time. When the server
exceeds this, it rejects requests.</p>
</td>
</tr>
<tr>
<td>
<code>maxMutatingInflight</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxMutatingInflight is the maximum number of mutating requests in flight at a given time. When the server
exceeds this, it rejects requests.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.AccessRestriction">AccessRestriction
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.AccessRestrictionWithOptions">AccessRestrictionWithOptions</a>, 
<a href="#core.gardener.cloud/v1beta1.Region">Region</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>AccessRestriction describes an access restriction for a Kubernetes cluster (e.g., EU access-only).</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.AccessRestrictionWithOptions">AccessRestrictionWithOptions
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>AccessRestrictionWithOptions describes an access restriction for a Kubernetes cluster (e.g., EU access-only) and
allows to specify additional options.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>AccessRestriction</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AccessRestriction">
AccessRestriction
</a>
</em>
</td>
<td>
<p>
(Members of <code>AccessRestriction</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>options</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Options is a map of additional options for the access restriction.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Addon">Addon
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubernetesDashboard">KubernetesDashboard</a>, 
<a href="#core.gardener.cloud/v1beta1.NginxIngress">NginxIngress</a>)
</p>
<p>
<p>Addon allows enabling or disabling a specific addon and is used to derive from.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Enabled indicates whether the addon is enabled or not.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Addons">Addons
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Addons is a collection of configuration for specific addons which are managed by the Gardener.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.KubernetesDashboard">
KubernetesDashboard
</a>
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
<a href="#core.gardener.cloud/v1beta1.NginxIngress">
NginxIngress
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NginxIngress holds configuration settings for the nginx-ingress addon.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.AdmissionPlugin">AdmissionPlugin
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>AdmissionPlugin contains information about a specific admission plugin and its corresponding configuration.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
bool
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
<h3 id="core.gardener.cloud/v1beta1.Alerting">Alerting
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Monitoring">Monitoring</a>)
</p>
<p>
<p>Alerting contains information about how alerting will be done (i.e. who will receive alerts and how).</p>
</p>
<table>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MonitoringEmailReceivers is a list of recipients for alerts</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.AuditConfig">AuditConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>AuditConfig contains settings for audit of the api server</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.AuditPolicy">
AuditPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuditPolicy contains configuration settings for audit policy of the kube-apiserver.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.AuditPolicy">AuditPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.AuditConfig">AuditConfig</a>)
</p>
<p>
<p>AuditPolicy contains audit policy for kube-apiserver</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfigMapRef is a reference to a ConfigMap object in the same namespace,
which contains the audit policy for the kube-apiserver.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.AuthorizerKubeconfigReference">AuthorizerKubeconfigReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.StructuredAuthorization">StructuredAuthorization</a>)
</p>
<p>
<p>AuthorizerKubeconfigReference is a reference for a kubeconfig for a authorization webhook.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.AvailabilityZone">AvailabilityZone
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Region">Region</a>)
</p>
<p>
<p>AvailabilityZone is an availability zone.</p>
</p>
<table>
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
[]string
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>UnavailableVolumeTypes is a list of volume type names that are not availability in this zone.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.BackupBucketProvider">BackupBucketProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupBucketSpec">BackupBucketSpec</a>)
</p>
<p>
<p>BackupBucketProvider holds the details of cloud provider of the object store.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.BackupBucketSpec">BackupBucketSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupBucket">BackupBucket</a>)
</p>
<p>
<p>BackupBucketSpec is the specification of a Backup Bucket.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.BackupBucketProvider">
BackupBucketProvider
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the configuration passed to BackupBucket resource.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret that contains the credentials to access object store.</p>
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
<p>SeedName holds the name of the seed allocated to BackupBucket for running controller.
This field is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.BackupBucketStatus">BackupBucketStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupBucket">BackupBucket</a>)
</p>
<p>
<p>BackupBucketStatus holds the most recently observed status of the Backup Bucket.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="#core.gardener.cloud/v1beta1.LastOperation">
LastOperation
</a>
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
<a href="#core.gardener.cloud/v1beta1.LastError">
LastError
</a>
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
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this BackupBucket. It corresponds to the
BackupBucket&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>generatedSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GeneratedSecretRef is reference to the secret generated by backup bucket, which
will have object store specific credentials.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.BackupEntrySpec">BackupEntrySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupEntry">BackupEntry</a>)
</p>
<p>
<p>BackupEntrySpec is the specification of a Backup Entry.</p>
</p>
<table>
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
<p>SeedName holds the name of the seed to which this BackupEntry is scheduled</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.BackupEntryStatus">BackupEntryStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupEntry">BackupEntry</a>)
</p>
<p>
<p>BackupEntryStatus holds the most recently observed status of the Backup Entry.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.LastOperation">
LastOperation
</a>
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
<a href="#core.gardener.cloud/v1beta1.LastError">
LastError
</a>
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
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this BackupEntry. It corresponds to the
BackupEntry&rsquo;s generation, which is updated on mutation by the API Server.</p>
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
<p>SeedName is the name of the seed to which this BackupEntry is currently scheduled. This field is populated
at the beginning of a create/reconcile operation. It is used when moving the BackupEntry between seeds.</p>
</td>
</tr>
<tr>
<td>
<code>migrationStartTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MigrationStartTime is the time when a migration to a different seed was initiated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Bastion">Bastion
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>Bastion contains the bastions creation info</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.BastionMachineImage">
BastionMachineImage
</a>
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
<a href="#core.gardener.cloud/v1beta1.BastionMachineType">
BastionMachineType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineType contains the bastions machine type properties</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.BastionMachineImage">BastionMachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Bastion">Bastion</a>)
</p>
<p>
<p>BastionMachineImage contains the bastions machine image properties</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.BastionMachineType">BastionMachineType
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Bastion">Bastion</a>)
</p>
<p>
<p>BastionMachineType contains the bastions machine type properties</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.CARotation">CARotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation</a>)
</p>
<p>
<p>CARotation contains information about the certificate authority credential rotation.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CredentialsRotationPhase">
CredentialsRotationPhase
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the certificate authority credential rotation was successfully
completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationFinishedTime is the recent time when the certificate authority credential rotation initiation was
completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTriggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTriggeredTime is the recent time when the certificate authority credential rotation completion was
triggered.</p>
</td>
</tr>
<tr>
<td>
<code>pendingWorkersRollouts</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.PendingWorkersRollout">
[]PendingWorkersRollout
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkersRollouts contains the name of a worker pool and the initiation time of their last rollout due to
credentials rotation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CRI">CRI
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MachineImageVersion">MachineImageVersion</a>, 
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>CRI contains information about the Container Runtimes.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CRIName">
CRIName
</a>
</em>
</td>
<td>
<p>The name of the CRI library. Supported values are <code>containerd</code>.</p>
</td>
</tr>
<tr>
<td>
<code>containerRuntimes</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ContainerRuntime">
[]ContainerRuntime
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContainerRuntimes is the list of the required container runtimes supported for a worker pool.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CRIName">CRIName
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CRI">CRI</a>)
</p>
<p>
<p>CRIName is a type alias for the CRI name string.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.Capabilities">Capabilities
(<code>map[string]..CapabilityValues</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CapabilitySet">CapabilitySet</a>, 
<a href="#core.gardener.cloud/v1beta1.MachineType">MachineType</a>)
</p>
<p>
<p>Capabilities of a machine type or machine image.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.CapabilityDefinition">CapabilityDefinition
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>CapabilityDefinition contains the Name and Values of a capability.</p>
</p>
<table>
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
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CapabilityValues">
CapabilityValues
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CapabilitySet">CapabilitySet
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MachineImageVersion">MachineImageVersion</a>)
</p>
<p>
<p>CapabilitySet is a wrapper for Capabilities.
This is a workaround as the Protobuf generator can&rsquo;t handle a slice of maps.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>-</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Capabilities">
Capabilities
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CapabilityValues">CapabilityValues
(<code>[]string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CapabilityDefinition">CapabilityDefinition</a>)
</p>
<p>
<p>CapabilityValues contains capability values.
This is a workaround as the Protobuf generator can&rsquo;t handle a map with slice values.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.CloudProfileReference">CloudProfileReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>CloudProfileReference holds the information about a CloudProfile or a NamespacedCloudProfile.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfile">CloudProfile</a>, 
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileStatus">NamespacedCloudProfileStatus</a>)
</p>
<p>
<p>CloudProfileSpec is the specification of a CloudProfile.
It must contain exactly one of its defined keys.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.KubernetesSettings">
KubernetesSettings
</a>
</em>
</td>
<td>
<p>Kubernetes contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.MachineImage">
[]MachineImage
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineType">
[]MachineType
</a>
</em>
</td>
<td>
<p>MachineTypes contains constraints regarding allowed values for machine types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Region">
[]Region
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSelector">
SeedSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector contains an optional list of labels on <code>Seed</code> resources that marks those seeds whose shoots may use this provider profile.
An empty list means that all seeds of the same provider type are supported.
This is useful for environments that are of the same type (like openstack) but may have different &ldquo;instances&rdquo;/landscapes.
Optionally a list of possible providers can be added to enable cross-provider scheduling. By default, the provider
type of the seed must match the shoot&rsquo;s provider.</p>
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
<a href="#core.gardener.cloud/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>bastion</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Bastion">
Bastion
</a>
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
<a href="#core.gardener.cloud/v1beta1.Limits">
Limits
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits configures operational limits for Shoot clusters using this CloudProfile.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md">https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md</a>.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CapabilityDefinition">
[]CapabilityDefinition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capabilities contains the definition of all possible capabilities in the CloudProfile.
Only capabilities and values defined here can be used to describe MachineImages and MachineTypes.
The order of values for a given capability is relevant. The most important value is listed first.
During maintenance upgrades, the image that matches most capabilities will be selected.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ClusterAutoscaler">ClusterAutoscaler
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>ClusterAutoscaler contains the configuration flags for the Kubernetes cluster autoscaler.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
float64
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="#core.gardener.cloud/v1beta1.ExpanderMode">
ExpanderMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Expander defines the algorithm to use during scale up (default: least-waste).
See: <a href="https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#what-are-expanders">https://github.com/gardener/autoscaler/blob/machine-controller-manager-provider/cluster-autoscaler/FAQ.md#what-are-expanders</a>.</p>
</td>
</tr>
<tr>
<td>
<code>maxNodeProvisionTime</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
int32
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IgnoreTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.
Deprecated: Ignore taints are deprecated as of K8S 1.29 and treated as startup taints</p>
</td>
</tr>
<tr>
<td>
<code>newPodScaleUpDelay</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxEmptyBulkDelete specifies the maximum number of empty nodes that can be deleted at the same time (default: 10).</p>
</td>
</tr>
<tr>
<td>
<code>ignoreDaemonsetsUtilization</code></br>
<em>
bool
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
int32
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartupTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.
Cluster Autoscaler treats nodes tainted with startup taints as unready, but taken into account during scale up logic, assuming they will become ready shortly.</p>
</td>
</tr>
<tr>
<td>
<code>statusTaints</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>StatusTaints specifies a list of taint keys to ignore in node templates when considering to scale a node group.
Cluster Autoscaler internally treats nodes tainted with status taints as ready, but filtered out during scale up logic.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ClusterAutoscalerOptions">ClusterAutoscalerOptions
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>ClusterAutoscalerOptions contains the cluster autoscaler configurations for a worker pool.</p>
</p>
<table>
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
float64
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
float64
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNodeProvisionTime defines how long CA waits for node to be provisioned.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Condition">Condition
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerInstallationStatus">ControllerInstallationStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedStatus">SeedStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>Condition holds the information about the state of a resource.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ConditionType">
ConditionType
</a>
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
<a href="#core.gardener.cloud/v1beta1.ConditionStatus">
ConditionStatus
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<p>The reason for the condition&rsquo;s last transition.</p>
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
<a href="#core.gardener.cloud/v1beta1.ErrorCode">
[]ErrorCode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Well-defined error codes in case the condition reports a problem.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ConditionStatus">ConditionStatus
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Condition">Condition</a>)
</p>
<p>
<p>ConditionStatus is the status of a condition.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ConditionType">ConditionType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Condition">Condition</a>)
</p>
<p>
<p>ConditionType is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ContainerRuntime">ContainerRuntime
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CRI">CRI</a>)
</p>
<p>
<p>ContainerRuntime contains information about worker&rsquo;s available container runtime</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the configuration passed to container runtime resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControlPlane">ControlPlane
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>ControlPlane holds information about the general settings for the control plane of a shoot.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.HighAvailability">
HighAvailability
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HighAvailability holds the configuration settings for high availability of the
control plane of a shoot.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControlPlaneAutoscaling">ControlPlaneAutoscaling
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ETCDConfig">ETCDConfig</a>, 
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>ControlPlaneAutoscaling contains auto-scaling configuration options for control-plane components.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>minAllowed</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>MinAllowed configures the minimum allowed resource requests for vertical pod autoscaling..
Configuration of minAllowed resources is an advanced feature that can help clusters to overcome scale-up delays.
Default values are not applied to this field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerDeploymentPolicy">ControllerDeploymentPolicy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationDeployment">ControllerRegistrationDeployment</a>)
</p>
<p>
<p>ControllerDeploymentPolicy is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ControllerInstallationSpec">ControllerInstallationSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerInstallation">ControllerInstallation</a>)
</p>
<p>
<p>ControllerInstallationSpec is the specification of a ControllerInstallation.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<p>RegistrationRef is used to reference a ControllerRegistration resource.
The name field of the RegistrationRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>seedRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<p>SeedRef is used to reference a Seed resource. The name field of the SeedRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>deploymentRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeploymentRef is used to reference a ControllerDeployment resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerInstallationStatus">ControllerInstallationStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerInstallation">ControllerInstallation</a>)
</p>
<p>
<p>ControllerInstallationStatus is the status of a ControllerInstallation.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.Condition">
[]Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a ControllerInstallations&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>providerStatus</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains type-specific status.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerRegistrationDeployment">ControllerRegistrationDeployment
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationSpec">ControllerRegistrationSpec</a>)
</p>
<p>
<p>ControllerRegistrationDeployment contains information for how this controller is deployed.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ControllerDeploymentPolicy">
ControllerDeploymentPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Policy controls how the controller is deployed. It defaults to &lsquo;OnDemand&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>seedSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector contains an optional label selector for seeds. Only if the labels match then this controller will be
considered for a deployment.
An empty list means that all seeds are selected.</p>
</td>
</tr>
<tr>
<td>
<code>deploymentRefs</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.DeploymentRef">
[]DeploymentRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeploymentRefs holds references to <code>ControllerDeployments</code>. Only one element is supported currently.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerRegistrationSpec">ControllerRegistrationSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistration">ControllerRegistration</a>)
</p>
<p>
<p>ControllerRegistrationSpec is the specification of a ControllerRegistration.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ControllerResource">
[]ControllerResource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of combinations of kinds (DNSProvider, Infrastructure, Generic, &hellip;) and their actual types
(aws-route53, gcp, auditlog, &hellip;).</p>
</td>
</tr>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationDeployment">
ControllerRegistrationDeployment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment contains information for how this controller is deployed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerResource">ControllerResource
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationSpec">ControllerRegistrationSpec</a>)
</p>
<p>
<p>ControllerResource is a combination of a kind (DNSProvider, Infrastructure, Generic, &hellip;) and the actual type for this
kind (aws-route53, gcp, auditlog, &hellip;).</p>
</p>
<table>
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
<p>Kind is the resource kind, for example &ldquo;OperatingSystemConfig&rdquo;.</p>
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
<p>Type is the resource type, for example &ldquo;coreos&rdquo; or &ldquo;ubuntu&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>globallyEnabled</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>GloballyEnabled determines if this ControllerResource is required by all Shoot clusters.
This field is defaulted to false when kind is &ldquo;Extension&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>reconcileTimeout</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.
This field is defaulted to 3m0s when kind is &ldquo;Extension&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>primary</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Primary determines if the controller backed by this ControllerRegistration is responsible for the extension
resource&rsquo;s lifecycle. This field defaults to true. There must be exactly one primary controller for this kind/type
combination. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControllerResourceLifecycle">
ControllerResourceLifecycle
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Lifecycle defines a strategy that determines when different operations on a ControllerResource should be performed.
This field is defaulted in the following way when kind is &ldquo;Extension&rdquo;.
Reconcile: &ldquo;AfterKubeAPIServer&rdquo;
Delete: &ldquo;BeforeKubeAPIServer&rdquo;
Migrate: &ldquo;BeforeKubeAPIServer&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>workerlessSupported</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkerlessSupported specifies whether this ControllerResource supports Workerless Shoot clusters.
This field is only relevant when kind is &ldquo;Extension&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerResourceLifecycle">ControllerResourceLifecycle
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerResource">ControllerResource</a>)
</p>
<p>
<p>ControllerResourceLifecycle defines the lifecycle of a controller resource.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ControllerResourceLifecycleStrategy">
ControllerResourceLifecycleStrategy
</a>
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
<a href="#core.gardener.cloud/v1beta1.ControllerResourceLifecycleStrategy">
ControllerResourceLifecycleStrategy
</a>
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
<a href="#core.gardener.cloud/v1beta1.ControllerResourceLifecycleStrategy">
ControllerResourceLifecycleStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Migrate defines the strategy during migration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ControllerResourceLifecycleStrategy">ControllerResourceLifecycleStrategy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerResourceLifecycle">ControllerResourceLifecycle</a>)
</p>
<p>
<p>ControllerResourceLifecycleStrategy is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.CoreDNS">CoreDNS
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SystemComponents">SystemComponents</a>)
</p>
<p>
<p>CoreDNS contains the settings of the Core DNS components running in the data plane of the Shoot cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CoreDNSAutoscaling">
CoreDNSAutoscaling
</a>
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
<a href="#core.gardener.cloud/v1beta1.CoreDNSRewriting">
CoreDNSRewriting
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Rewriting contains the setting related to rewriting of requests, which are obviously incorrect due to the unnecessary application of the search path.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CoreDNSAutoscaling">CoreDNSAutoscaling
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CoreDNS">CoreDNS</a>)
</p>
<p>
<p>CoreDNSAutoscaling contains the settings related to autoscaling of the Core DNS components running in the data plane of the Shoot cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CoreDNSAutoscalingMode">
CoreDNSAutoscalingMode
</a>
</em>
</td>
<td>
<p>The mode of the autoscaling to be used for the Core DNS components running in the data plane of the Shoot cluster.
Supported values are <code>horizontal</code> and <code>cluster-proportional</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CoreDNSAutoscalingMode">CoreDNSAutoscalingMode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CoreDNSAutoscaling">CoreDNSAutoscaling</a>)
</p>
<p>
<p>CoreDNSAutoscalingMode is a type alias for the Core DNS autoscaling mode string.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.CoreDNSRewriting">CoreDNSRewriting
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CoreDNS">CoreDNS</a>)
</p>
<p>
<p>CoreDNSRewriting contains the setting related to rewriting requests, which are obviously incorrect due to the unnecessary application of the search path.</p>
</p>
<table>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CommonSuffixes are expected to be the suffix of a fully qualified domain name. Each suffix should contain at least one or two dots (&lsquo;.&rsquo;) to prevent accidental clashes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.CredentialsRotationPhase">CredentialsRotationPhase
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CARotation">CARotation</a>, 
<a href="#core.gardener.cloud/v1beta1.ETCDEncryptionKeyRotation">ETCDEncryptionKeyRotation</a>, 
<a href="#core.gardener.cloud/v1beta1.ServiceAccountKeyRotation">ServiceAccountKeyRotation</a>)
</p>
<p>
<p>CredentialsRotationPhase is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.DNS">DNS
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>DNS holds information about the provider, the hosted zone id and the domain.</p>
</p>
<table>
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
<p>Domain is the external available domain of the Shoot cluster. This domain will be written into the
kubeconfig that is handed out to end-users. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>providers</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.DNSProvider">
[]DNSProvider
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Providers is a list of DNS providers that shall be enabled for this shoot cluster. Only relevant if
not a default domain is used.</p>
<p>Deprecated: Configuring multiple DNS providers is deprecated and will be forbidden in a future release.
Please use the DNS extension provider config (e.g. shoot-dns-service) for additional providers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.DNSIncludeExclude">DNSIncludeExclude
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.DNSProvider">DNSProvider</a>)
</p>
<p>
<p>DNSIncludeExclude contains information about which domains shall be included/excluded.</p>
</p>
<table>
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
[]string
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Exclude is a list of domains that shall be excluded.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.DNSProvider">DNSProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.DNS">DNS</a>)
</p>
<p>
<p>DNSProvider contains information about a DNS provider.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.DNSIncludeExclude">
DNSIncludeExclude
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Domains contains information about which domains shall be included/excluded for this provider.</p>
<p>Deprecated: This field is deprecated and will be removed in a future release.
Please use the DNS extension provider config (e.g. shoot-dns-service) for additional configuration.</p>
</td>
</tr>
<tr>
<td>
<code>primary</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Primary indicates that this DNSProvider is used for shoot related domains.</p>
<p>Deprecated: This field is deprecated and will be removed in a future release.
Please use the DNS extension provider config (e.g. shoot-dns-service) for additional and non-primary providers.</p>
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
<p>SecretName is a name of a secret containing credentials for the stated domain and the
provider. When not specified, the Gardener will use the cloud provider credentials referenced
by the Shoot and try to find respective credentials there (primary provider only). Specifying this field may override
this behavior, i.e. forcing the Gardener to only look into the given secret.</p>
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
<a href="#core.gardener.cloud/v1beta1.DNSIncludeExclude">
DNSIncludeExclude
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones contains information about which hosted zones shall be included/excluded for this provider.</p>
<p>Deprecated: This field is deprecated and will be removed in a future release.
Please use the DNS extension provider config (e.g. shoot-dns-service) for additional configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.DataVolume">DataVolume
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>DataVolume contains information about a data volume.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Encrypted determines if the volume should be encrypted.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.DeploymentRef">DeploymentRef
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControllerRegistrationDeployment">ControllerRegistrationDeployment</a>)
</p>
<p>
<p>DeploymentRef contains information about <code>ControllerDeployment</code> references.</p>
</p>
<table>
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
<p>Name is the name of the <code>ControllerDeployment</code> that is being referred to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.DualApprovalForDeletion">DualApprovalForDeletion
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ProjectSpec">ProjectSpec</a>)
</p>
<p>
<p>DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeServiceAccounts specifies whether the concept also applies when deletion is triggered by ServiceAccounts.
Defaults to true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ETCD">ETCD
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>ETCD contains configuration for etcds of the shoot cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ETCDConfig">
ETCDConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.ETCDConfig">
ETCDConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Events contains configuration for the events etcd.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ETCDConfig">ETCDConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ETCD">ETCD</a>)
</p>
<p>
<p>ETCDConfig contains etcd configuration.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ControlPlaneAutoscaling">
ControlPlaneAutoscaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains auto-scaling configuration options for etcd.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ETCDEncryptionKeyRotation">ETCDEncryptionKeyRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation</a>)
</p>
<p>
<p>ETCDEncryptionKeyRotation contains information about the ETCD encryption key credential rotation.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CredentialsRotationPhase">
CredentialsRotationPhase
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the ETCD encryption key credential rotation was successfully
completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationFinishedTime is the recent time when the ETCD encryption key credential rotation initiation was
completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTriggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTriggeredTime is the recent time when the ETCD encryption key credential rotation completion was
triggered.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.EncryptionConfig">EncryptionConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>EncryptionConfig contains customizable encryption configuration of the API server.</p>
</p>
<table>
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
[]string
</em>
</td>
<td>
<p>Resources contains the list of resources that shall be encrypted in addition to secrets.
Each item is a Kubernetes resource name in plural (resource or resource.group) that should be encrypted.
Wildcards are not supported for now.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md">https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md</a> for more details.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ErrorCode">ErrorCode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Condition">Condition</a>, 
<a href="#core.gardener.cloud/v1beta1.LastError">LastError</a>)
</p>
<p>
<p>ErrorCode is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ExpanderMode">ExpanderMode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ClusterAutoscaler">ClusterAutoscaler</a>)
</p>
<p>
<p>ExpanderMode is type used for Expander values</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ExpirableVersion">ExpirableVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubernetesSettings">KubernetesSettings</a>, 
<a href="#core.gardener.cloud/v1beta1.MachineImageVersion">MachineImageVersion</a>)
</p>
<p>
<p>ExpirableVersion contains a version and an expiration date.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationDate defines the time at which this version expires.</p>
</td>
</tr>
<tr>
<td>
<code>classification</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.VersionClassification">
VersionClassification
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Classification defines the state of a version (preview, supported, deprecated)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ExposureClassScheduling">ExposureClassScheduling
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ExposureClass">ExposureClass</a>)
</p>
<p>
<p>ExposureClassScheduling holds information to select applicable Seed&rsquo;s for ExposureClass usage.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SeedSelector">
SeedSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector is an optional label selector for Seed&rsquo;s which are suitable to use the ExposureClass.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Toleration">
[]Toleration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations contains the tolerations for taints on Seed clusters.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Extension">Extension
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Extension contains type and provider information for extensions.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Disabled allows to disable extensions that were marked as &lsquo;globally enabled&rsquo; by Gardener administrators.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ExtensionResourceState">ExtensionResourceState
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStateSpec">ShootStateSpec</a>)
</p>
<p>
<p>ExtensionResourceState contains the kind of the extension custom resource and its last observed state in the Shoot&rsquo;s
namespace on the Seed cluster.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in the state by their names.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.FailureTolerance">FailureTolerance
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.HighAvailability">HighAvailability</a>)
</p>
<p>
<p>FailureTolerance describes information about failure tolerance level of a highly available resource.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.FailureToleranceType">
FailureToleranceType
</a>
</em>
</td>
<td>
<p>Type specifies the type of failure that the highly available resource can tolerate</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.FailureToleranceType">FailureToleranceType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.FailureTolerance">FailureTolerance</a>)
</p>
<p>
<p>FailureToleranceType specifies the type of failure that a highly available
shoot control plane that can tolerate.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.Gardener">Gardener
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedStatus">SeedStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>Gardener holds the information about the Gardener version that operated a resource.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.GardenerResourceData">GardenerResourceData
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStateSpec">ShootStateSpec</a>)
</p>
<p>
<p>GardenerResourceData holds the data which is used to generate resources, deployed in the Shoot&rsquo;s control plane.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels are labels of the object</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.HelmControllerDeployment">HelmControllerDeployment
</h3>
<p>
<p>HelmControllerDeployment configures how an extension controller is deployed using helm.
This is the legacy structure that used to be defined in gardenlet&rsquo;s ControllerInstallation controller for
ControllerDeployment&rsquo;s with type=helm.
While this is not a proper API type, we need to define the structure in the API package so that we can convert it
to the internal API version in the new representation.</p>
</p>
<table>
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
[]byte
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#json-v1-apiextensions-k8s-io">
Kubernetes apiextensions/v1.JSON
</a>
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
<a href="#core.gardener.cloud/v1beta1.OCIRepository">
OCIRepository
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OCIRepository defines where to pull the chart.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Hibernation">Hibernation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Hibernation contains information whether the Shoot is suspended or not.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enabled specifies whether the Shoot needs to be hibernated or not. If it is true, the Shoot&rsquo;s desired state is to be hibernated.
If it is false or nil, the Shoot&rsquo;s desired state is to be awakened.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.HibernationSchedule">
[]HibernationSchedule
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Schedules determine the hibernation schedules.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.HibernationSchedule">HibernationSchedule
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Hibernation">Hibernation</a>)
</p>
<p>
<p>HibernationSchedule determines the hibernation schedule of a Shoot.
A Shoot will be regularly hibernated at each start time and will be woken up at each end time.
Start or End can be omitted, though at least one of each has to be specified.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.HighAvailability">HighAvailability
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ControlPlane">ControlPlane</a>)
</p>
<p>
<p>HighAvailability specifies the configuration settings for high availability for a resource. Typical
usages could be to configure HA for shoot control plane or for seed system components.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.FailureTolerance">
FailureTolerance
</a>
</em>
</td>
<td>
<p>FailureTolerance holds information about failure tolerance level of a highly available resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.HorizontalPodAutoscalerConfig">HorizontalPodAutoscalerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeControllerManagerConfig">KubeControllerManagerConfig</a>)
</p>
<p>
<p>HorizontalPodAutoscalerConfig contains horizontal pod autoscaler configuration settings for the kube-controller-manager.
Note: Descriptions were taken from the Kubernetes documentation.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>The minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.IPFamily">IPFamily
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Networking">Networking</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedNetworks">SeedNetworks</a>)
</p>
<p>
<p>IPFamily is a type for specifying an IP protocol version to use in Gardener clusters.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.InPlaceUpdates">InPlaceUpdates
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MachineImageVersion">MachineImageVersion</a>)
</p>
<p>
<p>InPlaceUpdates contains the configuration for in-place updates for a machine image version.</p>
</p>
<table>
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
bool
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
<h3 id="core.gardener.cloud/v1beta1.InPlaceUpdatesStatus">InPlaceUpdatesStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>InPlaceUpdatesStatus contains information about in-place updates for the Shoot workers.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.PendingWorkerUpdates">
PendingWorkerUpdates
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkerUpdates contains information about worker pools pending in-place updates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Ingress">Ingress
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>Ingress configures the Ingress specific settings of the cluster</p>
</p>
<table>
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
<p>Domain specifies the IngressDomain of the cluster pointing to the ingress controller endpoint. It will be used
to construct ingress URLs for system applications running in Shoot/Garden clusters. Once set this field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>controller</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.IngressController">
IngressController
</a>
</em>
</td>
<td>
<p>Controller configures a Gardener managed Ingress Controller listening on the ingressDomain</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.IngressController">IngressController
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Ingress">Ingress</a>)
</p>
<p>
<p>IngressController enables a Gardener managed Ingress Controller listening on the ingressDomain</p>
</p>
<table>
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
<p>Kind defines which kind of IngressController to use. At the moment only <code>nginx</code> is supported</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig specifies infrastructure specific configuration for the ingressController</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>KubeAPIServerConfig contains configuration settings for the kube-apiserver.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KubernetesConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubernetesConfig">
KubernetesConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>KubernetesConfig</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>admissionPlugins</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AdmissionPlugin">
[]AdmissionPlugin
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener), and, if desired, the corresponding
configuration.</p>
</td>
</tr>
<tr>
<td>
<code>apiAudiences</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIAudiences are the identifiers of the API. The service account token authenticator will
validate that tokens used against the API are bound to at least one of these audiences.
Defaults to [&ldquo;kubernetes&rdquo;].</p>
</td>
</tr>
<tr>
<td>
<code>auditConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AuditConfig">
AuditConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.OIDCConfig">
OIDCConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OIDCConfig contains configuration settings for the OIDC provider.</p>
<p>Deprecated: This field is deprecated and will be forbidden starting from Kubernetes 1.32.
Please configure and use structured authentication instead of oidc flags.
For more information check <a href="https://github.com/gardener/gardener/issues/9858">https://github.com/gardener/gardener/issues/9858</a>
TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.31 is dropped.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeConfig</code></br>
<em>
map[string]bool
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
<a href="#core.gardener.cloud/v1beta1.ServiceAccountConfig">
ServiceAccountConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountConfig contains configuration settings for the service account handling
of the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>watchCacheSizes</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.WatchCacheSizes">
WatchCacheSizes
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WatchCacheSizes contains configuration of the API server&rsquo;s watch cache sizes.
Configuring these flags might be useful for large-scale Shoot clusters with a lot of parallel update requests
and a lot of watching controllers (e.g. large ManagedSeed clusters). When the API server&rsquo;s watch cache&rsquo;s
capacity is too small to cope with the amount of update requests and watchers for a particular resource, it
might happen that controller watches are permanently stopped with <code>too old resource version</code> errors.
Starting from kubernetes v1.19, the API server&rsquo;s watch cache size is adapted dynamically and setting the watch
cache size flags will have no effect, except when setting it to 0 (which disables the watch cache).</p>
</td>
</tr>
<tr>
<td>
<code>requests</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.APIServerRequests">
APIServerRequests
</a>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deprecated: This field is deprecated and will be removed in a future release.
Please use anonymous authentication configuration instead.
For more information see: <a href="https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration">https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration</a>
TODO(marc1404): Forbid this field when the feature gate AnonymousAuthConfigurableEndpoints has graduated.
EnableAnonymousAuthentication defines whether anonymous requests to the secure port
of the API server should be allowed (flag <code>--anonymous-auth</code>).
See: <a href="https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/">https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/</a></p>
</td>
</tr>
<tr>
<td>
<code>eventTTL</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EventTTL controls the amount of time to retain events.
Defaults to 1h.</p>
</td>
</tr>
<tr>
<td>
<code>logging</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.APIServerLogging">
APIServerLogging
</a>
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
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultNotReadyTolerationSeconds indicates the tolerationSeconds of the toleration for notReady:NoExecute
that is added by default to every pod that does not already have such a toleration (flag <code>--default-not-ready-toleration-seconds</code>).
The field has effect only when the <code>DefaultTolerationSeconds</code> admission plugin is enabled.
Defaults to 300.</p>
</td>
</tr>
<tr>
<td>
<code>defaultUnreachableTolerationSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultUnreachableTolerationSeconds indicates the tolerationSeconds of the toleration for unreachable:NoExecute
that is added by default to every pod that does not already have such a toleration (flag <code>--default-unreachable-toleration-seconds</code>).
The field has effect only when the <code>DefaultTolerationSeconds</code> admission plugin is enabled.
Defaults to 300.</p>
</td>
</tr>
<tr>
<td>
<code>encryptionConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.EncryptionConfig">
EncryptionConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.StructuredAuthentication">
StructuredAuthentication
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StructuredAuthentication contains configuration settings for structured authentication for the kube-apiserver.
This field is only available for Kubernetes v1.30 or later.</p>
</td>
</tr>
<tr>
<td>
<code>structuredAuthorization</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.StructuredAuthorization">
StructuredAuthorization
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StructuredAuthorization contains configuration settings for structured authorization for the kube-apiserver.
This field is only available for Kubernetes v1.30 or later.</p>
</td>
</tr>
<tr>
<td>
<code>autoscaling</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ControlPlaneAutoscaling">
ControlPlaneAutoscaling
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains auto-scaling configuration options for the kube-apiserver.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeControllerManagerConfig">KubeControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>KubeControllerManagerConfig contains configuration settings for the kube-controller-manager.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KubernetesConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubernetesConfig">
KubernetesConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>KubernetesConfig</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>horizontalPodAutoscaler</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.HorizontalPodAutoscalerConfig">
HorizontalPodAutoscalerConfig
</a>
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
int32
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodEvictionTimeout defines the grace period for deleting pods on failed nodes. Defaults to 2m.</p>
<p>Deprecated: The corresponding kube-controller-manager flag <code>--pod-eviction-timeout</code> is deprecated
in favor of the kube-apiserver flags <code>--default-not-ready-toleration-seconds</code> and <code>--default-unreachable-toleration-seconds</code>.
The <code>--pod-eviction-timeout</code> flag does not have effect when the taint based eviction is enabled. The taint
based eviction is beta (enabled by default) since Kubernetes 1.13 and GA since Kubernetes 1.18. Hence,
instead of setting this field, set the <code>spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds</code> and
<code>spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds</code>. This field will be removed in gardener v1.120.</p>
</td>
</tr>
<tr>
<td>
<code>nodeMonitorGracePeriod</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeMonitorGracePeriod defines the grace period before an unresponsive node is marked unhealthy.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeProxyConfig">KubeProxyConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>KubeProxyConfig contains configuration settings for the kube-proxy.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KubernetesConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubernetesConfig">
KubernetesConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>KubernetesConfig</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>mode</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProxyMode">
ProxyMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mode specifies which proxy mode to use.
defaults to IPTables.</p>
</td>
</tr>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enabled indicates whether kube-proxy should be deployed or not.
Depending on the networking extensions switching kube-proxy off might be rejected. Consulting the respective documentation of the used networking extension is recommended before using this field.
defaults to true if not specified.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeSchedulerConfig">KubeSchedulerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>KubeSchedulerConfig contains configuration settings for the kube-scheduler.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KubernetesConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubernetesConfig">
KubernetesConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>KubernetesConfig</code> are embedded into this type.)
</p>
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
<p>KubeMaxPDVols allows to configure the <code>KUBE_MAX_PD_VOLS</code> environment variable for the kube-scheduler.
Please find more information here: <a href="https://kubernetes.io/docs/concepts/storage/storage-limits/#custom-limits">https://kubernetes.io/docs/concepts/storage/storage-limits/#custom-limits</a>
Note that using this field is considered alpha-/experimental-level and is on your own risk. You should be aware
of all the side-effects and consequences when changing it.</p>
</td>
</tr>
<tr>
<td>
<code>profile</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SchedulingProfile">
SchedulingProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Profile configures the scheduling profile for the cluster.
If not specified, the used profile is &ldquo;balanced&rdquo; (provides the default kube-scheduler behavior).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>, 
<a href="#core.gardener.cloud/v1beta1.WorkerKubernetes">WorkerKubernetes</a>)
</p>
<p>
<p>KubeletConfig contains configuration settings for the kubelet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>KubernetesConfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubernetesConfig">
KubernetesConfig
</a>
</em>
</td>
<td>
<p>
(Members of <code>KubernetesConfig</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>cpuCFSQuota</code></br>
<em>
bool
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
<a href="#core.gardener.cloud/v1beta1.KubeletConfigEviction">
KubeletConfigEviction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionHard describes a set of eviction thresholds (e.g. memory.available<1Gi) that if met would trigger a Pod eviction.
Default:
memory.available:   &ldquo;100Mi/1Gi/5%&rdquo;
nodefs.available:   &ldquo;5%&rdquo;
nodefs.inodesFree:  &ldquo;5%&rdquo;
imagefs.available:  &ldquo;5%&rdquo;
imagefs.inodesFree: &ldquo;5%&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>evictionMaxPodGracePeriod</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionMaxPodGracePeriod describes the maximum allowed grace period (in seconds) to use when terminating pods in response to a soft eviction threshold being met.
Default: 90</p>
</td>
</tr>
<tr>
<td>
<code>evictionMinimumReclaim</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfigEvictionMinimumReclaim">
KubeletConfigEvictionMinimumReclaim
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionMinimumReclaim configures the amount of resources below the configured eviction threshold that the kubelet attempts to reclaim whenever the kubelet observes resource pressure.
Default: 0 for each resource</p>
</td>
</tr>
<tr>
<td>
<code>evictionPressureTransitionPeriod</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionPressureTransitionPeriod is the duration for which the kubelet has to wait before transitioning out of an eviction pressure condition.
Default: 4m0s</p>
</td>
</tr>
<tr>
<td>
<code>evictionSoft</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfigEviction">
KubeletConfigEviction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionSoft describes a set of eviction thresholds (e.g. memory.available<1.5Gi) that if met over a corresponding grace period would trigger a Pod eviction.
Default:
memory.available:   &ldquo;200Mi/1.5Gi/10%&rdquo;
nodefs.available:   &ldquo;10%&rdquo;
nodefs.inodesFree:  &ldquo;10%&rdquo;
imagefs.available:  &ldquo;10%&rdquo;
imagefs.inodesFree: &ldquo;10%&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>evictionSoftGracePeriod</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfigEvictionSoftGracePeriod">
KubeletConfigEvictionSoftGracePeriod
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionSoftGracePeriod describes a set of eviction grace periods (e.g. memory.available=1m30s) that correspond to how long a soft eviction threshold must hold before triggering a Pod eviction.
Default:
memory.available:   1m30s
nodefs.available:   1m30s
nodefs.inodesFree:  1m30s
imagefs.available:  1m30s
imagefs.inodesFree: 1m30s</p>
</td>
</tr>
<tr>
<td>
<code>maxPods</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxPods is the maximum number of Pods that are allowed by the Kubelet.
Default: 110</p>
</td>
</tr>
<tr>
<td>
<code>podPidsLimit</code></br>
<em>
int64
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
bool
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
<a href="#core.gardener.cloud/v1beta1.KubeletConfigReserved">
KubeletConfigReserved
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeReserved is the configuration for resources reserved for kubernetes node components (mainly kubelet and container runtime).
When updating these values, be aware that cgroup resizes may not succeed on active worker nodes. Look for the NodeAllocatableEnforced event to determine if the configuration was applied.
Default: cpu=80m,memory=1Gi,pid=20k</p>
</td>
</tr>
<tr>
<td>
<code>systemReserved</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfigReserved">
KubeletConfigReserved
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SystemReserved is the configuration for resources reserved for system processes not managed by kubernetes (e.g. journald).
When updating these values, be aware that cgroup resizes may not succeed on active worker nodes. Look for the NodeAllocatableEnforced event to determine if the configuration was applied.</p>
<p>Deprecated: Separately configuring resource reservations for system processes is deprecated in Gardener and will be forbidden starting from Kubernetes 1.31.
Please merge existing resource reservations into the kubeReserved field.
TODO(MichaelEischer): Drop this field after support for Kubernetes 1.30 is dropped.</p>
</td>
</tr>
<tr>
<td>
<code>imageGCHighThresholdPercent</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageGCHighThresholdPercent describes the percent of the disk usage which triggers image garbage collection.
Default: 50</p>
</td>
</tr>
<tr>
<td>
<code>imageGCLowThresholdPercent</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageGCLowThresholdPercent describes the percent of the disk to which garbage collection attempts to free.
Default: 40</p>
</td>
</tr>
<tr>
<td>
<code>serializeImagePulls</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>SerializeImagePulls describes whether the images are pulled one at a time.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>registryPullQPS</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>RegistryPullQPS is the limit of registry pulls per second. The value must not be a negative number.
Setting it to 0 means no limit.
Default: 5</p>
</td>
</tr>
<tr>
<td>
<code>registryBurst</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>RegistryBurst is the maximum size of bursty pulls, temporarily allows pulls to burst to this number,
while still not exceeding registryPullQPS. The value must not be a negative number.
Only used if registryPullQPS is greater than 0.
Default: 10</p>
</td>
</tr>
<tr>
<td>
<code>seccompDefault</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeccompDefault enables the use of <code>RuntimeDefault</code> as the default seccomp profile for all workloads.</p>
</td>
</tr>
<tr>
<td>
<code>containerLogMaxSize</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A quantity defines the maximum size of the container log file before it is rotated. For example: &ldquo;5Mi&rdquo; or &ldquo;256Ki&rdquo;.
Default: 100Mi</p>
</td>
</tr>
<tr>
<td>
<code>containerLogMaxFiles</code></br>
<em>
int32
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProtectKernelDefaults ensures that the kernel tunables are equal to the kubelet defaults.
Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>streamingConnectionIdleTimeout</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StreamingConnectionIdleTimeout is the maximum time a streaming connection can be idle before the connection is automatically closed.
This field cannot be set lower than &ldquo;30s&rdquo; or greater than &ldquo;4h&rdquo;.
Default: &ldquo;5m&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>memorySwap</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.MemorySwapConfiguration">
MemorySwapConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemorySwap configures swap memory available to container workloads.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeletConfigEviction">KubeletConfigEviction
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>KubeletConfigEviction contains kubelet eviction thresholds supporting either a resource.Quantity or a percentage based value.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.KubeletConfigEvictionMinimumReclaim">KubeletConfigEvictionMinimumReclaim
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>KubeletConfigEvictionMinimumReclaim contains configuration for the kubelet eviction minimum reclaim.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSInodesFree is the threshold for the inodes reclaim in the nodefs filesystem.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeletConfigEvictionSoftGracePeriod">KubeletConfigEvictionSoftGracePeriod
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>KubeletConfigEvictionSoftGracePeriod contains grace periods for kubelet eviction thresholds.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeFSInodesFree is the grace period for the NodeFSInodesFree eviction threshold.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubeletConfigReserved">KubeletConfigReserved
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>KubeletConfigReserved contains reserved resources for daemons</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PID is the reserved process-ids.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Kubernetes">Kubernetes
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Kubernetes contains the version and configuration variables for the Shoot control plane.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ClusterAutoscaler">
ClusterAutoscaler
</a>
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
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">
KubeAPIServerConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.KubeControllerManagerConfig">
KubeControllerManagerConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.KubeSchedulerConfig">
KubeSchedulerConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.KubeProxyConfig">
KubeProxyConfig
</a>
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
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">
KubeletConfig
</a>
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
<p>Version is the semantic Kubernetes version to use for the Shoot cluster.
Defaults to the highest supported minor and patch version given in the referenced cloud profile.
The version can be omitted completely or partially specified, e.g. <code>&lt;major&gt;.&lt;minor&gt;</code>.</p>
</td>
</tr>
<tr>
<td>
<code>verticalPodAutoscaler</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.VerticalPodAutoscaler">
VerticalPodAutoscaler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>enableStaticTokenKubeconfig</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableStaticTokenKubeconfig indicates whether static token kubeconfig secret will be created for the Shoot cluster.
Setting this field to true is not supported.</p>
<p>Deprecated: This field is deprecated and will be removed in gardener v1.120</p>
</td>
</tr>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ETCD">
ETCD
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCD contains configuration for etcds of the shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubernetesConfig">KubernetesConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>, 
<a href="#core.gardener.cloud/v1beta1.KubeControllerManagerConfig">KubeControllerManagerConfig</a>, 
<a href="#core.gardener.cloud/v1beta1.KubeProxyConfig">KubeProxyConfig</a>, 
<a href="#core.gardener.cloud/v1beta1.KubeSchedulerConfig">KubeSchedulerConfig</a>, 
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>KubernetesConfig contains common configuration fields for the control plane components.</p>
</p>
<table>
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
map[string]bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.KubernetesDashboard">KubernetesDashboard
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>KubernetesDashboard describes configuration values for the kubernetes-dashboard addon.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Addon</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Addon">
Addon
</a>
</em>
</td>
<td>
<p>
(Members of <code>Addon</code> are embedded into this type.)
</p>
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
<h3 id="core.gardener.cloud/v1beta1.KubernetesSettings">KubernetesSettings
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec</a>)
</p>
<p>
<p>KubernetesSettings contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ExpirableVersion">
[]ExpirableVersion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.LastError">LastError
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupBucketStatus">BackupBucketStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.BackupEntryStatus">BackupEntryStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>LastError indicates the last occurred error for an operation on a resource.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ErrorCode">
[]ErrorCode
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Last time the error was reported</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.LastMaintenance">LastMaintenance
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>LastMaintenance holds information about a maintenance operation on the Shoot.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="#core.gardener.cloud/v1beta1.LastOperationState">
LastOperationState
</a>
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
<h3 id="core.gardener.cloud/v1beta1.LastOperation">LastOperation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.BackupBucketStatus">BackupBucketStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.BackupEntryStatus">BackupEntryStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedStatus">SeedStatus</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>LastOperation indicates the type and the state of the last operation, along with a description
message and a progress indicator.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
int32
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
<a href="#core.gardener.cloud/v1beta1.LastOperationState">
LastOperationState
</a>
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
<a href="#core.gardener.cloud/v1beta1.LastOperationType">
LastOperationType
</a>
</em>
</td>
<td>
<p>Type of the last operation, one of Create, Reconcile, Delete, Migrate, Restore.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.LastOperationState">LastOperationState
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.LastMaintenance">LastMaintenance</a>, 
<a href="#core.gardener.cloud/v1beta1.LastOperation">LastOperation</a>)
</p>
<p>
<p>LastOperationState is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.LastOperationType">LastOperationType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.LastOperation">LastOperation</a>)
</p>
<p>
<p>LastOperationType is a string alias.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.Limits">Limits
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec</a>)
</p>
<p>
<p>Limits configures operational limits for Shoot clusters using this CloudProfile.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md">https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md</a>.</p>
</p>
<table>
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxNodesTotal configures the maximum node count a Shoot cluster can have during runtime.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.LoadBalancerServicesProxyProtocol">LoadBalancerServicesProxyProtocol
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServices">SeedSettingLoadBalancerServices</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServicesZones">SeedSettingLoadBalancerServicesZones</a>)
</p>
<p>
<p>LoadBalancerServicesProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Allowed controls whether the ProxyProtocol is optionally allowed for the load balancer services.
This should only be enabled if the load balancer services are already using ProxyProtocol or will be reconfigured to use it soon.
Until the load balancers are configured with ProxyProtocol, enabling this setting may allow clients to spoof their source IP addresses.
The option allows a migration from non-ProxyProtocol to ProxyProtocol without downtime (depending on the infrastructure).
Defaults to false.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Machine">Machine
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>Machine contains information about the machine type and image.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image holds information about the machine image to use for all nodes of this pool. It will default to the
latest version of the first image stated in the referenced CloudProfile if no value has been provided.</p>
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
<h3 id="core.gardener.cloud/v1beta1.MachineControllerManagerSettings">MachineControllerManagerSettings
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>MachineControllerManagerSettings contains configurations for different worker-pools. Eg. MachineDrainTimeout, MachineHealthTimeout.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
int32
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
[]string
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableHealthTimeout if set to true, health timeout will be ignored. Leading to machine never being declared failed.
This is intended to be used only for in-place updates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MachineImage">MachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec</a>)
</p>
<p>
<p>MachineImage defines the name and multiple versions of the machine image in any environment.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.MachineImageVersion">
[]MachineImageVersion
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineImageUpdateStrategy">
MachineImageUpdateStrategy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UpdateStrategy is the update strategy to use for the machine image. Possible values are:
- patch: update to the latest patch version of the current minor version.
- minor: update to the latest minor and patch version.
- major: always update to the overall latest version (default).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MachineImageUpdateStrategy">MachineImageUpdateStrategy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MachineImage">MachineImage</a>)
</p>
<p>
<p>MachineImageUpdateStrategy is the update strategy to use for a machine image</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.MachineImageVersion">MachineImageVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MachineImage">MachineImage</a>)
</p>
<p>
<p>MachineImageVersion is an expirable version with list of supported container runtimes and interfaces</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ExpirableVersion</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ExpirableVersion">
ExpirableVersion
</a>
</em>
</td>
<td>
<p>
(Members of <code>ExpirableVersion</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>cri</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CRI">
[]CRI
</a>
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
[]string
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
<p>KubeletVersionConstraint is a constraint describing the supported kubelet versions by the machine image in this version.
If the field is not specified, it is assumed that the machine image in this version supports all kubelet versions.
Examples:
- &lsquo;&gt;= 1.26&rsquo; - supports only kubelet versions greater than or equal to 1.26
- &lsquo;&lt; 1.26&rsquo; - supports only kubelet versions less than 1.26</p>
</td>
</tr>
<tr>
<td>
<code>inPlaceUpdates</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.InPlaceUpdates">
InPlaceUpdates
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InPlaceUpdates contains the configuration for in-place updates for this machine image version.</p>
</td>
</tr>
<tr>
<td>
<code>capabilitySets</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CapabilitySet">
[]CapabilitySet
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CapabilitySets is an array of capability sets. Each entry represents a combination of capabilities that is provided by
the machine image version.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MachineType">MachineType
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec</a>)
</p>
<p>
<p>MachineType contains certain properties of a machine type.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineTypeStorage">
MachineTypeStorage
</a>
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
bool
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
<a href="#core.gardener.cloud/v1beta1.Capabilities">
Capabilities
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capabilities contains the machine type capabilities.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MachineTypeStorage">MachineTypeStorage
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MachineType">MachineType</a>)
</p>
<p>
<p>MachineTypeStorage is the amount of storage associated with the root volume of this machine type.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinSize is the minimal supported storage size.
This overrides any other common minimum size configuration from <code>spec.volumeTypes[*].minSize</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MachineUpdateStrategy">MachineUpdateStrategy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>MachineUpdateStrategy specifies the machine update strategy for the worker pool.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.Maintenance">Maintenance
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Maintenance contains information about the time window for maintenance operations and which
operations should be performed.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.MaintenanceAutoUpdate">
MaintenanceAutoUpdate
</a>
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
<a href="#core.gardener.cloud/v1beta1.MaintenanceTimeWindow">
MaintenanceTimeWindow
</a>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConfineSpecUpdateRollout prevents that changes/updates to the shoot specification will be rolled out immediately.
Instead, they are rolled out during the shoot&rsquo;s maintenance time window. There is one exception that will trigger
an immediate roll out which is changes to the Spec.Hibernation.Enabled field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MaintenanceAutoUpdate">MaintenanceAutoUpdate
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Maintenance">Maintenance</a>)
</p>
<p>
<p>MaintenanceAutoUpdate contains information about which constraints should be automatically updated.</p>
</p>
<table>
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
bool
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImageVersion indicates whether the machine image version may be automatically updated (default: true).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MaintenanceTimeWindow">MaintenanceTimeWindow
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Maintenance">Maintenance</a>)
</p>
<p>
<p>MaintenanceTimeWindow contains information about the time window for maintenance operations.</p>
</p>
<table>
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
<p>Begin is the beginning of the time window in the format HHMMSS+ZONE, e.g. &ldquo;220000+0100&rdquo;.
If not present, a random value will be computed.</p>
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
<p>End is the end of the time window in the format HHMMSS+ZONE, e.g. &ldquo;220000+0100&rdquo;.
If not present, the value will be computed based on the &ldquo;Begin&rdquo; value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.MemorySwapConfiguration">MemorySwapConfiguration
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>MemorySwapConfiguration contains kubelet swap configuration
For more information, please see KEP: 2400-node-swap</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SwapBehavior">
SwapBehavior
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SwapBehavior configures swap memory available to container workloads. May be one of {&ldquo;LimitedSwap&rdquo;, &ldquo;UnlimitedSwap&rdquo;}
defaults to: LimitedSwap</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Monitoring">Monitoring
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Monitoring contains information about the monitoring configuration for the shoot.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.Alerting">
Alerting
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alerting contains information about the alerting configuration for the shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NamedResourceReference">NamedResourceReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ExtensionResourceState">ExtensionResourceState</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>NamedResourceReference is a named reference to a resource.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#crossversionobjectreference-v1-autoscaling">
Kubernetes autoscaling/v1.CrossVersionObjectReference
</a>
</em>
</td>
<td>
<p>ResourceRef is a reference to a resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfile">NamespacedCloudProfile</a>)
</p>
<p>
<p>NamespacedCloudProfileSpec is the specification of a NamespacedCloudProfile.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.KubernetesSettings">
KubernetesSettings
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubernetes contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.MachineImage">
[]MachineImage
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineType">
[]MachineType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineTypes contains constraints regarding allowed values for machine types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>volumeTypes</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>parent</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileReference">
CloudProfileReference
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Limits">
Limits
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Limits configures operational limits for Shoot clusters using this NamespacedCloudProfile.
Any limits specified here override those set in the parent CloudProfile.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md">https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md</a>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NamespacedCloudProfileStatus">NamespacedCloudProfileStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfile">NamespacedCloudProfile</a>)
</p>
<p>
<p>NamespacedCloudProfileStatus holds the most recently observed status of the NamespacedCloudProfile.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">
CloudProfileSpec
</a>
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
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this NamespacedCloudProfile.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Networking">Networking
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Networking defines networking parameters for the shoot cluster.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<p>Nodes is the CIDR of the entire node network.
This field is mutable.</p>
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
<a href="#core.gardener.cloud/v1beta1.IPFamily">
[]IPFamily
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamilies specifies the IP protocol versions to use for shoot networking. This field is immutable.
See <a href="https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md">https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md</a>.
Defaults to [&ldquo;IPv4&rdquo;].</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NetworkingStatus">NetworkingStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>NetworkingStatus contains information about cluster networking such as CIDRs.</p>
</p>
<table>
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
[]string
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
[]string
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
[]string
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EgressCIDRs is a list of CIDRs used by the shoot as the source IP for egress traffic as reported by the used
Infrastructure extension controller. For certain environments the egress IPs may not be stable in which case the
extension controller may opt to not populate this field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NginxIngress">NginxIngress
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>NginxIngress describes configuration values for the nginx-ingress addon.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Addon</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Addon">
Addon
</a>
</em>
</td>
<td>
<p>
(Members of <code>Addon</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerSourceRanges</code></br>
<em>
[]string
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
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config contains custom configuration for the nginx-ingress-controller configuration.
See <a href="https://github.com/kubernetes/ingress-nginx/blob/master/docs/user-guide/nginx-configuration/configmap.md#configuration-options">https://github.com/kubernetes/ingress-nginx/blob/master/docs/user-guide/nginx-configuration/configmap.md#configuration-options</a></p>
</td>
</tr>
<tr>
<td>
<code>externalTrafficPolicy</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#serviceexternaltrafficpolicy-v1-core">
Kubernetes core/v1.ServiceExternalTrafficPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExternalTrafficPolicy controls the <code>.spec.externalTrafficPolicy</code> value of the load balancer <code>Service</code>
exposing the nginx-ingress. Defaults to <code>Cluster</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.NodeLocalDNS">NodeLocalDNS
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SystemComponents">SystemComponents</a>)
</p>
<p>
<p>NodeLocalDNS contains the settings of the node local DNS components running in the data plane of the Shoot cluster.</p>
</p>
<table>
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
bool
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceTCPToClusterDNS indicates whether the connection from the node local DNS to the cluster DNS (Core DNS) will be forced to TCP or not.
Default, if unspecified, is to enforce TCP.</p>
</td>
</tr>
<tr>
<td>
<code>forceTCPToUpstreamDNS</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceTCPToUpstreamDNS indicates whether the connection from the node local DNS to the upstream DNS (infrastructure DNS) will be forced to TCP or not.
Default, if unspecified, is to enforce TCP.</p>
</td>
</tr>
<tr>
<td>
<code>disableForwardToUpstreamDNS</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>DisableForwardToUpstreamDNS indicates whether requests from node local DNS to upstream DNS should be disabled.
Default, if unspecified, is to forward requests for external domains to upstream DNS</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.OCIRepository">OCIRepository
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.HelmControllerDeployment">HelmControllerDeployment</a>)
</p>
<p>
<p>OCIRepository configures where to pull an OCI Artifact, that could contain for example a Helm Chart.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PullSecretRef is a reference to a secret containing the pull secret.
The secret must be of type <code>kubernetes.io/dockerconfigjson</code> and must be located in the <code>garden</code> namespace.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.OIDCConfig">OIDCConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>OIDCConfig contains configuration settings for the OIDC provider.
Note: Descriptions were taken from the Kubernetes documentation.</p>
</p>
<table>
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
<p>If set, the OpenID server&rsquo;s certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host&rsquo;s root CA set will be used.</p>
</td>
</tr>
<tr>
<td>
<code>clientAuthentication</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.OpenIDConnectClientAuthentication">
OpenIDConnectClientAuthentication
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientAuthentication can optionally contain client configuration used for kubeconfig generation.</p>
<p>Deprecated: This field has no implemented use and will be forbidden starting from Kubernetes 1.31.
It&rsquo;s use was planned for genereting OIDC kubeconfig <a href="https://github.com/gardener/gardener/issues/1433">https://github.com/gardener/gardener/issues/1433</a>
TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.30 is dropped.</p>
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
map[string]string
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of allowed JOSE asymmetric signing algorithms. JWTs with a &lsquo;alg&rsquo; header value not in this list will be rejected. Values are defined by RFC 7518 <a href="https://tools.ietf.org/html/rfc7518#section-3.1">https://tools.ietf.org/html/rfc7518#section-3.1</a></p>
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
<p>The OpenID claim to use as the user name. Note that claims other than the default (&lsquo;sub&rsquo;) is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default &ldquo;sub&rdquo;)</p>
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
<p>If provided, all usernames will be prefixed with this value. If not provided, username claims other than &lsquo;email&rsquo; are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value &lsquo;-&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ObservabilityRotation">ObservabilityRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation</a>)
</p>
<p>
<p>ObservabilityRotation contains information about the observability credential rotation.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the observability credential rotation was successfully completed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.OpenIDConnectClientAuthentication">OpenIDConnectClientAuthentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.OIDCConfig">OIDCConfig</a>)
</p>
<p>
<p>OpenIDConnectClientAuthentication contains configuration for OIDC clients.</p>
</p>
<table>
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
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extra configuration added to kubeconfig&rsquo;s auth-provider.
Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token</p>
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
<h3 id="core.gardener.cloud/v1beta1.PendingWorkerUpdates">PendingWorkerUpdates
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.InPlaceUpdatesStatus">InPlaceUpdatesStatus</a>)
</p>
<p>
<p>PendingWorkerUpdates contains information about worker pools pending in-place update.</p>
</p>
<table>
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
[]string
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManualInPlaceUpdate contains the names of the pending worker pools with strategy ManualInPlaceUpdate..</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.PendingWorkersRollout">PendingWorkersRollout
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CARotation">CARotation</a>, 
<a href="#core.gardener.cloud/v1beta1.ServiceAccountKeyRotation">ServiceAccountKeyRotation</a>)
</p>
<p>
<p>PendingWorkersRollout contains the name of a worker pool and the initiation time of their last rollout due to
credentials rotation.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationTime is the most recent time when the credential rotation was initiated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ProjectMember">ProjectMember
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ProjectSpec">ProjectSpec</a>)
</p>
<p>
<p>ProjectMember is a member of a project.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Subject</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<p>
(Members of <code>Subject</code> are embedded into this type.)
</p>
<p>Subject is representing a user name, an email address, or any other identifier of a user, group, or service
account that has a certain role.</p>
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
<p>Role represents the role of this member.
IMPORTANT: Be aware that this field will be removed in the <code>v1</code> version of this API in favor of the <code>roles</code>
list.
TODO: Remove this field in favor of the <code>roles</code> list in <code>v1</code>.</p>
</td>
</tr>
<tr>
<td>
<code>roles</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles represents the list of roles of this member.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ProjectPhase">ProjectPhase
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ProjectStatus">ProjectStatus</a>)
</p>
<p>
<p>ProjectPhase is a label for the condition of a project at the current time.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ProjectSpec">ProjectSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Project">Project</a>)
</p>
<p>
<p>ProjectSpec is the specification of a Project.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
who created the project. This field is immutable.</p>
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
<p>Description is a human-readable description of what the project is used for.</p>
</td>
</tr>
<tr>
<td>
<code>owner</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Owner is a subject representing a user name, an email address, or any other identifier of a user owning
the project.
IMPORTANT: Be aware that this field will be removed in the <code>v1</code> version of this API in favor of the <code>owner</code>
role. The only way to change the owner will be by moving the <code>owner</code> role. In this API version the only way
to change the owner is to use this field.
TODO: Remove this field in favor of the <code>owner</code> role in <code>v1</code>.</p>
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
<p>Purpose is a human-readable explanation of the project&rsquo;s purpose.</p>
</td>
</tr>
<tr>
<td>
<code>members</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProjectMember">
[]ProjectMember
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Members is a list of subjects representing a user name, an email address, or any other identifier of a user,
group, or service account that has a certain role.</p>
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
<p>Namespace is the name of the namespace that has been created for the Project object.
A nil value means that Gardener will determine the name of the namespace.
If set, its value must be prefixed with <code>garden-</code>.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProjectTolerations">
ProjectTolerations
</a>
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
<a href="#core.gardener.cloud/v1beta1.DualApprovalForDeletion">
[]DualApprovalForDeletion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DualApprovalForDeletion contains configuration for the dual approval concept for resource deletion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ProjectStatus">ProjectStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Project">Project</a>)
</p>
<p>
<p>ProjectStatus holds the most recently observed status of the project.</p>
</p>
<table>
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
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this project.</p>
</td>
</tr>
<tr>
<td>
<code>phase</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ProjectPhase">
ProjectPhase
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StaleAutoDeleteTimestamp contains the timestamp when the project will be garbage-collected/automatically deleted
because it&rsquo;s stale/unused.</p>
</td>
</tr>
<tr>
<td>
<code>lastActivityTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastActivityTimestamp contains the timestamp from the last activity performed in this project.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ProjectTolerations">ProjectTolerations
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ProjectSpec">ProjectSpec</a>)
</p>
<p>
<p>ProjectTolerations contains the tolerations for taints on seed clusters.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.Toleration">
[]Toleration
</a>
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
<a href="#core.gardener.cloud/v1beta1.Toleration">
[]Toleration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Whitelist contains a list of tolerations that are allowed to be added to the shoots in this project. Please note
that this list may only be added by users having the <code>spec-tolerations-whitelist</code> verb for project resources.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Provider">Provider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Provider contains provider-specific information that are handed-over to the provider-specific
extension controller.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControlPlaneConfig contains the provider-specific control plane config blob. Please look up the concrete
definition in the documentation of your provider extension.</p>
</td>
</tr>
<tr>
<td>
<code>infrastructureConfig</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InfrastructureConfig contains the provider-specific infrastructure config blob. Please look up the concrete
definition in the documentation of your provider extension.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Worker">
[]Worker
</a>
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
<a href="#core.gardener.cloud/v1beta1.WorkersSettings">
WorkersSettings
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkersSettings contains settings for all workers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ProxyMode">ProxyMode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeProxyConfig">KubeProxyConfig</a>)
</p>
<p>
<p>ProxyMode available in Linux platform: &lsquo;userspace&rsquo; (older, going to be EOL), &lsquo;iptables&rsquo;
(newer, faster), &lsquo;ipvs&rsquo; (newest, better in performance and scalability).
As of now only &lsquo;iptables&rsquo; and &lsquo;ipvs&rsquo; is supported by Gardener.
In Linux platform, if the iptables proxy is selected, regardless of how, but the system&rsquo;s kernel or iptables versions are
insufficient, this always falls back to the userspace proxy. IPVS mode will be enabled when proxy mode is set to &lsquo;ipvs&rsquo;,
and the fall back path is firstly iptables and then userspace.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.QuotaSpec">QuotaSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Quota">Quota</a>)
</p>
<p>
<p>QuotaSpec is the specification of a Quota.</p>
</p>
<table>
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.</p>
</td>
</tr>
<tr>
<td>
<code>metrics</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>Metrics is a list of resources which will be put under constraints.</p>
</td>
</tr>
<tr>
<td>
<code>scope</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<p>Scope is the scope of the Quota object, either &lsquo;project&rsquo;, &lsquo;secret&rsquo; or &lsquo;workloadidentity&rsquo;. This field is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Region">Region
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>Region contains certain properties of a region.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.AvailabilityZone">
[]AvailabilityZone
</a>
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
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels is an optional set of key-value pairs that contain certain administrator-controlled labels for this region.
It can be used by Gardener administrators/operators to provide additional information about a region, e.g. wrt
quality, reliability, etc.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AccessRestriction">
[]AccessRestriction
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions that can be used for Shoots using this region.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ResourceData">ResourceData
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStateSpec">ShootStateSpec</a>)
</p>
<p>
<p>ResourceData holds the data of a resource referred to by an extension controller state.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>CrossVersionObjectReference</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#crossversionobjectreference-v1-autoscaling">
Kubernetes autoscaling/v1.CrossVersionObjectReference
</a>
</em>
</td>
<td>
<p>
(Members of <code>CrossVersionObjectReference</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>data</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<p>Data of the resource</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ResourceWatchCacheSize">ResourceWatchCacheSize
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.WatchCacheSizes">WatchCacheSizes</a>)
</p>
<p>
<p>ResourceWatchCacheSize contains configuration of the API server&rsquo;s watch cache size for one specific resource.</p>
</p>
<table>
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
<p>APIGroup is the API group of the resource for which the watch cache size should be configured.
An unset value is used to specify the legacy core API (e.g. for <code>secrets</code>).</p>
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
<p>Resource is the name of the resource for which the watch cache size should be configured
(in lowercase plural form, e.g. <code>secrets</code>).</p>
</td>
</tr>
<tr>
<td>
<code>size</code></br>
<em>
int32
</em>
</td>
<td>
<p>CacheSize specifies the watch cache size that should be configured for the specified resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SSHAccess">SSHAccess
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.WorkersSettings">WorkersSettings</a>)
</p>
<p>
<p>SSHAccess contains settings regarding ssh access to the worker nodes.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Enabled indicates whether the SSH access to the worker nodes is ensured to be enabled or disabled in systemd.
Defaults to true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SchedulingProfile">SchedulingProfile
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeSchedulerConfig">KubeSchedulerConfig</a>)
</p>
<p>
<p>SchedulingProfile is a string alias used for scheduling profile values.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.SecretBindingProvider">SecretBindingProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SecretBinding">SecretBinding</a>)
</p>
<p>
<p>SecretBindingProvider defines the provider type of the SecretBinding.</p>
</p>
<table>
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
<p>Type is the type of the provider.</p>
<p>For backwards compatibility, the field can contain multiple providers separated by a comma.
However the usage of single SecretBinding (hence Secret) for different cloud providers is strongly discouraged.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedBackup">SeedBackup
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedBackup contains the object store configuration for backups for shoot (currently only etcd).</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing the cloud provider credentials for
the object store where backups should be stored. It should have enough privileges to manipulate
the objects as well as buckets.
Deprecated: This field will be removed after v1.121.0 has been released. Use <code>CredentialsRef</code> instead.
Until removed, this field is synced with the <code>CredentialsRef</code> field when it refers to a secret.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectreference-v1-core">
Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CredentialsRef is reference to a resource holding the credentials used for
authentication with the object store service where the backups are stored.
Supported referenced resources are v1.Secrets and
security.gardener.cloud/v1alpha1.WorkloadIdentity</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedDNS">SeedDNS
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedDNS contains DNS-relevant information about this seed cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SeedDNSProvider">
SeedDNSProvider
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider configures a DNSProvider</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedDNSProvider">SeedDNSProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedDNS">SeedDNS</a>)
</p>
<p>
<p>SeedDNSProvider configures a DNSProvider for Seeds</p>
</p>
<table>
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
<p>Type describes the type of the dns-provider, for example <code>aws-route53</code></p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing cloud provider credentials used for registering external domains.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedNetworks">SeedNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ShootNetworks">
ShootNetworks
</a>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>BlockCIDRs is a list of network addresses that should be blocked for shoot control plane components running
in the seed cluster.</p>
</td>
</tr>
<tr>
<td>
<code>ipFamilies</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.IPFamily">
[]IPFamily
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamilies specifies the IP protocol versions to use for seed networking. This field is immutable.
See <a href="https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md">https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md</a>.
Defaults to [&ldquo;IPv4&rdquo;].</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedProvider">SeedProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedProvider defines the provider-specific information of this Seed cluster.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones is the list of availability zones the seed cluster is deployed to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSelector">SeedSelector
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.ExposureClassScheduling">ExposureClassScheduling</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>SeedSelector contains constraints for selecting seed to be usable for shoots using a profile</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>LabelSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<p>
(Members of <code>LabelSelector</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>LabelSelector is optional and can be used to select seeds by their label settings</p>
</td>
</tr>
<tr>
<td>
<code>providerTypes</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Providers is optional and can be used by restricting seeds by their provider type. &lsquo;*&rsquo; can be used to enable seeds regardless of their provider type.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdog">SeedSettingDependencyWatchdog
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettings">SeedSettings</a>)
</p>
<p>
<p>SeedSettingDependencyWatchdog controls the dependency-watchdog settings for the seed.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdogWeeder">
SeedSettingDependencyWatchdogWeeder
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdogProber">
SeedSettingDependencyWatchdogProber
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Prober controls the prober settings for the dependency-watchdog for the seed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdogProber">SeedSettingDependencyWatchdogProber
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdog">SeedSettingDependencyWatchdog</a>)
</p>
<p>
<p>SeedSettingDependencyWatchdogProber controls the prober settings for the dependency-watchdog for the seed.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Enabled controls whether the probe controller(prober) of the dependency-watchdog should be enabled. This controller
scales down the kube-controller-manager, machine-controller-manager and cluster-autoscaler of shoot clusters in case their respective kube-apiserver is not
reachable via its external ingress in order to avoid melt-down situations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdogWeeder">SeedSettingDependencyWatchdogWeeder
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdog">SeedSettingDependencyWatchdog</a>)
</p>
<p>
<p>SeedSettingDependencyWatchdogWeeder controls the weeder settings for the dependency-watchdog for the seed.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Enabled controls whether the endpoint controller(weeder) of the dependency-watchdog should be enabled. This controller
helps to alleviate the delay where control plane components remain unavailable by finding the respective pods in
CrashLoopBackoff status and restarting them once their dependants become ready and available again.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingExcessCapacityReservation">SeedSettingExcessCapacityReservation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettings">SeedSettings</a>)
</p>
<p>
<p>SeedSettingExcessCapacityReservation controls the excess capacity reservation for shoot control planes in the seed.</p>
</p>
<table>
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
bool
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingExcessCapacityReservationConfig">
[]SeedSettingExcessCapacityReservationConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configs configures excess capacity reservation deployments for shoot control planes in the seed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingExcessCapacityReservationConfig">SeedSettingExcessCapacityReservationConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettingExcessCapacityReservation">SeedSettingExcessCapacityReservation</a>)
</p>
<p>
<p>SeedSettingExcessCapacityReservationConfig configures excess capacity reservation deployments for shoot control planes in the seed.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>Resources specify the resource requests and limits of the excess-capacity-reservation pod.</p>
</td>
</tr>
<tr>
<td>
<code>nodeSelector</code></br>
<em>
map[string]string
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tolerations specify the tolerations for the the excess-capacity-reservation pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServices">SeedSettingLoadBalancerServices
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettings">SeedSettings</a>)
</p>
<p>
<p>SeedSettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
seed.</p>
</p>
<table>
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
map[string]string
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#serviceexternaltrafficpolicy-v1-core">
Kubernetes core/v1.ServiceExternalTrafficPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExternalTrafficPolicy describes how nodes distribute service traffic they
receive on one of the service&rsquo;s &ldquo;externally-facing&rdquo; addresses.
Defaults to &ldquo;Cluster&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServicesZones">
[]SeedSettingLoadBalancerServicesZones
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones controls settings, which are specific to the single-zone load balancers in a multi-zonal setup.
Can be empty for single-zone seeds. Each specified zone has to relate to one of the zones in seed.spec.provider.zones.</p>
</td>
</tr>
<tr>
<td>
<code>proxyProtocol</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.LoadBalancerServicesProxyProtocol">
LoadBalancerServicesProxyProtocol
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
Defaults to nil, which is equivalent to not allowing ProxyProtocol.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServicesZones">SeedSettingLoadBalancerServicesZones
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServices">SeedSettingLoadBalancerServices</a>)
</p>
<p>
<p>SeedSettingLoadBalancerServicesZones controls settings, which are specific to the single-zone load balancers in a
multi-zonal setup.</p>
</p>
<table>
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
map[string]string
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#serviceexternaltrafficpolicy-v1-core">
Kubernetes core/v1.ServiceExternalTrafficPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExternalTrafficPolicy describes how nodes distribute service traffic they
receive on one of the service&rsquo;s &ldquo;externally-facing&rdquo; addresses.
Defaults to &ldquo;Cluster&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>proxyProtocol</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.LoadBalancerServicesProxyProtocol">
LoadBalancerServicesProxyProtocol
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProxyProtocol controls whether ProxyProtocol is (optionally) allowed for the load balancer services.
Defaults to nil, which is equivalent to not allowing ProxyProtocol.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingScheduling">SeedSettingScheduling
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettings">SeedSettings</a>)
</p>
<p>
<p>SeedSettingScheduling controls settings for scheduling decisions for the seed.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Visible controls whether the gardener-scheduler shall consider this seed when scheduling shoots. Invisible seeds
are not considered by the scheduler.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingTopologyAwareRouting">SeedSettingTopologyAwareRouting
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettings">SeedSettings</a>)
</p>
<p>
<p>SeedSettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
See <a href="https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md">https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md</a>.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Enabled controls whether certain Services deployed in the seed cluster should be topology-aware.
These Services are etcd-main-client, etcd-events-client, kube-apiserver, gardener-resource-manager and vpa-webhook.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettingVerticalPodAutoscaler">SeedSettingVerticalPodAutoscaler
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSettings">SeedSettings</a>)
</p>
<p>
<p>SeedSettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
seed.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Enabled controls whether the VPA components shall be deployed into the garden namespace in the seed cluster. It
is enabled by default because Gardener heavily relies on a VPA being deployed. You should only disable this if
your seed cluster already has another, manually/custom managed VPA deployment.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSettings">SeedSettings
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedSettings contains certain settings for this seed cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingExcessCapacityReservation">
SeedSettingExcessCapacityReservation
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingScheduling">
SeedSettingScheduling
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingLoadBalancerServices">
SeedSettingLoadBalancerServices
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingVerticalPodAutoscaler">
SeedSettingVerticalPodAutoscaler
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingDependencyWatchdog">
SeedSettingDependencyWatchdog
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettingTopologyAwareRouting">
SeedSettingTopologyAwareRouting
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TopologyAwareRouting controls certain settings for topology-aware traffic routing in the seed.
See <a href="https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md">https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md</a>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedSpec">SeedSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Seed">Seed</a>, 
<a href="#core.gardener.cloud/v1beta1.SeedTemplate">SeedTemplate</a>)
</p>
<p>
<p>SeedSpec is the specification of a Seed.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SeedBackup">
SeedBackup
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedDNS">
SeedDNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedNetworks">
SeedNetworks
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedProvider">
SeedProvider
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedTaint">
[]SeedTaint
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedVolume">
SeedVolume
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettings">
SeedSettings
</a>
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
<a href="#core.gardener.cloud/v1beta1.Ingress">
Ingress
</a>
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
<a href="#core.gardener.cloud/v1beta1.AccessRestriction">
[]AccessRestriction
</a>
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
<a href="#core.gardener.cloud/v1beta1.Extension">
[]Extension
</a>
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
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in extension configs by their names.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedStatus">SeedStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Seed">Seed</a>)
</p>
<p>
<p>SeedStatus is the status of a Seed.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.Gardener">
Gardener
</a>
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
<a href="#core.gardener.cloud/v1beta1.Condition">
[]Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Seed&rsquo;s current state.</p>
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
<p>ObservedGeneration is the most recent generation observed for this Seed. It corresponds to the
Seed&rsquo;s generation, which is updated on mutation by the API Server.</p>
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
<code>capacity</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capacity represents the total resources of a seed.</p>
</td>
</tr>
<tr>
<td>
<code>allocatable</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allocatable represents the resources of a seed that are available for scheduling.
Defaults to Capacity.</p>
</td>
</tr>
<tr>
<td>
<code>clientCertificateExpirationTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientCertificateExpirationTimestamp is the timestamp at which gardenlet&rsquo;s client certificate expires.</p>
</td>
</tr>
<tr>
<td>
<code>lastOperation</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.LastOperation">
LastOperation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the Seed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedTaint">SeedTaint
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedTaint describes a taint on a seed.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.SeedTemplate">SeedTemplate
</h3>
<p>
<p>SeedTemplate is a template for creating a Seed object.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SeedSpec">
SeedSpec
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedBackup">
SeedBackup
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedDNS">
SeedDNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedNetworks">
SeedNetworks
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedProvider">
SeedProvider
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedTaint">
[]SeedTaint
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedVolume">
SeedVolume
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedSettings">
SeedSettings
</a>
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
<a href="#core.gardener.cloud/v1beta1.Ingress">
Ingress
</a>
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
<a href="#core.gardener.cloud/v1beta1.AccessRestriction">
[]AccessRestriction
</a>
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
<a href="#core.gardener.cloud/v1beta1.Extension">
[]Extension
</a>
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
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds a list of named resource references that can be referred to in extension configs by their names.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedVolume">SeedVolume
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedVolume contains settings for persistentvolumes created in the seed cluster.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
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
<a href="#core.gardener.cloud/v1beta1.SeedVolumeProvider">
[]SeedVolumeProvider
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Providers is a list of storage class provisioner types for the seed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SeedVolumeProvider">SeedVolumeProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedVolume">SeedVolume</a>)
</p>
<p>
<p>SeedVolumeProvider is a storage class provisioner type.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.ServiceAccountConfig">ServiceAccountConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>ServiceAccountConfig is the kube-apiserver configuration for service accounts.</p>
</p>
<table>
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
<p>Issuer is the identifier of the service account token issuer. The issuer will assert this
identifier in &ldquo;iss&rdquo; claim of issued tokens. This value is used to generate new service account tokens.
This value is a string or URI. Defaults to URI of the API server.</p>
</td>
</tr>
<tr>
<td>
<code>extendTokenExpiration</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExtendTokenExpiration turns on projected service account expiration extension during token generation, which
helps safe transition from legacy token to bound service account token feature. If this flag is enabled,
admission injected tokens would be extended up to 1 year to prevent unexpected failure during transition,
ignoring value of service-account-max-token-expiration.</p>
</td>
</tr>
<tr>
<td>
<code>maxTokenExpiration</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxTokenExpiration is the maximum validity duration of a token created by the service account token issuer. If an
otherwise valid TokenRequest with a validity duration larger than this value is requested, a token will be issued
with a validity duration of this value.
This field must be within [30d,90d].</p>
</td>
</tr>
<tr>
<td>
<code>acceptedIssuers</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AcceptedIssuers is an additional set of issuers that are used to determine which service account tokens are accepted.
These values are not used to generate new service account tokens. Only useful when service account tokens are also
issued by another external system or a change of the current issuer that is used for generating tokens is being performed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ServiceAccountKeyRotation">ServiceAccountKeyRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation</a>)
</p>
<p>
<p>ServiceAccountKeyRotation contains information about the service account key credential rotation.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CredentialsRotationPhase">
CredentialsRotationPhase
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the service account key credential rotation was successfully
completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastInitiationTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastInitiationFinishedTime is the recent time when the service account key credential rotation initiation was
completed.</p>
</td>
</tr>
<tr>
<td>
<code>lastCompletionTriggeredTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTriggeredTime is the recent time when the service account key credential rotation completion was
triggered.</p>
</td>
</tr>
<tr>
<td>
<code>pendingWorkersRollouts</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.PendingWorkersRollout">
[]PendingWorkersRollout
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingWorkersRollouts contains the name of a worker pool and the initiation time of their last rollout due to
credentials rotation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootAdvertisedAddress">ShootAdvertisedAddress
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>ShootAdvertisedAddress contains information for the shoot&rsquo;s Kube API server.</p>
</p>
<table>
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
<p>The URL of the API Server. e.g. <a href="https://api.foo.bar">https://api.foo.bar</a> or <a href="https://1.2.3.4">https://1.2.3.4</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootCredentials">ShootCredentials
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>ShootCredentials contains information about the shoot credentials.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">
ShootCredentialsRotation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Rotation contains information about the credential rotations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentials">ShootCredentials</a>)
</p>
<p>
<p>ShootCredentialsRotation contains information about the rotation of credentials.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CARotation">
CARotation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertificateAuthorities contains information about the certificate authority credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfig</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ShootKubeconfigRotation">
ShootKubeconfigRotation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubeconfig contains information about the kubeconfig credential rotation.</p>
<p>Deprecated: This field is deprecated and will be removed in gardener v1.120</p>
</td>
</tr>
<tr>
<td>
<code>sshKeypair</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ShootSSHKeypairRotation">
ShootSSHKeypairRotation
</a>
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
<a href="#core.gardener.cloud/v1beta1.ObservabilityRotation">
ObservabilityRotation
</a>
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
<a href="#core.gardener.cloud/v1beta1.ServiceAccountKeyRotation">
ServiceAccountKeyRotation
</a>
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
<a href="#core.gardener.cloud/v1beta1.ETCDEncryptionKeyRotation">
ETCDEncryptionKeyRotation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCDEncryptionKey contains information about the ETCD encryption key credential rotation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootKubeconfigRotation">ShootKubeconfigRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation</a>)
</p>
<p>
<p>ShootKubeconfigRotation contains information about the kubeconfig credential rotation.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the kubeconfig credential rotation was successfully completed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootMachineImage">ShootMachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Machine">Machine</a>)
</p>
<p>
<p>ShootMachineImage defines the name and the version of the shoot&rsquo;s machine image in any environment. Has to be
defined in the respective CloudProfile.</p>
</p>
<table>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the shoot&rsquo;s individual configuration passed to an extension resource.</p>
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
<p>Version is the version of the shoot&rsquo;s image.
If version is not provided, it will be defaulted to the latest version from the CloudProfile.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootNetworks">ShootNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.SeedNetworks">SeedNetworks</a>)
</p>
<p>
<p>ShootNetworks contains the default networks CIDRs for shoots.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.ShootPurpose">ShootPurpose
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>ShootPurpose is a type alias for string.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.ShootSSHKeypairRotation">ShootSSHKeypairRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootCredentialsRotation">ShootCredentialsRotation</a>)
</p>
<p>
<p>ShootSSHKeypairRotation contains information about the ssh-keypair credential rotation.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastCompletionTime is the most recent time when the ssh-keypair credential rotation was successfully completed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootSpec">ShootSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Shoot">Shoot</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootTemplate">ShootTemplate</a>)
</p>
<p>
<p>ShootSpec is the specification of a Shoot.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.Addons">
Addons
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Addons contains information about enabled/disabled addons and their configuration.</p>
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
<p>CloudProfileName is a name of a CloudProfile object.
Deprecated: This field will be removed in a future version of Gardener. Use <code>CloudProfile</code> instead.
Until removed, this field is synced with the <code>CloudProfile</code> field.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.DNS">
DNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.Extension">
[]Extension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Hibernation">
Hibernation
</a>
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
<a href="#core.gardener.cloud/v1beta1.Kubernetes">
Kubernetes
</a>
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
<a href="#core.gardener.cloud/v1beta1.Networking">
Networking
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Networking contains information about cluster networking such as CNI Plugin type, CIDRs, &hellip;etc.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Maintenance">
Maintenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maintenance contains information about the time window for maintenance operations and which
operations should be performed.</p>
</td>
</tr>
<tr>
<td>
<code>monitoring</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Monitoring">
Monitoring
</a>
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
<a href="#core.gardener.cloud/v1beta1.Provider">
Provider
</a>
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
<a href="#core.gardener.cloud/v1beta1.ShootPurpose">
ShootPurpose
</a>
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
<p>SecretBindingName is the name of a SecretBinding that has a reference to the provider secret.
The credentials inside the provider secret will be used to create the shoot in the respective account.
The field is mutually exclusive with CredentialsBindingName.
This field is immutable.</p>
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
<a href="#core.gardener.cloud/v1beta1.SeedSelector">
SeedSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector is an optional selector which must match a seed&rsquo;s labels for the shoot to be scheduled on that seed.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
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
<a href="#core.gardener.cloud/v1beta1.Toleration">
[]Toleration
</a>
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
<p>ExposureClassName is the optional name of an exposure class to apply a control plane endpoint exposure strategy.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>systemComponents</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SystemComponents">
SystemComponents
</a>
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
<a href="#core.gardener.cloud/v1beta1.ControlPlane">
ControlPlane
</a>
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
<p>SchedulerName is the name of the responsible scheduler which schedules the shoot.
If not specified, the default scheduler takes over.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>cloudProfile</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileReference">
CloudProfileReference
</a>
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
<p>CredentialsBindingName is the name of a CredentialsBinding that has a reference to the provider credentials.
The credentials will be used to create the shoot in the respective account. The field is mutually exclusive with SecretBindingName.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AccessRestrictionWithOptions">
[]AccessRestrictionWithOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions for this shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootStateSpec">ShootStateSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootState">ShootState</a>)
</p>
<p>
<p>ShootStateSpec is the specification of the ShootState.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.GardenerResourceData">
[]GardenerResourceData
</a>
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
<a href="#core.gardener.cloud/v1beta1.ExtensionResourceState">
[]ExtensionResourceState
</a>
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
<a href="#core.gardener.cloud/v1beta1.ResourceData">
[]ResourceData
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources holds the data of resources referred to by extension controller states</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootStatus">ShootStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Shoot">Shoot</a>)
</p>
<p>
<p>ShootStatus holds the most recently observed status of the Shoot cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.Condition">
[]Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Shoots&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Condition">
[]Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Constraints represents conditions of a Shoot&rsquo;s current state that constraint some operations on it.</p>
</td>
</tr>
<tr>
<td>
<code>gardener</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Gardener">
Gardener
</a>
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
bool
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
<a href="#core.gardener.cloud/v1beta1.LastOperation">
LastOperation
</a>
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
<a href="#core.gardener.cloud/v1beta1.LastError">
[]LastError
</a>
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
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this Shoot. It corresponds to the
Shoot&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
<tr>
<td>
<code>retryCycleStartTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RetryCycleStartTime is the start time of the last retry cycle (used to determine how often an operation
must be retried until we give up).</p>
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
<p>SeedName is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.</p>
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
<p>TechnicalID is a unique technical ID for this Shoot. It is used for the infrastructure resources, and
basically everything that is related to this particular Shoot. For regular shoot clusters, this is also the name
of the namespace in the seed cluster running the shoot&rsquo;s control plane. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>uid</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/types#UID">
k8s.io/apimachinery/pkg/types.UID
</a>
</em>
</td>
<td>
<p>UID is a unique identifier for the Shoot cluster to avoid portability between Kubernetes clusters.
It is used to compute unique hashes. This field is immutable.</p>
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
<a href="#core.gardener.cloud/v1beta1.ShootAdvertisedAddress">
[]ShootAdvertisedAddress
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of addresses that are relevant to the shoot.
These include the Kube API server address and also the service account issuer.</p>
</td>
</tr>
<tr>
<td>
<code>migrationStartTime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
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
<a href="#core.gardener.cloud/v1beta1.ShootCredentials">
ShootCredentials
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastHibernationTriggerTime indicates the last time when the hibernation controller
managed to change the hibernation settings of the cluster</p>
</td>
</tr>
<tr>
<td>
<code>lastMaintenance</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.LastMaintenance">
LastMaintenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastMaintenance holds information about the last maintenance operations on the Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>encryptedResources</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EncryptedResources is the list of resources in the Shoot which are currently encrypted.
Secrets are encrypted by default and are not part of the list.
See <a href="https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md">https://github.com/gardener/gardener/blob/master/docs/usage/security/etcd_encryption_config.md</a> for more details.</p>
</td>
</tr>
<tr>
<td>
<code>networking</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.NetworkingStatus">
NetworkingStatus
</a>
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
<a href="#core.gardener.cloud/v1beta1.InPlaceUpdatesStatus">
InPlaceUpdatesStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>InPlaceUpdates contains information about in-place updates for the Shoot workers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.ShootTemplate">ShootTemplate
</h3>
<p>
<p>ShootTemplate is a template for creating a Shoot object.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.ShootSpec">
ShootSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the desired behavior of the Shoot.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>addons</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Addons">
Addons
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Addons contains information about enabled/disabled addons and their configuration.</p>
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
<p>CloudProfileName is a name of a CloudProfile object.
Deprecated: This field will be removed in a future version of Gardener. Use <code>CloudProfile</code> instead.
Until removed, this field is synced with the <code>CloudProfile</code> field.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.DNS">
DNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.Extension">
[]Extension
</a>
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
<a href="#core.gardener.cloud/v1beta1.Hibernation">
Hibernation
</a>
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
<a href="#core.gardener.cloud/v1beta1.Kubernetes">
Kubernetes
</a>
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
<a href="#core.gardener.cloud/v1beta1.Networking">
Networking
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Networking contains information about cluster networking such as CNI Plugin type, CIDRs, &hellip;etc.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Maintenance">
Maintenance
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Maintenance contains information about the time window for maintenance operations and which
operations should be performed.</p>
</td>
</tr>
<tr>
<td>
<code>monitoring</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Monitoring">
Monitoring
</a>
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
<a href="#core.gardener.cloud/v1beta1.Provider">
Provider
</a>
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
<a href="#core.gardener.cloud/v1beta1.ShootPurpose">
ShootPurpose
</a>
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
<p>SecretBindingName is the name of a SecretBinding that has a reference to the provider secret.
The credentials inside the provider secret will be used to create the shoot in the respective account.
The field is mutually exclusive with CredentialsBindingName.
This field is immutable.</p>
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
<a href="#core.gardener.cloud/v1beta1.SeedSelector">
SeedSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedSelector is an optional selector which must match a seed&rsquo;s labels for the shoot to be scheduled on that seed.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.NamedResourceReference">
[]NamedResourceReference
</a>
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
<a href="#core.gardener.cloud/v1beta1.Toleration">
[]Toleration
</a>
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
<p>ExposureClassName is the optional name of an exposure class to apply a control plane endpoint exposure strategy.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>systemComponents</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.SystemComponents">
SystemComponents
</a>
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
<a href="#core.gardener.cloud/v1beta1.ControlPlane">
ControlPlane
</a>
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
<p>SchedulerName is the name of the responsible scheduler which schedules the shoot.
If not specified, the default scheduler takes over.
This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>cloudProfile</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileReference">
CloudProfileReference
</a>
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
<p>CredentialsBindingName is the name of a CredentialsBinding that has a reference to the provider credentials.
The credentials will be used to create the shoot in the respective account. The field is mutually exclusive with SecretBindingName.</p>
</td>
</tr>
<tr>
<td>
<code>accessRestrictions</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AccessRestrictionWithOptions">
[]AccessRestrictionWithOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessRestrictions describe a list of access restrictions for this shoot cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.StructuredAuthentication">StructuredAuthentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>StructuredAuthentication contains authentication config for kube-apiserver.</p>
</p>
<table>
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
<p>ConfigMapName is the name of the ConfigMap in the project namespace which contains AuthenticationConfiguration
for the kube-apiserver.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.StructuredAuthorization">StructuredAuthorization
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>StructuredAuthorization contains authorization config for kube-apiserver.</p>
</p>
<table>
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
<p>ConfigMapName is the name of the ConfigMap in the project namespace which contains AuthorizationConfiguration for
the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfigs</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.AuthorizerKubeconfigReference">
[]AuthorizerKubeconfigReference
</a>
</em>
</td>
<td>
<p>Kubeconfigs is a list of references for kubeconfigs for the authorization webhooks.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.SwapBehavior">SwapBehavior
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.MemorySwapConfiguration">MemorySwapConfiguration</a>)
</p>
<p>
<p>SwapBehavior configures swap memory available to container workloads</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.SystemComponents">SystemComponents
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>SystemComponents contains the settings of system components in the control or data plane of the Shoot cluster.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.CoreDNS">
CoreDNS
</a>
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
<a href="#core.gardener.cloud/v1beta1.NodeLocalDNS">
NodeLocalDNS
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeLocalDNS contains the settings of the node local DNS components running in the data plane of the Shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Toleration">Toleration
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ExposureClassScheduling">ExposureClassScheduling</a>, 
<a href="#core.gardener.cloud/v1beta1.ProjectTolerations">ProjectTolerations</a>, 
<a href="#core.gardener.cloud/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Toleration is a toleration for a seed taint.</p>
</p>
<table>
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
<h3 id="core.gardener.cloud/v1beta1.VersionClassification">VersionClassification
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.ExpirableVersion">ExpirableVersion</a>)
</p>
<p>
<p>VersionClassification is the logical state of a version.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.VerticalPodAutoscaler">VerticalPodAutoscaler
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>VerticalPodAutoscaler contains the configuration flags for the Kubernetes vertical pod autoscaler.</p>
</p>
<table>
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
bool
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictAfterOOMThreshold defines the threshold that will lead to pod eviction in case it OOMed in less than the given
threshold since its start and if it has only one container (default: 10m0s).</p>
</td>
</tr>
<tr>
<td>
<code>evictionRateBurst</code></br>
<em>
int32
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
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionRateLimit defines the number of pods that can be evicted per second. A rate limit set to 0 or -1 will
disable the rate limiter (default: -1).</p>
</td>
</tr>
<tr>
<td>
<code>evictionTolerance</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionTolerance defines the fraction of replica count that can be evicted for update in case more than one
pod can be evicted (default: 0.5).</p>
</td>
</tr>
<tr>
<td>
<code>recommendationMarginFraction</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationMarginFraction is the fraction of usage added as the safety margin to the recommended request
(default: 0.15).</p>
</td>
</tr>
<tr>
<td>
<code>updaterInterval</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
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
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetCPUPercentile is the usage percentile that will be used as a base for CPU target recommendation.
Doesn&rsquo;t affect CPU lower bound, CPU upper bound nor memory recommendations.
(default: 0.9)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationLowerBoundCPUPercentile</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationLowerBoundCPUPercentile is the usage percentile that will be used for the lower bound on CPU recommendation.
(default: 0.5)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationUpperBoundCPUPercentile</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationUpperBoundCPUPercentile is the usage percentile that will be used for the upper bound on CPU recommendation.
(default: 0.95)</p>
</td>
</tr>
<tr>
<td>
<code>targetMemoryPercentile</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetMemoryPercentile is the usage percentile that will be used as a base for memory target recommendation.
Doesn&rsquo;t affect memory lower bound nor memory upper bound.
(default: 0.9)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationLowerBoundMemoryPercentile</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationLowerBoundMemoryPercentile is the usage percentile that will be used for the lower bound on memory recommendation.
(default: 0.5)</p>
</td>
</tr>
<tr>
<td>
<code>recommendationUpperBoundMemoryPercentile</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>RecommendationUpperBoundMemoryPercentile is the usage percentile that will be used for the upper bound on memory recommendation.
(default: 0.95)</p>
</td>
</tr>
<tr>
<td>
<code>cpuHistogramDecayHalfLife</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CPUHistogramDecayHalfLife is the amount of time it takes a historical CPU usage sample to lose half of its weight.
(default: 24h)</p>
</td>
</tr>
<tr>
<td>
<code>memoryHistogramDecayHalfLife</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryHistogramDecayHalfLife is the amount of time it takes a historical memory usage sample to lose half of its weight.
(default: 24h)</p>
</td>
</tr>
<tr>
<td>
<code>memoryAggregationInterval</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAggregationInterval is the length of a single interval, for which the peak memory usage is computed.
(default: 24h)</p>
</td>
</tr>
<tr>
<td>
<code>memoryAggregationIntervalCount</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>MemoryAggregationIntervalCount is the number of consecutive memory-aggregation-intervals which make up the
MemoryAggregationWindowLength which in turn is the period for memory usage aggregation by VPA. In other words,
<code>MemoryAggregationWindowLength = memory-aggregation-interval * memory-aggregation-interval-count</code>.
(default: 8)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Volume">Volume
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>Volume contains information about the volume type, size, and encryption.</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Encrypted determines if the volume should be encrypted.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.VolumeType">VolumeType
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.CloudProfileSpec">CloudProfileSpec</a>, 
<a href="#core.gardener.cloud/v1beta1.NamespacedCloudProfileSpec">NamespacedCloudProfileSpec</a>)
</p>
<p>
<p>VolumeType contains certain properties of a volume type.</p>
</p>
<table>
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
bool
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
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinSize is the minimal supported storage size.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.WatchCacheSizes">WatchCacheSizes
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>WatchCacheSizes contains configuration of the API server&rsquo;s watch cache sizes.</p>
</p>
<table>
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
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Default configures the default watch cache size of the kube-apiserver
(flag <code>--default-watch-cache-size</code>, defaults to 100).
See: <a href="https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/">https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/</a></p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.ResourceWatchCacheSize">
[]ResourceWatchCacheSize
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources configures the watch cache size of the kube-apiserver per resource
(flag <code>--watch-cache-sizes</code>).
See: <a href="https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/">https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.Worker">Worker
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Provider">Provider</a>)
</p>
<p>
<p>Worker is the base definition of a worker group.</p>
</p>
<table>
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
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations is a map of key/value pairs for annotations for all the <code>Node</code> objects in this worker pool.</p>
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
<a href="#core.gardener.cloud/v1beta1.CRI">
CRI
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CRI contains configurations of CRI support of every machine in the worker pool.
Defaults to a CRI with name <code>containerd</code>.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.WorkerKubernetes">
WorkerKubernetes
</a>
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
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels is a map of key/value pairs for labels for all the <code>Node</code> objects in this worker pool.</p>
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
<a href="#core.gardener.cloud/v1beta1.Machine">
Machine
</a>
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
int32
</em>
</td>
<td>
<p>Maximum is the maximum number of machines to create.
This value is divided by the number of configured zones for a fair distribution.</p>
</td>
</tr>
<tr>
<td>
<code>minimum</code></br>
<em>
int32
</em>
</td>
<td>
<p>Minimum is the minimum number of machines to create.
This value is divided by the number of configured zones for a fair distribution.</p>
</td>
</tr>
<tr>
<td>
<code>maxSurge</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxSurge is maximum number of machines that are created during an update.
This value is divided by the number of configured zones for a fair distribution.
Defaults to 0 in case of an in-place update.
Defaults to 1 in case of a rolling update.</p>
</td>
</tr>
<tr>
<td>
<code>maxUnavailable</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/util/intstr#IntOrString">
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxUnavailable is the maximum number of machines that can be unavailable during an update.
This value is divided by the number of configured zones for a fair distribution.
Defaults to 1 in case of an in-place update.
Defaults to 0 in case of a rolling update.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/runtime#RawExtension">
k8s.io/apimachinery/pkg/runtime.RawExtension
</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Taints is a list of taints for all the <code>Node</code> objects in this worker pool.</p>
</td>
</tr>
<tr>
<td>
<code>volume</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.Volume">
Volume
</a>
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
<a href="#core.gardener.cloud/v1beta1.DataVolume">
[]DataVolume
</a>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones is a list of availability zones that are used to evenly distribute this worker pool. Optional
as not every provider may support availability zones.</p>
</td>
</tr>
<tr>
<td>
<code>systemComponents</code></br>
<em>
<a href="#core.gardener.cloud/v1beta1.WorkerSystemComponents">
WorkerSystemComponents
</a>
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
<a href="#core.gardener.cloud/v1beta1.MachineControllerManagerSettings">
MachineControllerManagerSettings
</a>
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
map[string]string
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
<a href="#core.gardener.cloud/v1beta1.ClusterAutoscalerOptions">
ClusterAutoscalerOptions
</a>
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
int32
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
<a href="#core.gardener.cloud/v1beta1.MachineUpdateStrategy">
MachineUpdateStrategy
</a>
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
<a href="#core.gardener.cloud/v1beta1.WorkerControlPlane">
WorkerControlPlane
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControlPlane specifies that the shoot cluster control plane components should be running in this worker pool.
This is only relevant for autonomous shoot clusters.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.WorkerControlPlane">WorkerControlPlane
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>WorkerControlPlane specifies that the shoot cluster control plane components should be running in this worker pool.</p>
</p>
<h3 id="core.gardener.cloud/v1beta1.WorkerKubernetes">WorkerKubernetes
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>WorkerKubernetes contains configuration for Kubernetes components related to this worker pool.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.KubeletConfig">
KubeletConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubelet contains configuration settings for all kubelets of this worker pool.
If set, all <code>spec.kubernetes.kubelet</code> settings will be overwritten for this worker pool (no merge of settings).</p>
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
<p>Version is the semantic Kubernetes version to use for the Kubelet in this Worker Group.
If not specified the kubelet version is derived from the global shoot cluster kubernetes version.
version must be equal or lower than the version of the shoot kubernetes version.
Only one minor version difference to other worker groups and global kubernetes version is allowed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.WorkerSystemComponents">WorkerSystemComponents
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>WorkerSystemComponents contains configuration for system components related to this worker pool</p>
</p>
<table>
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
bool
</em>
</td>
<td>
<p>Allow determines whether the pool should be allowed to host system components or not (defaults to true)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1beta1.WorkersSettings">WorkersSettings
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1beta1.Provider">Provider</a>)
</p>
<p>
<p>WorkersSettings contains settings for all workers.</p>
</p>
<table>
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
<a href="#core.gardener.cloud/v1beta1.SSHAccess">
SSHAccess
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SSHAccess contains settings regarding ssh access to the worker nodes.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
