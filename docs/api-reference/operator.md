<p>Packages:</p>
<ul>
<li>
<a href="#operator.gardener.cloud%2fv1alpha1">operator.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="operator.gardener.cloud/v1alpha1">operator.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the configuration of the Gardener Operator.</p>
</p>
Resource Types:
<ul></ul>
<h3 id="operator.gardener.cloud/v1alpha1.Backup">Backup
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ETCDMain">ETCDMain</a>)
</p>
<p>
<p>Backup contains the object store configuration for backups for the virtual garden etcd.</p>
</p>
<table>
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
<code>bucketName</code></br>
<em>
string
</em>
</td>
<td>
<p>BucketName is the name of the backup bucket.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing the cloud provider credentials for the object store where
backups should be stored. It should have enough privileges to manipulate the objects as well as buckets.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ControlPlane">ControlPlane
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>ControlPlane holds information about the general settings for the control plane of the virtual garden cluster.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.HighAvailability">
HighAvailability
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HighAvailability holds the configuration settings for high availability settings.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Credentials">Credentials
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenStatus">GardenStatus</a>)
</p>
<p>
<p>Credentials contains information about the virtual garden cluster credentials.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.CredentialsRotation">
CredentialsRotation
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
<h3 id="operator.gardener.cloud/v1alpha1.CredentialsRotation">CredentialsRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Credentials">Credentials</a>)
</p>
<p>
<p>CredentialsRotation contains information about the rotation of credentials.</p>
</p>
<table>
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.CARotation
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.ServiceAccountKeyRotation
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.ETCDEncryptionKeyRotation
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCDEncryptionKey contains information about the ETCD encryption key credential rotation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ETCD">ETCD
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>ETCD contains configuration for the etcds of the virtual garden cluster.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.ETCDMain">
ETCDMain
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
<a href="#operator.gardener.cloud/v1alpha1.ETCDEvents">
ETCDEvents
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
<h3 id="operator.gardener.cloud/v1alpha1.ETCDEvents">ETCDEvents
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ETCD">ETCD</a>)
</p>
<p>
<p>ETCDEvents contains configuration for the events etcd.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Storage">
Storage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage contains storage configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ETCDMain">ETCDMain
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ETCD">ETCD</a>)
</p>
<p>
<p>ETCDMain contains configuration for the main etcd.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.Backup">
Backup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup contains the object store configuration for backups for the virtual garden etcd.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Storage">
Storage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage contains storage configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Garden">Garden
</h3>
<p>
<p>Garden describes a list of gardens.</p>
</p>
<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
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
<a href="#operator.gardener.cloud/v1alpha1.GardenSpec">
GardenSpec
</a>
</em>
</td>
<td>
<p>Spec contains the specification of this garden.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>runtimeCluster</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">
RuntimeCluster
</a>
</em>
</td>
<td>
<p>RuntimeCluster contains configuration for the runtime cluster.</p>
</td>
</tr>
<tr>
<td>
<code>virtualCluster</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">
VirtualCluster
</a>
</em>
</td>
<td>
<p>VirtualCluster contains configuration for the virtual cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenStatus">
GardenStatus
</a>
</em>
</td>
<td>
<p>Status contains the status of this garden.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenSpec">GardenSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Garden">Garden</a>)
</p>
<p>
<p>GardenSpec contains the specification of a garden environment.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>runtimeCluster</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">
RuntimeCluster
</a>
</em>
</td>
<td>
<p>RuntimeCluster contains configuration for the runtime cluster.</p>
</td>
</tr>
<tr>
<td>
<code>virtualCluster</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">
VirtualCluster
</a>
</em>
</td>
<td>
<p>VirtualCluster contains configuration for the virtual cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenStatus">GardenStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Garden">Garden</a>)
</p>
<p>
<p>GardenStatus is the status of a garden environment.</p>
</p>
<table>
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.Gardener
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardener holds information about the Gardener which last acted on the Garden.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.Condition
</em>
</td>
<td>
<p>Conditions is a list of conditions.</p>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>credentials</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Credentials">
Credentials
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credentials contains information about the virtual garden cluster credentials.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.HighAvailability">HighAvailability
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ControlPlane">ControlPlane</a>)
</p>
<p>
<p>HighAvailability specifies the configuration settings for high availability for a resource.</p>
</p>
<h3 id="operator.gardener.cloud/v1alpha1.Maintenance">Maintenance
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>Maintenance contains information about the time window for maintenance operations.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>timeWindow</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.MaintenanceTimeWindow
</em>
</td>
<td>
<p>TimeWindow contains information about the time window for maintenance operations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Provider">Provider
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">RuntimeCluster</a>)
</p>
<p>
<p>Provider defines the provider-specific information for this cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>zones</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones is the list of availability zones the cluster is deployed to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.RuntimeCluster">RuntimeCluster
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenSpec">GardenSpec</a>)
</p>
<p>
<p>RuntimeCluster contains configuration for the runtime cluster.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.Provider">
Provider
</a>
</em>
</td>
<td>
<p>Provider defines the provider-specific information for this cluster.</p>
</td>
</tr>
<tr>
<td>
<code>settings</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Settings">
Settings
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Settings contains certain settings for this cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.SettingLoadBalancerServices">SettingLoadBalancerServices
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Settings">Settings</a>)
</p>
<p>
<p>SettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
runtime cluster.</p>
</p>
<table>
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
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.SettingVerticalPodAutoscaler">SettingVerticalPodAutoscaler
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Settings">Settings</a>)
</p>
<p>
<p>SettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
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
<em>(Optional)</em>
<p>Enabled controls whether the VPA components shall be deployed into this cluster. It is true by default because
the operator (and Gardener) heavily rely on a VPA being deployed. You should only disable this if your runtime
cluster already has another, manually/custom managed VPA deployment. If this is not the case, but you still
disable it, then reconciliation will fail.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Settings">Settings
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">RuntimeCluster</a>)
</p>
<p>
<p>Settings contains certain settings for this cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>loadBalancerServices</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.SettingLoadBalancerServices">
SettingLoadBalancerServices
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerServices controls certain settings for services of type load balancer that are created in the runtime
cluster.</p>
</td>
</tr>
<tr>
<td>
<code>verticalPodAutoscaler</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.SettingVerticalPodAutoscaler">
SettingVerticalPodAutoscaler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Storage">Storage
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ETCDEvents">ETCDEvents</a>, 
<a href="#operator.gardener.cloud/v1alpha1.ETCDMain">ETCDMain</a>)
</p>
<p>
<p>Storage contains storage configuration.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>capacity</code></br>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>Capacity is the storage capacity for the volumes.</p>
</td>
</tr>
<tr>
<td>
<code>className</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClassName is the name of a storage class.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenSpec">GardenSpec</a>)
</p>
<p>
<p>VirtualCluster contains configuration for the virtual cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>controlPlane</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ControlPlane">
ControlPlane
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControlPlane holds information about the general settings for the control plane of the virtual cluster.</p>
</td>
</tr>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ETCD">
ETCD
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ETCD contains configuration for the etcds of the virtual garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Maintenance">
Maintenance
</a>
</em>
</td>
<td>
<p>Maintenance contains information about the time window for maintenance operations.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
