<p>Packages:</p>
<ul>
<li>
<a href="#garden.sapcloud.io%2fv1beta1">garden.sapcloud.io/v1beta1</a>
</li>
</ul>
<h2 id="garden.sapcloud.io/v1beta1">garden.sapcloud.io/v1beta1</h2>
<p>
<p>Package v1beta1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#garden.sapcloud.io/v1beta1.CloudProfile">CloudProfile</a>
</li><li>
<a href="#garden.sapcloud.io/v1beta1.Project">Project</a>
</li><li>
<a href="#garden.sapcloud.io/v1beta1.Quota">Quota</a>
</li><li>
<a href="#garden.sapcloud.io/v1beta1.SecretBinding">SecretBinding</a>
</li><li>
<a href="#garden.sapcloud.io/v1beta1.Seed">Seed</a>
</li><li>
<a href="#garden.sapcloud.io/v1beta1.Shoot">Shoot</a>
</li></ul>
<h3 id="garden.sapcloud.io/v1beta1.CloudProfile">CloudProfile
</h3>
<p>
<p>CloudProfile represents certain properties about a cloud environment.</p>
</p>
<table>
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
garden.sapcloud.io/v1beta1
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
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
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">
CloudProfileSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the cloud environment properties.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>aws</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSProfile">
AWSProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AWS is the profile specification for the Amazon Web Services cloud.</p>
</td>
</tr>
<tr>
<td>
<code>azure</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureProfile">
AzureProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Azure is the profile specification for the Microsoft Azure cloud.</p>
</td>
</tr>
<tr>
<td>
<code>gcp</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPProfile">
GCPProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GCP is the profile specification for the Google Cloud Platform cloud.</p>
</td>
</tr>
<tr>
<td>
<code>openstack</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackProfile">
OpenStackProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OpenStack is the profile specification for the OpenStack cloud.</p>
</td>
</tr>
<tr>
<td>
<code>alicloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudProfile">
AlicloudProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alicloud is the profile specification for the Alibaba cloud.</p>
</td>
</tr>
<tr>
<td>
<code>packet</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.PacketProfile">
PacketProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Packet is the profile specification for the Packet cloud.</p>
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
<p>CABundle is a certificate bundle which will be installed onto every host machine of the Shoot cluster.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Project">Project
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
garden.sapcloud.io/v1beta1
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
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
<a href="#garden.sapcloud.io/v1beta1.ProjectSpec">
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
who created the project.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Owner is a subject representing a user name, an email address, or any other identifier of a user owning
the project.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
[]Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Members is a list of subjects representing a user name, an email address, or any other identifier of a user
that should be part of this project with full permissions to manage it.</p>
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
A nil value means that Gardener will determine the name of the namespace.</p>
</td>
</tr>
<tr>
<td>
<code>viewers</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
[]Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Viewers is a list of subjects representing a user name, an email address, or any other identifier of a user
that should be part of this project with limited permissions to only view some resources.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.ProjectStatus">
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
<h3 id="garden.sapcloud.io/v1beta1.Quota">Quota
</h3>
<p>
</p>
<table>
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
garden.sapcloud.io/v1beta1
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
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
<a href="#garden.sapcloud.io/v1beta1.QuotaSpec">
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
int
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#resourcelist-v1-core">
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
<a href="#garden.sapcloud.io/v1beta1.QuotaScope">
QuotaScope
</a>
</em>
</td>
<td>
<p>Scope is the scope of the Quota object, either &lsquo;project&rsquo; or &lsquo;secret&rsquo;.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.SecretBinding">SecretBinding
</h3>
<p>
</p>
<table>
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
garden.sapcloud.io/v1beta1
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a secret object in the same or another namespace.</p>
</td>
</tr>
<tr>
<td>
<code>quotas</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core">
[]Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Quotas is a list of references to Quota objects in the same or another namespace.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Seed">Seed
</h3>
<p>
<p>Seed holds certain properties about a Seed cluster.</p>
</p>
<table>
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
garden.sapcloud.io/v1beta1
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
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
<a href="#garden.sapcloud.io/v1beta1.SeedSpec">
SeedSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec defines the Seed cluster properties.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>cloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.SeedCloud">
SeedCloud
</a>
</em>
</td>
<td>
<p>Cloud defines the cloud profile and the region this Seed cluster belongs to.</p>
</td>
</tr>
<tr>
<td>
<code>ingressDomain</code></br>
<em>
string
</em>
</td>
<td>
<p>IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
to construct ingress URLs for system applications running in Shoot clusters.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#secretreference-v1-core">
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
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.SeedNetworks">
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
<code>visible</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Visible labels the Seed cluster as selectable for the seedfinder admission controller.</p>
</td>
</tr>
<tr>
<td>
<code>protected</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Protected prevent that the Seed Cluster can be used for regular Shoot cluster control planes.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.BackupProfile">
BackupProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup holds the object store configuration for the backups of shoot(currently only etcd).
If it is not specified, then there won&rsquo;t be any backups taken for Shoots associated with this Seed.
If backup field is present in Seed, then backups of the etcd from Shoot controlplane will be stored under the
configured object store.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.SeedStatus">
SeedStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Seed cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Shoot">Shoot
</h3>
<p>
</p>
<table>
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
garden.sapcloud.io/v1beta1
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta">
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
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">
ShootSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specification of the Shoot cluster.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>addons</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Addons">
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
<code>cloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">
Cloud
</a>
</em>
</td>
<td>
<p>Cloud contains information about the cloud environment and their specific settings.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNS">
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
<a href="#garden.sapcloud.io/v1beta1.Extension">
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
<a href="#garden.sapcloud.io/v1beta1.Hibernation">
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
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">
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
<a href="#garden.sapcloud.io/v1beta1.Networking">
Networking
</a>
</em>
</td>
<td>
<p>Networking contains information about cluster networking such as CNI Plugin type, CIDRs, &hellip;etc.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Maintenance">
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
<a href="#garden.sapcloud.io/v1beta1.Monitoring">
Monitoring
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Monitoring contains information about custom monitoring configurations for the shoot.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.ShootStatus">
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
<h3 id="garden.sapcloud.io/v1beta1.AWSCloud">AWSCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">Cloud</a>)
</p>
<p>
<p>AWSCloud contains the Shoot specification for AWS.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSNetworks">
AWSNetworks
</a>
</em>
</td>
<td>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSWorker">
[]AWSWorker
</a>
</em>
</td>
<td>
<p>Workers is a list of worker groups.</p>
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
<p>Zones is a list of availability zones to deploy the Shoot cluster to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSProfile">AWSProfile</a>)
</p>
<p>
<p>AWSConstraints is an object containing constraints for certain values in the Shoot specification.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dnsProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNSProviderConstraint">
[]DNSProviderConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSProviders contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">
KubernetesConstraints
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
<a href="#garden.sapcloud.io/v1beta1.MachineImage">
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
<a href="#garden.sapcloud.io/v1beta1.MachineType">
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
<code>volumeTypes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<p>Zones contains constraints regarding allowed values for &lsquo;zones&rsquo; block in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AWSNetworks">AWSNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSCloud">AWSCloud</a>)
</p>
<p>
<p>AWSNetworks holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>vpc</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSVPC">
AWSVPC
</a>
</em>
</td>
<td>
<p>VPC indicates whether to use an existing VPC or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>internal</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Internal is a list of private subnets to create (used for internal load balancers).</p>
</td>
</tr>
<tr>
<td>
<code>public</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Public is a list of public subnets to create (used for bastion and load balancers).</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Workers is a list of worker subnets (private) to create (used for the VMs).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AWSProfile">AWSProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>AWSProfile defines certain constraints and definitions for the AWS cloud.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">
AWSConstraints
</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AWSVPC">AWSVPC
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSNetworks">AWSNetworks</a>)
</p>
<p>
<p>AWSVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).</p>
</p>
<table>
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
<em>(Optional)</em>
<p>ID is the AWS VPC id of an existing VPC.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CIDR is a CIDR range for a new VPC.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AWSWorker">AWSWorker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSCloud">AWSCloud</a>)
</p>
<p>
<p>AWSWorker is the definition of a worker group.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Worker</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Worker">
Worker
</a>
</em>
</td>
<td>
<p>
(Members of <code>Worker</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeType is the type of the root volumes.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeSize is the size of the root volume.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Addon">Addon
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AddonClusterAutoscaler">AddonClusterAutoscaler</a>, 
<a href="#garden.sapcloud.io/v1beta1.Heapster">Heapster</a>, 
<a href="#garden.sapcloud.io/v1beta1.HelmTiller">HelmTiller</a>, 
<a href="#garden.sapcloud.io/v1beta1.Kube2IAM">Kube2IAM</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubeLego">KubeLego</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubernetesDashboard">KubernetesDashboard</a>, 
<a href="#garden.sapcloud.io/v1beta1.Monocular">Monocular</a>, 
<a href="#garden.sapcloud.io/v1beta1.NginxIngress">NginxIngress</a>)
</p>
<p>
<p>Addon also enabling or disabling a specific addon and is used to derive from.</p>
</p>
<table>
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
<h3 id="garden.sapcloud.io/v1beta1.AddonClusterAutoscaler">AddonClusterAutoscaler
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>ClusterAutoscaler describes configuration values for the cluster-autoscaler addon.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Addons">Addons
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
<code>kubernetes-dashboard</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesDashboard">
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
<code>nginx-ingress</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.NginxIngress">
NginxIngress
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NginxIngress holds configuration settings for the nginx-ingress addon.
DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>cluster-autoscaler</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AddonClusterAutoscaler">
AddonClusterAutoscaler
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClusterAutoscaler holds configuration settings for the cluster autoscaler addon.
DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>heapster</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Heapster">
Heapster
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Heapster holds configuration settings for the heapster addon.
DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>kube2iam</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Kube2IAM">
Kube2IAM
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kube2IAM holds configuration settings for the kube2iam addon (only AWS).
DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>kube-lego</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeLego">
KubeLego
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeLego holds configuration settings for the kube-lego addon.
DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>monocular</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Monocular">
Monocular
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Monocular holds configuration settings for the monocular addon.
DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AdmissionPlugin">AdmissionPlugin
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
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
<a href="../core#core.gardener.cloud/v1alpha1.ProviderConfig">
github.com/gardener/gardener/pkg/apis/core/v1alpha1.ProviderConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config is the configuration of the plugin.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Alerting">Alerting
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Monitoring">Monitoring</a>)
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
<h3 id="garden.sapcloud.io/v1beta1.Alicloud">Alicloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">Cloud</a>)
</p>
<p>
<p>Alicloud contains the Shoot specification for Alibaba cloud</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudNetworks">
AlicloudNetworks
</a>
</em>
</td>
<td>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudWorker">
[]AlicloudWorker
</a>
</em>
</td>
<td>
<p>Workers is a list of worker groups.</p>
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
<p>Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudProfile">AlicloudProfile</a>)
</p>
<p>
<p>AlicloudConstraints is an object containing constraints for certain values in the Shoot specification</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dnsProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNSProviderConstraint">
[]DNSProviderConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSProviders contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">
KubernetesConstraints
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
<a href="#garden.sapcloud.io/v1beta1.MachineImage">
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
<a href="#garden.sapcloud.io/v1beta1.AlicloudMachineType">
[]AlicloudMachineType
</a>
</em>
</td>
<td>
<p>MachineTypes contains constraints regarding allowed values for machine types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>volumeTypes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudVolumeType">
[]AlicloudVolumeType
</a>
</em>
</td>
<td>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<p>Zones contains constraints regarding allowed values for &lsquo;zones&rsquo; block in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudMachineType">AlicloudMachineType
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints</a>)
</p>
<p>
<p>AlicloudMachineType defines certain machine types and zone constraints.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>MachineType</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.MachineType">
MachineType
</a>
</em>
</td>
<td>
<p>
(Members of <code>MachineType</code> are embedded into this type.)
</p>
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
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudNetworks">AlicloudNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Alicloud">Alicloud</a>)
</p>
<p>
<p>AlicloudNetworks holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>vpc</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudVPC">
AlicloudVPC
</a>
</em>
</td>
<td>
<p>VPC indicates whether to use an existing VPC or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Workers is a CIDR of a worker subnet (private) to create (used for the VMs).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudProfile">AlicloudProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>AlicloudProfile defines constraints and definitions in Alibaba Cloud environment.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">
AlicloudConstraints
</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudVPC">AlicloudVPC
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudNetworks">AlicloudNetworks</a>)
</p>
<p>
<p>AlicloudVPC contains either an id (of an existing VPC) or the CIDR (for a VPC to be created).</p>
</p>
<table>
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
<em>(Optional)</em>
<p>ID is the Alicloud VPC id of an existing VPC.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CIDR is a CIDR range for a new VPC.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudVolumeType">AlicloudVolumeType
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints</a>)
</p>
<p>
<p>AlicloudVolumeType defines certain volume types and zone constraints.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>VolumeType</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.VolumeType">
VolumeType
</a>
</em>
</td>
<td>
<p>
(Members of <code>VolumeType</code> are embedded into this type.)
</p>
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
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AlicloudWorker">AlicloudWorker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Alicloud">Alicloud</a>)
</p>
<p>
<p>AlicloudWorker is the definition of a worker group.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Worker</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Worker">
Worker
</a>
</em>
</td>
<td>
<p>
(Members of <code>Worker</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeType is the type of the root volumes.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeSize is the size of the root volume.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AuditConfig">AuditConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
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
<a href="#garden.sapcloud.io/v1beta1.AuditPolicy">
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
<h3 id="garden.sapcloud.io/v1beta1.AuditPolicy">AuditPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AuditConfig">AuditConfig</a>)
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectreference-v1-core">
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
<h3 id="garden.sapcloud.io/v1beta1.AzureCloud">AzureCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">Cloud</a>)
</p>
<p>
<p>AzureCloud contains the Shoot specification for Azure.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureNetworks">
AzureNetworks
</a>
</em>
</td>
<td>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureResourceGroup">
AzureResourceGroup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup indicates whether to use an existing resource group or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureWorker">
[]AzureWorker
</a>
</em>
</td>
<td>
<p>Workers is a list of worker groups.</p>
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
<p>Zones is a list of availability zones to deploy the Shoot cluster to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AzureProfile">AzureProfile</a>)
</p>
<p>
<p>AzureConstraints is an object containing constraints for certain values in the Shoot specification.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dnsProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNSProviderConstraint">
[]DNSProviderConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSProviders contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">
KubernetesConstraints
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
<a href="#garden.sapcloud.io/v1beta1.MachineImage">
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
<a href="#garden.sapcloud.io/v1beta1.MachineType">
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
<code>volumeTypes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones contains constraints regarding allowed values for &lsquo;zones&rsquo; block in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureDomainCount">AzureDomainCount
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AzureProfile">AzureProfile</a>)
</p>
<p>
<p>AzureDomainCount defines the region and the count for this domain count value.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is a region in Azure.</p>
</td>
</tr>
<tr>
<td>
<code>count</code></br>
<em>
int
</em>
</td>
<td>
<p>Count is the count value for the respective domain count.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureNetworks">AzureNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AzureCloud">AzureCloud</a>)
</p>
<p>
<p>AzureNetworks holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>vnet</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureVNet">
AzureVNet
</a>
</em>
</td>
<td>
<p>VNet indicates whether to use an existing VNet or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
string
</em>
</td>
<td>
<p>Workers is a CIDR of a worker subnet (private) to create (used for the VMs).</p>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureProfile">AzureProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>AzureProfile defines certain constraints and definitions for the Azure cloud.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">
AzureConstraints
</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>countUpdateDomains</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureDomainCount">
[]AzureDomainCount
</a>
</em>
</td>
<td>
<p>CountUpdateDomains is list of Azure update domain counts for each region.</p>
</td>
</tr>
<tr>
<td>
<code>countFaultDomains</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureDomainCount">
[]AzureDomainCount
</a>
</em>
</td>
<td>
<p>CountFaultDomains is list of Azure fault domain counts for each region.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureResourceGroup">AzureResourceGroup
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AzureCloud">AzureCloud</a>)
</p>
<p>
<p>AzureResourceGroup indicates whether to use an existing resource group or create a new one.</p>
</p>
<table>
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
<p>Name is the name of an existing resource group.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureVNet">AzureVNet
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AzureNetworks">AzureNetworks</a>)
</p>
<p>
<p>AzureVNet indicates whether to use an existing VNet or create a new one.</p>
</p>
<table>
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
<p>Name is the AWS VNet name of an existing VNet.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is the resourceGroup where the VNet is located.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CIDR is a CIDR range for a new VNet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.AzureWorker">AzureWorker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AzureCloud">AzureCloud</a>)
</p>
<p>
<p>AzureWorker is the definition of a worker group.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Worker</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Worker">
Worker
</a>
</em>
</td>
<td>
<p>
(Members of <code>Worker</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeType is the type of the root volumes.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeSize is the size of the root volume.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.BackupProfile">BackupProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>BackupProfile contains the object store configuration for backups for shoot(currently only etcd).</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.CloudProvider">
CloudProvider
</a>
</em>
</td>
<td>
<p>Provider is a provider name.</p>
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
<p>Region is a region name.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing the cloud provider credentials for
the object store where backups should be stored. It should have enough privileges to manipulate
the objects as well as buckets.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Cloud">Cloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Cloud contains information about the cloud environment and their specific settings.
It must contain exactly one key of the below cloud providers.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>profile</code></br>
<em>
string
</em>
</td>
<td>
<p>Profile is a name of a CloudProfile object.</p>
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
<p>Region is a name of a cloud provider region.</p>
</td>
</tr>
<tr>
<td>
<code>secretBindingRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretBindingRef is a reference to a SecretBinding object.</p>
</td>
</tr>
<tr>
<td>
<code>seed</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Seed is the name of a Seed object.</p>
</td>
</tr>
<tr>
<td>
<code>aws</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSCloud">
AWSCloud
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AWS contains the Shoot specification for the Amazon Web Services cloud.</p>
</td>
</tr>
<tr>
<td>
<code>azure</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureCloud">
AzureCloud
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Azure contains the Shoot specification for the Microsoft Azure cloud.</p>
</td>
</tr>
<tr>
<td>
<code>gcp</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPCloud">
GCPCloud
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GCP contains the Shoot specification for the Google Cloud Platform cloud.</p>
</td>
</tr>
<tr>
<td>
<code>openstack</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackCloud">
OpenStackCloud
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OpenStack contains the Shoot specification for the OpenStack cloud.</p>
</td>
</tr>
<tr>
<td>
<code>alicloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Alicloud">
Alicloud
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alicloud contains the Shoot specification for the Alibaba cloud.</p>
</td>
</tr>
<tr>
<td>
<code>packet</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.PacketCloud">
PacketCloud
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Packet contains the Shoot specification for the Packet cloud.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.CloudControllerManagerConfig">CloudControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.KubernetesConfig">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfile">CloudProfile</a>)
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
<code>aws</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AWSProfile">
AWSProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AWS is the profile specification for the Amazon Web Services cloud.</p>
</td>
</tr>
<tr>
<td>
<code>azure</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AzureProfile">
AzureProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Azure is the profile specification for the Microsoft Azure cloud.</p>
</td>
</tr>
<tr>
<td>
<code>gcp</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPProfile">
GCPProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GCP is the profile specification for the Google Cloud Platform cloud.</p>
</td>
</tr>
<tr>
<td>
<code>openstack</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackProfile">
OpenStackProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OpenStack is the profile specification for the OpenStack cloud.</p>
</td>
</tr>
<tr>
<td>
<code>alicloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AlicloudProfile">
AlicloudProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alicloud is the profile specification for the Alibaba cloud.</p>
</td>
</tr>
<tr>
<td>
<code>packet</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.PacketProfile">
PacketProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Packet is the profile specification for the Packet cloud.</p>
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
<p>CABundle is a certificate bundle which will be installed onto every host machine of the Shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.CloudProvider">CloudProvider
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.BackupProfile">BackupProfile</a>)
</p>
<p>
<p>CloudProvider is a string alias.</p>
</p>
<h3 id="garden.sapcloud.io/v1beta1.ClusterAutoscaler">ClusterAutoscaler
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>)
</p>
<p>
<p>ClusterAutoscaler contains the configration flags for the Kubernetes cluster autoscaler.</p>
</p>
<table>
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
<p>ScaleDownUtilizationThreshold defines the threshold in % under which a node is being removed</p>
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
<p>ScaleDownUnneededTime defines how long a node should be unneeded before it is eligible for scale down (default: 10 mins).</p>
</td>
</tr>
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
<p>ScaleDownDelayAfterAdd defines how long after scale up that scale down evaluation resumes (default: 10 mins).</p>
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
<code>scaleDownDelayAfterDelete</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDownDelayAfterDelete how long after node deletion that scale down evaluation resumes, defaults to scanInterval (defaults to ScanInterval).</p>
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.DNS">DNS
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
<p>Domain is the external available domain of the Shoot cluster.</p>
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
by the Shoot and try to find respective credentials there. Specifying this field may override
this behavior, i.e. forcing the Gardener to only look into the given secret.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Provider is the DNS provider type for the Shoot.  Only relevant if not the default
domain is used for this shoot.</p>
</td>
</tr>
<tr>
<td>
<code>includeDomains</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeDomains is a list of domains that shall be included. Only relevant if not the default
domain is used for this shoot.</p>
</td>
</tr>
<tr>
<td>
<code>excludeDomains</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcludeDomains is a list of domains that shall be excluded. Only relevant if not the default
domain is used for this shoot.</p>
</td>
</tr>
<tr>
<td>
<code>includeZones</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>IncludeZones is a list of hosted zone IDs that shall be included. Only relevant if not the default
domain is used for this shoot.</p>
</td>
</tr>
<tr>
<td>
<code>excludeZones</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExcludeZones is a list of hosted zone IDs that shall be excluded. Only relevant if not the default
domain is used for this shoot.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.DNSProviderConstraint">DNSProviderConstraint
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints</a>)
</p>
<p>
<p>DNSProviderConstraint contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</p>
<table>
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
<p>Name is the name of the DNS provider.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Extension">Extension
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
</p>
<p>
<p>Extension contains type and provider information for Shoot extensions.</p>
</p>
<table>
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
<a href="../core#core.gardener.cloud/v1alpha1.ProviderConfig">
github.com/gardener/gardener/pkg/apis/core/v1alpha1.ProviderConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the configuration passed to extension resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GCPCloud">GCPCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">Cloud</a>)
</p>
<p>
<p>GCPCloud contains the Shoot specification for GCP.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPNetworks">
GCPNetworks
</a>
</em>
</td>
<td>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPWorker">
[]GCPWorker
</a>
</em>
</td>
<td>
<p>Workers is a list of worker groups.</p>
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
<p>Zones is a list of availability zones to deploy the Shoot cluster to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.GCPProfile">GCPProfile</a>)
</p>
<p>
<p>GCPConstraints is an object containing constraints for certain values in the Shoot specification.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dnsProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNSProviderConstraint">
[]DNSProviderConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSProviders contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">
KubernetesConstraints
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
<a href="#garden.sapcloud.io/v1beta1.MachineImage">
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
<a href="#garden.sapcloud.io/v1beta1.MachineType">
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
<code>volumeTypes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<p>Zones contains constraints regarding allowed values for &lsquo;zones&rsquo; block in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GCPNetworks">GCPNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.GCPCloud">GCPCloud</a>)
</p>
<p>
<p>GCPNetworks holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>vpc</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPVPC">
GCPVPC
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VPC indicates whether to use an existing VPC or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).</p>
</td>
</tr>
<tr>
<td>
<code>internal</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Internal is a private subnet (used for internal load balancers).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GCPProfile">GCPProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>GCPProfile defines certain constraints and definitions for the GCP cloud.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">
GCPConstraints
</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GCPVPC">GCPVPC
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.GCPNetworks">GCPNetworks</a>)
</p>
<p>
<p>GCPVPC indicates whether to use an existing VPC or create a new one.</p>
</p>
<table>
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
<p>Name is the name of an existing GCP VPC.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GCPWorker">GCPWorker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.GCPCloud">GCPCloud</a>)
</p>
<p>
<p>GCPWorker is the definition of a worker group.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Worker</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Worker">
Worker
</a>
</em>
</td>
<td>
<p>
(Members of <code>Worker</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeType is the type of the root volumes.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeSize is the size of the root volume.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Gardener">Gardener
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.SeedStatus">SeedStatus</a>, 
<a href="#garden.sapcloud.io/v1beta1.ShootStatus">ShootStatus</a>)
</p>
<p>
<p>Gardener holds the information about the Gardener</p>
</p>
<table>
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
<p>ID is the Docker container id of the Gardener which last acted on a Shoot cluster.</p>
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
<p>Name is the hostname (pod name) of the Gardener which last acted on a Shoot cluster.</p>
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
<p>Version is the version of the Gardener which last acted on a Shoot cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.GardenerDuration">GardenerDuration
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.HorizontalPodAutoscalerConfig">HorizontalPodAutoscalerConfig</a>)
</p>
<p>
<p>GardenerDuration is a workaround for missing OpenAPI functions on metav1.Duration struct.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Duration</code></br>
<em>
<a href="https://godoc.org/time#Duration">
time.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Heapster">Heapster
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>Heapster describes configuration values for the heapster addon.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.HelmTiller">HelmTiller
</h3>
<p>
<p>HelmTiller describes configuration values for the helm-tiller addon.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Hibernation">Hibernation
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
If it is false or nil, the Shoot&rsquo;s desired state is to be awaken.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.HibernationSchedule">
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
<h3 id="garden.sapcloud.io/v1beta1.HibernationSchedule">HibernationSchedule
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Hibernation">Hibernation</a>)
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
<p>Location is the time location in which both start and and shall be evaluated.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.HorizontalPodAutoscalerConfig">HorizontalPodAutoscalerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeControllerManagerConfig">KubeControllerManagerConfig</a>)
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
<code>downscaleDelay</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GardenerDuration">
GardenerDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The period since last downscale, before another downscale can be performed in horizontal pod autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>syncPeriod</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GardenerDuration">
GardenerDuration
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
<tr>
<td>
<code>upscaleDelay</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GardenerDuration">
GardenerDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The period since last upscale, before another upscale can be performed in horizontal pod autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>downscaleStabilization</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GardenerDuration">
GardenerDuration
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
<a href="#garden.sapcloud.io/v1beta1.GardenerDuration">
GardenerDuration
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
<code>cpuInitializationPeriod</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.GardenerDuration">
GardenerDuration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The period after which a ready pod transition is considered to be the first.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.K8SNetworks">K8SNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSNetworks">AWSNetworks</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudNetworks">AlicloudNetworks</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureNetworks">AzureNetworks</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPNetworks">GCPNetworks</a>, 
<a href="#garden.sapcloud.io/v1beta1.Networking">Networking</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackNetworks">OpenStackNetworks</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketNetworks">PacketNetworks</a>)
</p>
<p>
<p>K8SNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.</p>
</p>
<table>
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
<p>Nodes is the CIDR of the node network.</p>
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
<h3 id="garden.sapcloud.io/v1beta1.Kube2IAM">Kube2IAM
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>Kube2IAM describes configuration values for the kube2iam addon.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
<code>roles</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Kube2IAMRole">
[]Kube2IAMRole
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Roles is list of AWS IAM roles which should be created by the Gardener.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Kube2IAMRole">Kube2IAMRole
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kube2IAM">Kube2IAM</a>)
</p>
<p>
<p>Kube2IAMRole allows passing AWS IAM policies which will result in IAM roles.</p>
</p>
<table>
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
<p>Name is the name of the IAM role. Will be extended by the Shoot name.</p>
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
<p>Description is a human readable message indiciating what this IAM role can be used for.</p>
</td>
</tr>
<tr>
<td>
<code>policy</code></br>
<em>
string
</em>
</td>
<td>
<p>Policy is an AWS IAM policy document.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>)
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
<a href="#garden.sapcloud.io/v1beta1.KubernetesConfig">
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
<a href="#garden.sapcloud.io/v1beta1.AdmissionPlugin">
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
If <code>serviceAccountConfig.issuer</code> is configured and this is not, this defaults to a single
element list containing the issuer URL.</p>
</td>
</tr>
<tr>
<td>
<code>auditConfig</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.AuditConfig">
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
<code>enableBasicAuthentication</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableBasicAuthentication defines whether basic authentication should be enabled for this cluster or not.</p>
</td>
</tr>
<tr>
<td>
<code>oidcConfig</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OIDCConfig">
OIDCConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OIDCConfig contains configuration settings for the OIDC provider.</p>
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
<a href="#garden.sapcloud.io/v1beta1.ServiceAccountConfig">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeControllerManagerConfig">KubeControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>)
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
<a href="#garden.sapcloud.io/v1beta1.KubernetesConfig">
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
<a href="#garden.sapcloud.io/v1beta1.HorizontalPodAutoscalerConfig">
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
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeCIDRMaskSize defines the mask size for node cidr in cluster (default is 24)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeLego">KubeLego
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>KubeLego describes configuration values for the kube-lego addon.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
<code>email</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Mail is the email address to register at Let&rsquo;s Encrypt.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeProxyConfig">KubeProxyConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>)
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
<a href="#garden.sapcloud.io/v1beta1.KubernetesConfig">
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
<a href="#garden.sapcloud.io/v1beta1.ProxyMode">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeSchedulerConfig">KubeSchedulerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>)
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
<a href="#garden.sapcloud.io/v1beta1.KubernetesConfig">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeletConfig">KubeletConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes</a>, 
<a href="#garden.sapcloud.io/v1beta1.Worker">Worker</a>)
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
<a href="#garden.sapcloud.io/v1beta1.KubernetesConfig">
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
<code>evictionHard</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfigEviction">
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
<code>evictionSoft</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfigEviction">
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
<a href="#garden.sapcloud.io/v1beta1.KubeletConfigEvictionSoftGracePeriod">
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
<code>evictionMinimumReclaim</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfigEvictionMinimumReclaim">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubeletConfigEviction">KubeletConfigEviction
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfig">KubeletConfig</a>)
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
<h3 id="garden.sapcloud.io/v1beta1.KubeletConfigEvictionMinimumReclaim">KubeletConfigEvictionMinimumReclaim
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfig">KubeletConfig</a>)
</p>
<p>
<p>KubeletConfigEviction contains configuration for the kubelet eviction minimum reclaim.</p>
</p>
<table>
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
<h3 id="garden.sapcloud.io/v1beta1.KubeletConfigEvictionSoftGracePeriod">KubeletConfigEvictionSoftGracePeriod
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfig">KubeletConfig</a>)
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
<h3 id="garden.sapcloud.io/v1beta1.Kubernetes">Kubernetes
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
<code>allowPrivilegedContainers</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>AllowPrivilegedContainers indicates whether privileged containers are allowed in the Shoot (default: true).</p>
</td>
</tr>
<tr>
<td>
<code>kubeAPIServer</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeAPIServerConfig">
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
<code>cloudControllerManager</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.CloudControllerManagerConfig">
CloudControllerManagerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudControllerManager contains configuration settings for the cloud-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>kubeControllerManager</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeControllerManagerConfig">
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
<a href="#garden.sapcloud.io/v1beta1.KubeSchedulerConfig">
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
<a href="#garden.sapcloud.io/v1beta1.KubeProxyConfig">
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
<a href="#garden.sapcloud.io/v1beta1.KubeletConfig">
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
<p>Version is the semantic Kubernetes version to use for the Shoot cluster.</p>
</td>
</tr>
<tr>
<td>
<code>clusterAutoscaler</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.ClusterAutoscaler">
ClusterAutoscaler
</a>
</em>
</td>
<td>
<p>ClusterAutoscaler contains the configration flags for the Kubernetes cluster autoscaler.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubernetesConfig">KubernetesConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudControllerManagerConfig">CloudControllerManagerConfig</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubeControllerManagerConfig">KubeControllerManagerConfig</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubeProxyConfig">KubeProxyConfig</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubeSchedulerConfig">KubeSchedulerConfig</a>, 
<a href="#garden.sapcloud.io/v1beta1.KubeletConfig">KubeletConfig</a>)
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
<h3 id="garden.sapcloud.io/v1beta1.KubernetesConstraints">KubernetesConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints</a>)
</p>
<p>
<p>KubernetesConstraints contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</p>
<table>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions is the list of allowed Kubernetes versions for Shoot clusters (e.g., 1.13.1).</p>
</td>
</tr>
<tr>
<td>
<code>offeredVersions</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesVersion">
[]KubernetesVersion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OfferedVersions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.KubernetesDashboard">KubernetesDashboard
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
<h3 id="garden.sapcloud.io/v1beta1.KubernetesVersion">KubernetesVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">KubernetesConstraints</a>)
</p>
<p>
<p>KubernetesVersion contains the version code and optional expiration date for a kubernetes version</p>
</p>
<table>
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
<p>Version is the kubernetes version</p>
</td>
</tr>
<tr>
<td>
<code>expirationDate</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationDate defines the time at which this kubernetes version is not supported any more. This has the following implications:
1) A shoot that opted out of automatic kubernetes system updates and that is running this kubernetes version will be forcefully updated to the latest kubernetes patch version for the current minor version
2) Shoot&rsquo;s with this kubernetes version cannot be created</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.MachineImage">MachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints</a>)
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DEPRECATED: This field will be removed in a future version.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.MachineImageVersion">
[]MachineImageVersion
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions contains versions and expiration dates of the machine image</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.MachineImageVersion">MachineImageVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.MachineImage">MachineImage</a>)
</p>
<p>
<p>MachineImageVersion contains a version and an expiration date of a machine image</p>
</p>
<table>
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
<p>Version is the version of the image.</p>
</td>
</tr>
<tr>
<td>
<code>expirationDate</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationDate defines the time at which a shoot that opted out of automatic operating system updates and
that is running this image version will be forcefully updated to the latest version specified in the referenced
cloud profile.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.MachineType">MachineType
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudMachineType">AlicloudMachineType</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackMachineType">OpenStackMachineType</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints</a>)
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
<code>storage</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.MachineTypeStorage">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.MachineTypeStorage">MachineTypeStorage
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.MachineType">MachineType</a>)
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
<p>Size is the storage size.</p>
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Maintenance">Maintenance
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
<a href="#garden.sapcloud.io/v1beta1.MaintenanceAutoUpdate">
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
<a href="#garden.sapcloud.io/v1beta1.MaintenanceTimeWindow">
MaintenanceTimeWindow
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TimeWindow contains information about the time window for maintenance operations.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.MaintenanceAutoUpdate">MaintenanceAutoUpdate
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Maintenance">Maintenance</a>)
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
<p>KubernetesVersion indicates whether the patch Kubernetes version may be automatically updated.</p>
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
<h3 id="garden.sapcloud.io/v1beta1.MaintenanceTimeWindow">MaintenanceTimeWindow
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Maintenance">Maintenance</a>)
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
<h3 id="garden.sapcloud.io/v1beta1.Monitoring">Monitoring
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
<a href="#garden.sapcloud.io/v1beta1.Alerting">
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
<h3 id="garden.sapcloud.io/v1beta1.Monocular">Monocular
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
</p>
<p>
<p>Monocular describes configuration values for the monocular addon.</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Networking">Networking
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec</a>)
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
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
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
<p>Type identifies the type of the networking plugin</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="../core#core.gardener.cloud/v1alpha1.ProviderConfig">
github.com/gardener/gardener/pkg/apis/core/v1alpha1.ProviderConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the configuration passed to network resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.NginxIngress">NginxIngress
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Addons">Addons</a>)
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
<a href="#garden.sapcloud.io/v1beta1.Addon">
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
<p>LoadBalancerSourceRanges is list of whitelist IP sources for NginxIngress</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#serviceexternaltrafficpolicytype-v1-core">
Kubernetes core/v1.ServiceExternalTrafficPolicyType
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
<h3 id="garden.sapcloud.io/v1beta1.OIDCConfig">OIDCConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
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
<code>clientID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The client ID for the OpenID Connect client, must be set if oidc-issuer-url is set.</p>
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
<p>The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).</p>
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
<p>ATTENTION: Only meaningful for Kubernetes &gt;= 1.11
key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.</p>
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
<tr>
<td>
<code>clientAuthentication</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenIDConnectClientAuthentication">
OpenIDConnectClientAuthentication
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientAuthentication can optionally contain client configuration used for kubeconfig generation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenIDConnectClientAuthentication">OpenIDConnectClientAuthentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OIDCConfig">OIDCConfig</a>)
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackCloud">OpenStackCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">Cloud</a>)
</p>
<p>
<p>OpenStackCloud contains the Shoot specification for OpenStack.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>floatingPoolName</code></br>
<em>
string
</em>
</td>
<td>
<p>FloatingPoolName is the name of the floating pool to get FIPs from.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerProvider</code></br>
<em>
string
</em>
</td>
<td>
<p>LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerClasses</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackLoadBalancerClass">
[]OpenStackLoadBalancerClass
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerClasses available for a dedicated Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>machineImage</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackNetworks">
OpenStackNetworks
</a>
</em>
</td>
<td>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackWorker">
[]OpenStackWorker
</a>
</em>
</td>
<td>
<p>Workers is a list of worker groups.</p>
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
<p>Zones is a list of availability zones to deploy the Shoot cluster to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackProfile">OpenStackProfile</a>)
</p>
<p>
<p>OpenStackConstraints is an object containing constraints for certain values in the Shoot specification.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dnsProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNSProviderConstraint">
[]DNSProviderConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSProviders contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>floatingPools</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackFloatingPool">
[]OpenStackFloatingPool
</a>
</em>
</td>
<td>
<p>FloatingPools contains constraints regarding allowed values of the &lsquo;floatingPoolName&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">
KubernetesConstraints
</a>
</em>
</td>
<td>
<p>Kubernetes contains constraints regarding allowed values of the &lsquo;kubernetes&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackLoadBalancerProvider">
[]OpenStackLoadBalancerProvider
</a>
</em>
</td>
<td>
<p>LoadBalancerProviders contains constraints regarding allowed values of the &lsquo;loadBalancerProvider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.MachineImage">
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
<a href="#garden.sapcloud.io/v1beta1.OpenStackMachineType">
[]OpenStackMachineType
</a>
</em>
</td>
<td>
<p>MachineTypes contains constraints regarding allowed values for machine types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<p>Zones contains constraints regarding allowed values for &lsquo;zones&rsquo; block in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackFloatingPool">OpenStackFloatingPool
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>)
</p>
<p>
<p>OpenStackFloatingPool contains constraints regarding allowed values of the &lsquo;floatingPoolName&rsquo; block in the Shoot specification.</p>
</p>
<table>
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
<p>Name is the name of the floating pool.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerClasses</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackLoadBalancerClass">
[]OpenStackLoadBalancerClass
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerClasses contains a list of supported labeled load balancer network settings.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackLoadBalancerClass">OpenStackLoadBalancerClass
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackCloud">OpenStackCloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackFloatingPool">OpenStackFloatingPool</a>)
</p>
<p>
<p>OpenStackLoadBalancerClass defines a restricted network setting for generic LoadBalancer classes usable in CloudProfiles.</p>
</p>
<table>
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
<p>Name is the name of the LB class</p>
</td>
</tr>
<tr>
<td>
<code>floatingSubnetID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingSubnetID is the subnetwork ID of a dedicated subnet in floating network pool.</p>
</td>
</tr>
<tr>
<td>
<code>floatingNetworkID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>FloatingNetworkID is the network ID of the floating network pool.</p>
</td>
</tr>
<tr>
<td>
<code>subnetID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubnetID is the ID of a local subnet used for LoadBalancer provisioning. Only usable if no FloatingPool
configuration is done.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackLoadBalancerProvider">OpenStackLoadBalancerProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>)
</p>
<p>
<p>OpenStackLoadBalancerProvider contains constraints regarding allowed values of the &lsquo;loadBalancerProvider&rsquo; block in the Shoot specification.</p>
</p>
<table>
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
<p>Name is the name of the load balancer provider.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackMachineType">OpenStackMachineType
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>)
</p>
<p>
<p>OpenStackMachineType contains certain properties of a machine type in OpenStack</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>MachineType</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.MachineType">
MachineType
</a>
</em>
</td>
<td>
<p>
(Members of <code>MachineType</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeType is the type of that volume.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code></br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<p>VolumeSize is the amount of disk storage for this machine type.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackNetworks">OpenStackNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackCloud">OpenStackCloud</a>)
</p>
<p>
<p>OpenStackNetworks holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>router</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackRouter">
OpenStackRouter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Router indicates whether to use an existing router or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Workers is a list of CIDRs of worker subnets (private) to create (used for the VMs).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackProfile">OpenStackProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>OpenStackProfile defines certain constraints and definitions for the OpenStack cloud.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">
OpenStackConstraints
</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>keystoneURL</code></br>
<em>
string
</em>
</td>
<td>
<p>KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).</p>
</td>
</tr>
<tr>
<td>
<code>dnsServers</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSServers is a list of IPs of DNS servers used while creating subnets.</p>
</td>
</tr>
<tr>
<td>
<code>dhcpDomain</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for
Kubernetes 1.10.1+. See <a href="https://github.com/kubernetes/kubernetes/pull/61890">https://github.com/kubernetes/kubernetes/pull/61890</a> for details.</p>
</td>
</tr>
<tr>
<td>
<code>requestTimeout</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>RequestTimeout specifies the HTTP timeout against the OpenStack API.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackRouter">OpenStackRouter
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackNetworks">OpenStackNetworks</a>)
</p>
<p>
<p>OpenStackRouter indicates whether to use an existing router or create a new one.</p>
</p>
<table>
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
<p>ID is the router id of an existing OpenStack router.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.OpenStackWorker">OpenStackWorker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.OpenStackCloud">OpenStackCloud</a>)
</p>
<p>
<p>OpenStackWorker is the definition of a worker group.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Worker</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Worker">
Worker
</a>
</em>
</td>
<td>
<p>
(Members of <code>Worker</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.PacketCloud">PacketCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">Cloud</a>)
</p>
<p>
<p>PacketCloud contains the Shoot specification for Packet cloud</p>
</p>
<table>
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
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.PacketNetworks">
PacketNetworks
</a>
</em>
</td>
<td>
<p>Networks holds information about the Kubernetes and infrastructure networks.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.PacketWorker">
[]PacketWorker
</a>
</em>
</td>
<td>
<p>Workers is a list of worker groups.</p>
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
<p>Zones is a list of availability zones to deploy the Shoot cluster to, currently, only one is supported.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.PacketProfile">PacketProfile</a>)
</p>
<p>
<p>PacketConstraints is an object containing constraints for certain values in the Shoot specification</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>dnsProviders</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNSProviderConstraint">
[]DNSProviderConstraint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNSProviders contains constraints regarding allowed values of the &lsquo;dns.provider&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubernetesConstraints">
KubernetesConstraints
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
<a href="#garden.sapcloud.io/v1beta1.MachineImage">
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
<a href="#garden.sapcloud.io/v1beta1.MachineType">
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
<code>volumeTypes</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.VolumeType">
[]VolumeType
</a>
</em>
</td>
<td>
<p>VolumeTypes contains constraints regarding allowed values for volume types in the &lsquo;workers&rsquo; block in the Shoot specification.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<p>Zones contains constraints regarding allowed values for &lsquo;zones&rsquo; block in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.PacketNetworks">PacketNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.PacketCloud">PacketCloud</a>)
</p>
<p>
<p>PacketNetworks holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>K8SNetworks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.K8SNetworks">
K8SNetworks
</a>
</em>
</td>
<td>
<p>
(Members of <code>K8SNetworks</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.PacketProfile">PacketProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.CloudProfileSpec">CloudProfileSpec</a>)
</p>
<p>
<p>PacketProfile defines constraints and definitions in Packet Cloud environment.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>constraints</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">
PacketConstraints
</a>
</em>
</td>
<td>
<p>Constraints is an object containing constraints for certain values in the Shoot specification.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.PacketWorker">PacketWorker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.PacketCloud">PacketCloud</a>)
</p>
<p>
<p>PacketWorker is the definition of a worker group.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>Worker</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Worker">
Worker
</a>
</em>
</td>
<td>
<p>
(Members of <code>Worker</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>volumeType</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeType is the type of the root volumes.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeSize is the size of the root volume.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ProjectPhase">ProjectPhase
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.ProjectStatus">ProjectStatus</a>)
</p>
<p>
<p>ProjectPhase is a label for the condition of a project at the current time.</p>
</p>
<h3 id="garden.sapcloud.io/v1beta1.ProjectSpec">ProjectSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Project">Project</a>)
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CreatedBy is a subject representing a user name, an email address, or any other identifier of a user
who created the project.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Owner is a subject representing a user name, an email address, or any other identifier of a user owning
the project.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
[]Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Members is a list of subjects representing a user name, an email address, or any other identifier of a user
that should be part of this project with full permissions to manage it.</p>
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
A nil value means that Gardener will determine the name of the namespace.</p>
</td>
</tr>
<tr>
<td>
<code>viewers</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac">
[]Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Viewers is a list of subjects representing a user name, an email address, or any other identifier of a user
that should be part of this project with limited permissions to only view some resources.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ProjectStatus">ProjectStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Project">Project</a>)
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
<a href="#garden.sapcloud.io/v1beta1.ProjectPhase">
ProjectPhase
</a>
</em>
</td>
<td>
<p>Phase is the current phase of the project.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ProxyMode">ProxyMode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeProxyConfig">KubeProxyConfig</a>)
</p>
<p>
<p>ProxyMode available in Linux platform: &lsquo;userspace&rsquo; (older, going to be EOL), &lsquo;iptables&rsquo;
(newer, faster), &lsquo;ipvs&rsquo;(newest, better in performance and scalability).</p>
<p>As of now only &lsquo;iptables&rsquo; and &lsquo;ipvs&rsquo; is supported by Gardener.</p>
<p>In Linux platform, if the iptables proxy is selected, regardless of how, but the system&rsquo;s kernel or iptables versions are
insufficient, this always falls back to the userspace proxy. IPVS mode will be enabled when proxy mode is set to &lsquo;ipvs&rsquo;,
and the fall back path is firstly iptables and then userspace.</p>
</p>
<h3 id="garden.sapcloud.io/v1beta1.QuotaScope">QuotaScope
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.QuotaSpec">QuotaSpec</a>)
</p>
<p>
<p>QuotaScope is a string alias.</p>
</p>
<h3 id="garden.sapcloud.io/v1beta1.QuotaSpec">QuotaSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Quota">Quota</a>)
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
int
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#resourcelist-v1-core">
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
<a href="#garden.sapcloud.io/v1beta1.QuotaScope">
QuotaScope
</a>
</em>
</td>
<td>
<p>Scope is the scope of the Quota object, either &lsquo;project&rsquo; or &lsquo;secret&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.SeedCloud">SeedCloud
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.SeedSpec">SeedSpec</a>)
</p>
<p>
<p>SeedCloud defines the cloud profile and the region this Seed cluster belongs to.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>profile</code></br>
<em>
string
</em>
</td>
<td>
<p>Profile is the name of a cloud profile.</p>
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.SeedNetworks">SeedNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.SeedSpec">SeedSpec</a>)
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
<p>Nodes is the CIDR of the node network.</p>
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
<p>Services is the CIDR of the service network.</p>
</td>
</tr>
<tr>
<td>
<code>shootDefaults</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.ShootNetworks">
ShootNetworks
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootDefaults contains the default networks CIDRs for shoots.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.SeedSpec">SeedSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Seed">Seed</a>)
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
<code>cloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.SeedCloud">
SeedCloud
</a>
</em>
</td>
<td>
<p>Cloud defines the cloud profile and the region this Seed cluster belongs to.</p>
</td>
</tr>
<tr>
<td>
<code>ingressDomain</code></br>
<em>
string
</em>
</td>
<td>
<p>IngressDomain is the domain of the Seed cluster pointing to the ingress controller endpoint. It will be used
to construct ingress URLs for system applications running in Shoot clusters.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#secretreference-v1-core">
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
<code>networks</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.SeedNetworks">
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
<code>visible</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Visible labels the Seed cluster as selectable for the seedfinder admission controller.</p>
</td>
</tr>
<tr>
<td>
<code>protected</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Protected prevent that the Seed Cluster can be used for regular Shoot cluster control planes.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.BackupProfile">
BackupProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Backup holds the object store configuration for the backups of shoot(currently only etcd).
If it is not specified, then there won&rsquo;t be any backups taken for Shoots associated with this Seed.
If backup field is present in Seed, then backups of the etcd from Shoot controlplane will be stored under the
configured object store.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.SeedStatus">SeedStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Seed">Seed</a>)
</p>
<p>
<p>SeedStatus holds the most recently observed status of the Seed cluster.</p>
</p>
<table>
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
<a href="../core#core.gardener.cloud/v1alpha1.Condition">
[]github.com/gardener/gardener/pkg/apis/core/v1alpha1.Condition
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
<code>gardener</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Gardener">
Gardener
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Gardener holds information about the Gardener which last acted on the Seed.</p>
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ServiceAccountConfig">ServiceAccountConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
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
identifier in &ldquo;iss&rdquo; claim of issued tokens. This value is a string or URI.</p>
</td>
</tr>
<tr>
<td>
<code>signingKeySecretName</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SigningKeySecret is a reference to a secret that contains the current private key of the
service account token issuer. The issuer will sign issued ID tokens with this private key.
(Requires the &lsquo;TokenRequest&rsquo; feature gate.)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ShootMachineImage">ShootMachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSCloud">AWSCloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.Alicloud">Alicloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureCloud">AzureCloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPCloud">GCPCloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackCloud">OpenStackCloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketCloud">PacketCloud</a>, 
<a href="#garden.sapcloud.io/v1beta1.Worker">Worker</a>)
</p>
<p>
<p>MachineImage defines the name and the version of the shoot&rsquo;s machine image in any environment. Has to be defined in the respective CloudProfile.</p>
</p>
<table>
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version of the shoot&rsquo;s image.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
<a href="../core#core.gardener.cloud/v1alpha1.ProviderConfig">
github.com/gardener/gardener/pkg/apis/core/v1alpha1.ProviderConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the shoot&rsquo;s individual configuration passed to an extension resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ShootNetworks">ShootNetworks
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.SeedNetworks">SeedNetworks</a>)
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
<h3 id="garden.sapcloud.io/v1beta1.ShootSpec">ShootSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Shoot">Shoot</a>)
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
<a href="#garden.sapcloud.io/v1beta1.Addons">
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
<code>cloud</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Cloud">
Cloud
</a>
</em>
</td>
<td>
<p>Cloud contains information about the cloud environment and their specific settings.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.DNS">
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
<a href="#garden.sapcloud.io/v1beta1.Extension">
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
<a href="#garden.sapcloud.io/v1beta1.Hibernation">
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
<a href="#garden.sapcloud.io/v1beta1.Kubernetes">
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
<a href="#garden.sapcloud.io/v1beta1.Networking">
Networking
</a>
</em>
</td>
<td>
<p>Networking contains information about cluster networking such as CNI Plugin type, CIDRs, &hellip;etc.</p>
</td>
</tr>
<tr>
<td>
<code>maintenance</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.Maintenance">
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
<a href="#garden.sapcloud.io/v1beta1.Monitoring">
Monitoring
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Monitoring contains information about custom monitoring configurations for the shoot.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.ShootStatus">ShootStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.Shoot">Shoot</a>)
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
<a href="../core#core.gardener.cloud/v1alpha1.Condition">
[]github.com/gardener/gardener/pkg/apis/core/v1alpha1.Condition
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
<a href="../core#core.gardener.cloud/v1alpha1.Condition">
[]github.com/gardener/gardener/pkg/apis/core/v1alpha1.Condition
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
<a href="#garden.sapcloud.io/v1beta1.Gardener">
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
<code>lastOperation</code></br>
<em>
<a href="../core#core.gardener.cloud/v1alpha1.LastOperation">
github.com/gardener/gardener/pkg/apis/core/v1alpha1.LastOperation
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
<code>lastError</code></br>
<em>
<a href="../core#core.gardener.cloud/v1alpha1.LastError">
github.com/gardener/gardener/pkg/apis/core/v1alpha1.LastError
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
<code>lastErrors</code></br>
<em>
<a href="../core#core.gardener.cloud/v1alpha1.LastError">
[]github.com/gardener/gardener/pkg/apis/core/v1alpha1.LastError
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#time-v1-meta">
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
<code>seed</code></br>
<em>
string
</em>
</td>
<td>
<p>Seed is the name of the seed cluster that runs the control plane of the Shoot. This value is only written
after a successful create/reconcile operation. It will be used when control planes are moved between Seeds.</p>
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
<em>(Optional)</em>
<p>IsHibernated indicates whether the Shoot is currently hibernated.</p>
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
<p>TechnicalID is the name that is used for creating the Seed namespace, the infrastructure resources, and
basically everything that is related to this particular Shoot.</p>
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
It is used to compute unique hashes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.VolumeType">VolumeType
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudVolumeType">AlicloudVolumeType</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints</a>)
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
<code>class</code></br>
<em>
string
</em>
</td>
<td>
<p>Class is the class of the volume type.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Worker">Worker
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSWorker">AWSWorker</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudWorker">AlicloudWorker</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureWorker">AzureWorker</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPWorker">GCPWorker</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackWorker">OpenStackWorker</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketWorker">PacketWorker</a>)
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
<code>machineType</code></br>
<em>
string
</em>
</td>
<td>
<p>MachineType is the machine type of the worker group.</p>
</td>
</tr>
<tr>
<td>
<code>machineImage</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.ShootMachineImage">
ShootMachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootMachineImage holds information about the machine image to use for all workers.
It will default to the latest version of the first image stated in the referenced CloudProfile if no
value has been provided.</p>
</td>
</tr>
<tr>
<td>
<code>autoScalerMin</code></br>
<em>
int
</em>
</td>
<td>
<p>AutoScalerMin is the minimum number of VMs to create.</p>
</td>
</tr>
<tr>
<td>
<code>autoScalerMax</code></br>
<em>
int
</em>
</td>
<td>
<p>AutoScalerMin is the maximum number of VMs to create.</p>
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
<p>MaxSurge is maximum number of VMs that are created during an update.</p>
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
<p>MaxUnavailable is the maximum number of VMs that can be unavailable during an update.</p>
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
<p>Annotations is a map of key/value pairs for annotations for all the <code>Node</code> objects in this worker pool.</p>
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
<code>taints</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#taint-v1-core">
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
<code>kubelet</code></br>
<em>
<a href="#garden.sapcloud.io/v1beta1.KubeletConfig">
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
</tbody>
</table>
<h3 id="garden.sapcloud.io/v1beta1.Zone">Zone
</h3>
<p>
(<em>Appears on:</em>
<a href="#garden.sapcloud.io/v1beta1.AWSConstraints">AWSConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AlicloudConstraints">AlicloudConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.AzureConstraints">AzureConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.GCPConstraints">GCPConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.OpenStackConstraints">OpenStackConstraints</a>, 
<a href="#garden.sapcloud.io/v1beta1.PacketConstraints">PacketConstraints</a>)
</p>
<p>
<p>Zone contains certain properties of an availability zone.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is a region name.</p>
</td>
</tr>
<tr>
<td>
<code>names</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Names is a list of availability zone names in this region.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>864ace2ad</code>.
</em></p>
