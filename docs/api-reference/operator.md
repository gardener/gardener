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
<h3 id="operator.gardener.cloud/v1alpha1.AdmissionDeploymentSpec">AdmissionDeploymentSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Deployment">Deployment</a>)
</p>
<p>
<p>AdmissionDeploymentSpec contains the deployment specification for the admission controller of an extension.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.DeploymentSpec">
DeploymentSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RuntimeCluster is the deployment configuration for the admission in the runtime cluster. The runtime deployment
is responsible for creating the admission controller in the runtime cluster.</p>
</td>
</tr>
<tr>
<td>
<code>virtualCluster</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DeploymentSpec">
DeploymentSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VirtualCluster is the deployment configuration for the admission deployment in the garden cluster. The garden deployment
installs necessary resources in the virtual garden cluster e.g. RBAC that are necessary for the admission controller.</p>
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<em>(Optional)</em>
<p>Values are the deployment values. The values will be applied to both admission deployments.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.AuditWebhook">AuditWebhook
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerAPIServerConfig">GardenerAPIServerConfig</a>, 
<a href="#operator.gardener.cloud/v1alpha1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>AuditWebhook contains settings related to an audit webhook configuration.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>batchMaxSize</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>BatchMaxSize is the maximum size of a batch.</p>
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
<p>KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this webhook.</p>
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
<p>Version is the API version to send and expect from the webhook.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Authentication">Authentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>Authentication contains settings related to authentication.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>webhook</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.AuthenticationWebhook">
AuthenticationWebhook
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Webhook contains settings related to an authentication webhook configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.AuthenticationWebhook">AuthenticationWebhook
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Authentication">Authentication</a>)
</p>
<p>
<p>AuthenticationWebhook contains settings related to an authentication webhook configuration.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cacheTTL</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CacheTTL is the duration to cache responses from the webhook authenticator.</p>
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
<p>KubeconfigSecretName specifies the name of a secret containing the kubeconfig for this webhook.</p>
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
<p>Version is the API version to send and expect from the webhook.</p>
</td>
</tr>
</tbody>
</table>
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
<em>(Optional)</em>
<p>BucketName is the name of the backup bucket. If not provided, gardener-operator attempts to manage a new bucket.
In this case, the cloud provider credentials provided in the SecretRef must have enough privileges for creating
and deleting buckets.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfig</code></br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the provider-specific configuration passed to BackupBucket resource.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
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
<tr>
<td>
<code>observability</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.ObservabilityRotation
</em>
</td>
<td>
<em>(Optional)</em>
<p>Observability contains information about the observability credential rotation.</p>
</td>
</tr>
<tr>
<td>
<code>workloadIdentityKey</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.WorkloadIdentityKeyRotation">
WorkloadIdentityKeyRotation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkloadIdentityKey contains information about the workload identity key credential rotation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DNS">DNS
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>DNS holds information about DNS settings.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.DNSDomain">
[]DNSDomain
</a>
</em>
</td>
<td>
<p>Domains are the external domains of the virtual garden cluster.
The first given domain in this list is immutable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DNSDomain">DNSDomain
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.DNS">DNS</a>, 
<a href="#operator.gardener.cloud/v1alpha1.Ingress">Ingress</a>)
</p>
<p>
<p>DNSDomain defines a DNS domain with optional provider.</p>
</p>
<table>
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
<p>Name is the domain name.</p>
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
<p>Provider is the name of the DNS provider as declared in the &lsquo;.spec.dns.providers&rsquo; section.
It is only optional, if the <code>.spec.dns</code> section is not provided at all.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DNSManagement">DNSManagement
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenSpec">GardenSpec</a>)
</p>
<p>
<p>DNSManagement contains specifications of DNS providers.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>providers</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DNSProvider">
[]DNSProvider
</a>
</em>
</td>
<td>
<p>Providers is a list of DNS providers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DNSProvider">DNSProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.DNSManagement">DNSManagement</a>)
</p>
<p>
<p>DNSProvider contains the configuration for a DNS provider.</p>
</p>
<table>
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
<code>providerConfig</code></br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>Config is the provider-specific configuration passed to DNSRecord resources.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing the DNS provider credentials.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DashboardGitHub">DashboardGitHub
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerDashboardConfig">GardenerDashboardConfig</a>)
</p>
<p>
<p>DashboardGitHub contains configuration for the GitHub ticketing feature.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiURL</code></br>
<em>
string
</em>
</td>
<td>
<p>APIURL is the URL to the GitHub API.</p>
</td>
</tr>
<tr>
<td>
<code>organisation</code></br>
<em>
string
</em>
</td>
<td>
<p>Organisation is the name of the GitHub organisation.</p>
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
<p>Repository is the name of the GitHub repository.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef is the reference to a secret in the garden namespace containing the GitHub credentials.</p>
</td>
</tr>
<tr>
<td>
<code>pollInterval</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PollInterval is the interval of how often the GitHub API is polled for issue updates. This field is used as a
fallback mechanism to ensure state synchronization, even when there is a GitHub webhook configuration. If a
webhook event is missed or not successfully delivered, the polling will help catch up on any missed updates.
If this field is not provided and there is no &lsquo;webhookSecret&rsquo; key in the referenced secret, it will be
implicitly defaulted to <code>15m</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DashboardOIDC">DashboardOIDC
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerDashboardConfig">GardenerDashboardConfig</a>)
</p>
<p>
<p>DashboardOIDC contains configuration for the OIDC settings.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clientIDPublic</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ClientIDPublic is the public client ID.
Falls back to the API server&rsquo;s OIDC client ID configuration if not set here.</p>
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
<p>The URL of the OpenID issuer, only HTTPS scheme will be accepted. Used to verify the OIDC JSON Web Token (JWT).
Falls back to the API server&rsquo;s OIDC issuer URL configuration if not set here.</p>
</td>
</tr>
<tr>
<td>
<code>sessionLifetime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SessionLifetime is the maximum duration of a session.</p>
</td>
</tr>
<tr>
<td>
<code>additionalScopes</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalScopes is the list of additional OIDC scopes.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef is the reference to a secret in the garden namespace containing the OIDC client ID and secret for the dashboard.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DashboardTerminal">DashboardTerminal
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerDashboardConfig">GardenerDashboardConfig</a>)
</p>
<p>
<p>DashboardTerminal contains configuration for the terminal settings.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>container</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DashboardTerminalContainer">
DashboardTerminalContainer
</a>
</em>
</td>
<td>
<p>Container contains configuration for the dashboard terminal container.</p>
</td>
</tr>
<tr>
<td>
<code>allowedHosts</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AllowedHosts should consist of permitted hostnames (without the scheme) for terminal connections.
It is important to consider that the usage of wildcards follows the rules defined by the content security policy.
&lsquo;<em>.seed.local.gardener.cloud&rsquo;, or &lsquo;</em>.other-seeds.local.gardener.cloud&rsquo;. For more information, see
<a href="https://github.com/gardener/dashboard/blob/master/docs/operations/webterminals.md#allowlist-for-hosts">https://github.com/gardener/dashboard/blob/master/docs/operations/webterminals.md#allowlist-for-hosts</a>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DashboardTerminalContainer">DashboardTerminalContainer
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.DashboardTerminal">DashboardTerminal</a>)
</p>
<p>
<p>DashboardTerminalContainer contains configuration for the dashboard terminal container.</p>
</p>
<table>
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
<p>Image is the container image for the dashboard terminal container.</p>
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
<p>Description is a description for the dashboard terminal container with hints for the user.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Deployment">Deployment
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ExtensionSpec">ExtensionSpec</a>)
</p>
<p>
<p>Deployment specifies how an extension can be installed for a Gardener landscape. It includes the specification
for installing an extension and/or an admission controller.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.ExtensionDeploymentSpec">
ExtensionDeploymentSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExtensionDeployment contains the deployment configuration an extension.</p>
</td>
</tr>
<tr>
<td>
<code>admission</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.AdmissionDeploymentSpec">
AdmissionDeploymentSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionDeployment contains the deployment configuration for an admission controller.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.DeploymentSpec">DeploymentSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.AdmissionDeploymentSpec">AdmissionDeploymentSpec</a>, 
<a href="#operator.gardener.cloud/v1alpha1.ExtensionDeploymentSpec">ExtensionDeploymentSpec</a>)
</p>
<p>
<p>DeploymentSpec is the specification for the deployment of a component.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>helm</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ExtensionHelm">
ExtensionHelm
</a>
</em>
</td>
<td>
<p>Helm contains the specification for a Helm deployment.</p>
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
<code>autoscaling</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.ControlPlaneAutoscaling
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains auto-scaling configuration options for etcd.</p>
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
<code>autoscaling</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.ControlPlaneAutoscaling
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaling contains auto-scaling configuration options for etcd.</p>
</td>
</tr>
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
<h3 id="operator.gardener.cloud/v1alpha1.Extension">Extension
</h3>
<p>
<p>Extension describes a Gardener extension.</p>
</p>
<table>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ExtensionSpec">
ExtensionSpec
</a>
</em>
</td>
<td>
<p>Spec contains the specification of this extension.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>resources</code></br>
<em>
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.ControllerResource
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of combinations of kinds (DNSRecord, Backupbucket, &hellip;) and their actual types
(aws-route53, gcp).</p>
</td>
</tr>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Deployment">
Deployment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment contains deployment configuration for an extension and it&rsquo;s admission controller.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ExtensionStatus">
ExtensionStatus
</a>
</em>
</td>
<td>
<p>Status contains the status of this extension.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ExtensionDeploymentSpec">ExtensionDeploymentSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Deployment">Deployment</a>)
</p>
<p>
<p>ExtensionDeploymentSpec specifies how to install the extension in a gardener landscape. The installation is split into two parts:
- installing the extension in the virtual garden cluster by creating the ControllerRegistration and ControllerDeployment
- installing the extension in the runtime cluster (if necessary).</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>DeploymentSpec</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DeploymentSpec">
DeploymentSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>DeploymentSpec</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>DeploymentSpec is the deployment configuration for the extension.</p>
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<em>(Optional)</em>
<p>Values are the deployment values used in the creation of the ControllerDeployment in the virtual garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeClusterValues</code></br>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<em>(Optional)</em>
<p>RuntimeClusterValues are the deployment values for the extension deployment running in the runtime garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>policy</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.ControllerDeploymentPolicy
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
<h3 id="operator.gardener.cloud/v1alpha1.ExtensionHelm">ExtensionHelm
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.DeploymentSpec">DeploymentSpec</a>)
</p>
<p>
<p>ExtensionHelm is the configuration for a helm deployment.</p>
</p>
<table>
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
<em>(Optional)</em>
<p>OCIRepository defines where to pull the chart from.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ExtensionSpec">ExtensionSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Extension">Extension</a>)
</p>
<p>
<p>ExtensionSpec contains the specification of a Gardener extension.</p>
</p>
<table>
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
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.ControllerResource
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of combinations of kinds (DNSRecord, Backupbucket, &hellip;) and their actual types
(aws-route53, gcp).</p>
</td>
</tr>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Deployment">
Deployment
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment contains deployment configuration for an extension and it&rsquo;s admission controller.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ExtensionStatus">ExtensionStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Extension">Extension</a>)
</p>
<p>
<p>ExtensionStatus is the status of a Gardener extension.</p>
</p>
<table>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
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
<em>(Optional)</em>
<p>Conditions represents the latest available observations of an Extension&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>providerStatus</code></br>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderStatus contains type-specific status.</p>
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
<code>dns</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DNSManagement">
DNSManagement
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNS contains specifications of DNS providers.</p>
</td>
</tr>
<tr>
<td>
<code>extensions</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenExtension">
[]GardenExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extensions contain type and provider information for Garden extensions.</p>
</td>
</tr>
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
<h3 id="operator.gardener.cloud/v1alpha1.GardenExtension">GardenExtension
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenSpec">GardenSpec</a>)
</p>
<p>
<p>GardenExtension contains type and provider information for Garden extensions.</p>
</p>
<table>
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
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfig is the configuration passed to extension resource.</p>
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
<code>dns</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DNSManagement">
DNSManagement
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DNS contains specifications of DNS providers.</p>
</td>
</tr>
<tr>
<td>
<code>extensions</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenExtension">
[]GardenExtension
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extensions contain type and provider information for Garden extensions.</p>
</td>
</tr>
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
<code>lastOperation</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.LastOperation
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastOperation holds information about the last operation on the Garden.</p>
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
<tr>
<td>
<code>encryptedResources</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EncryptedResources is the list of resources which are currently encrypted in the virtual garden by the virtual kube-apiserver.
Resources which are encrypted by default will not appear here.
See <a href="https://github.com/gardener/gardener/blob/master/docs/concepts/operator.md#etcd-encryption-config">https://github.com/gardener/gardener/blob/master/docs/concepts/operator.md#etcd-encryption-config</a> for more details.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Gardener">Gardener
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>Gardener contains the configuration settings for the Gardener components.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clusterIdentity</code></br>
<em>
string
</em>
</td>
<td>
<p>ClusterIdentity is the identity of the garden cluster. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerAPIServer</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerAPIServerConfig">
GardenerAPIServerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIServer contains configuration settings for the gardener-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerAdmissionController</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerAdmissionControllerConfig">
GardenerAdmissionControllerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionController contains configuration settings for the gardener-admission-controller.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerControllerManager</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerControllerManagerConfig">
GardenerControllerManagerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ControllerManager contains configuration settings for the gardener-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerScheduler</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerSchedulerConfig">
GardenerSchedulerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Scheduler contains configuration settings for the gardener-scheduler.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerDashboard</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerDashboardConfig">
GardenerDashboardConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Dashboard contains configuration settings for the gardener-dashboard.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerDiscoveryServer</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerDiscoveryServerConfig">
GardenerDiscoveryServerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DiscoveryServer contains configuration settings for the gardener-discovery-server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenerAPIServerConfig">GardenerAPIServerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">Gardener</a>)
</p>
<p>
<p>GardenerAPIServerConfig contains configuration settings for the gardener-apiserver.</p>
</p>
<table>
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.KubernetesConfig
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
[]github.com/gardener/gardener/pkg/apis/core/v1beta1.AdmissionPlugin
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener),
and, if desired, the corresponding configuration.</p>
</td>
</tr>
<tr>
<td>
<code>auditConfig</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.AuditConfig
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuditConfig contains configuration settings for the audit of the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>auditWebhook</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.AuditWebhook">
AuditWebhook
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuditWebhook contains settings related to an audit webhook configuration.</p>
</td>
</tr>
<tr>
<td>
<code>logging</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.APIServerLogging
</em>
</td>
<td>
<em>(Optional)</em>
<p>Logging contains configuration for the log level and HTTP access logs.</p>
</td>
</tr>
<tr>
<td>
<code>requests</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.APIServerRequests
</em>
</td>
<td>
<em>(Optional)</em>
<p>Requests contains configuration for request-specific settings for the kube-apiserver.</p>
</td>
</tr>
<tr>
<td>
<code>watchCacheSizes</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.WatchCacheSizes
</em>
</td>
<td>
<em>(Optional)</em>
<p>WatchCacheSizes contains configuration of the API server&rsquo;s watch cache sizes.
Configuring these flags might be useful for large-scale Garden clusters with a lot of parallel update requests
and a lot of watching controllers (e.g. large ManagedSeed clusters). When the API server&rsquo;s watch cache&rsquo;s
capacity is too small to cope with the amount of update requests and watchers for a particular resource, it
might happen that controller watches are permanently stopped with <code>too old resource version</code> errors.
Starting from kubernetes v1.19, the API server&rsquo;s watch cache size is adapted dynamically and setting the watch
cache size flags will have no effect, except when setting it to 0 (which disables the watch cache).</p>
</td>
</tr>
<tr>
<td>
<code>encryptionConfig</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.EncryptionConfig
</em>
</td>
<td>
<em>(Optional)</em>
<p>EncryptionConfig contains customizable encryption configuration of the Gardener API server.</p>
</td>
</tr>
<tr>
<td>
<code>goAwayChance</code></br>
<em>
float64
</em>
</td>
<td>
<em>(Optional)</em>
<p>GoAwayChance can be used to prevent HTTP/2 clients from getting stuck on a single apiserver, randomly close a
connection (GOAWAY). The client&rsquo;s other in-flight requests won&rsquo;t be affected, and the client will reconnect,
likely landing on a different apiserver after going through the load balancer again. This field sets the fraction
of requests that will be sent a GOAWAY. Min is 0 (off), Max is 0.02 (<sup>1</sup>&frasl;<sub>50</sub> requests); 0.001 (<sup>1</sup>&frasl;<sub>1000</sub>) is a
recommended starting point.</p>
</td>
</tr>
<tr>
<td>
<code>adminKubeconfigMaxExpiration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdminKubeconfigMaxExpiration is the maximum validity duration of a credential requested to a Shoot by an AdminKubeconfigRequest.
If an otherwise valid AdminKubeconfigRequest with a validity duration larger than this value is requested,
a credential will be issued with a validity duration of this value.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenerAdmissionControllerConfig">GardenerAdmissionControllerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">Gardener</a>)
</p>
<p>
<p>GardenerAdmissionControllerConfig contains configuration settings for the gardener-admission-controller.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>logLevel</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LogLevel is the configured log level for the gardener-admission-controller. Must be one of [info,debug,error].
Defaults to info.</p>
</td>
</tr>
<tr>
<td>
<code>resourceAdmissionConfiguration</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ResourceAdmissionConfiguration">
ResourceAdmissionConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceAdmissionConfiguration is the configuration for resource size restrictions for arbitrary Group-Version-Kinds.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenerControllerManagerConfig">GardenerControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">Gardener</a>)
</p>
<p>
<p>GardenerControllerManagerConfig contains configuration settings for the gardener-controller-manager.</p>
</p>
<table>
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.KubernetesConfig
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
<code>defaultProjectQuotas</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ProjectQuotaConfiguration">
[]ProjectQuotaConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultProjectQuotas is the default configuration matching projects are set up with if a quota is not already
specified.</p>
</td>
</tr>
<tr>
<td>
<code>logLevel</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LogLevel is the configured log level for the gardener-controller-manager. Must be one of [info,debug,error].
Defaults to info.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenerDashboardConfig">GardenerDashboardConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">Gardener</a>)
</p>
<p>
<p>GardenerDashboardConfig contains configuration settings for the gardener-dashboard.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enableTokenLogin</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableTokenLogin specifies whether it is possible to log into the dashboard with a JWT token. If disabled, OIDC
must be configured.</p>
</td>
</tr>
<tr>
<td>
<code>frontendConfigMapRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FrontendConfigMapRef is the reference to a ConfigMap in the garden namespace containing the frontend
configuration.</p>
</td>
</tr>
<tr>
<td>
<code>assetsConfigMapRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AssetsConfigMapRef is the reference to a ConfigMap in the garden namespace containing the assets (logos/icons).</p>
</td>
</tr>
<tr>
<td>
<code>gitHub</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DashboardGitHub">
DashboardGitHub
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>GitHub contains configuration for the GitHub ticketing feature.</p>
</td>
</tr>
<tr>
<td>
<code>logLevel</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LogLevel is the configured log level. Must be one of [trace,debug,info,warn,error].
Defaults to info.</p>
</td>
</tr>
<tr>
<td>
<code>oidcConfig</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DashboardOIDC">
DashboardOIDC
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OIDCConfig contains configuration for the OIDC provider. This field must be provided when EnableTokenLogin is false.</p>
</td>
</tr>
<tr>
<td>
<code>terminal</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DashboardTerminal">
DashboardTerminal
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Terminal contains configuration for the terminal settings.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GardenerDiscoveryServerConfig">GardenerDiscoveryServerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">Gardener</a>)
</p>
<p>
<p>GardenerDiscoveryServerConfig contains configuration settings for the gardener-discovery-server.</p>
</p>
<h3 id="operator.gardener.cloud/v1alpha1.GardenerSchedulerConfig">GardenerSchedulerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">Gardener</a>)
</p>
<p>
<p>GardenerSchedulerConfig contains configuration settings for the gardener-scheduler.</p>
</p>
<table>
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.KubernetesConfig
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
<code>logLevel</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LogLevel is the configured log level for the gardener-scheduler. Must be one of [info,debug,error].
Defaults to info.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.GroupResource">GroupResource
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>GroupResource contains a list of resources which should be stored in etcd-events instead of etcd-main.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>group</code></br>
<em>
string
</em>
</td>
<td>
<p>Group is the API group name.</p>
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
<p>Resource is the resource name.</p>
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
<h3 id="operator.gardener.cloud/v1alpha1.Ingress">Ingress
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">RuntimeCluster</a>)
</p>
<p>
<p>Ingress configures the Ingress specific settings of the runtime cluster.</p>
</p>
<table>
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
<a href="#operator.gardener.cloud/v1alpha1.DNSDomain">
[]DNSDomain
</a>
</em>
</td>
<td>
<p>Domains specify the ingress domains of the cluster pointing to the ingress controller endpoint. They will be used
to construct ingress URLs for system applications running in runtime cluster.</p>
</td>
</tr>
<tr>
<td>
<code>controller</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.IngressController
</em>
</td>
<td>
<p>Controller configures a Gardener managed Ingress Controller listening on the ingressDomain.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.KubeAPIServerConfig">KubeAPIServerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Kubernetes">Kubernetes</a>)
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
<code>KubeAPIServerConfig</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.KubeAPIServerConfig
</em>
</td>
<td>
<p>
(Members of <code>KubeAPIServerConfig</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>KubeAPIServerConfig contains all configuration values not specific to the virtual garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>auditWebhook</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.AuditWebhook">
AuditWebhook
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AuditWebhook contains settings related to an audit webhook configuration.</p>
</td>
</tr>
<tr>
<td>
<code>authentication</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Authentication">
Authentication
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Authentication contains settings related to authentication.</p>
</td>
</tr>
<tr>
<td>
<code>resourcesToStoreInETCDEvents</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.GroupResource">
[]GroupResource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourcesToStoreInETCDEvents contains a list of resources which should be stored in etcd-events instead of
etcd-main. The &lsquo;events&rsquo; resource is always stored in etcd-events. Note that adding or removing resources from
this list will not migrate them automatically from the etcd-main to etcd-events or vice versa.</p>
</td>
</tr>
<tr>
<td>
<code>sni</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.SNI">
SNI
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNI contains configuration options for the TLS SNI settings.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.KubeControllerManagerConfig">KubeControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Kubernetes">Kubernetes</a>)
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
<code>KubeControllerManagerConfig</code></br>
<em>
github.com/gardener/gardener/pkg/apis/core/v1beta1.KubeControllerManagerConfig
</em>
</td>
<td>
<p>
(Members of <code>KubeControllerManagerConfig</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>KubeControllerManagerConfig contains all configuration values not specific to the virtual garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>certificateSigningDuration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertificateSigningDuration is the maximum length of duration signed certificates will be given. Individual CSRs
may request shorter certs by setting <code>spec.expirationSeconds</code>.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Kubernetes">Kubernetes
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>Kubernetes contains the version and configuration options for the Kubernetes components of the virtual garden
cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kubeAPIServer</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.KubeAPIServerConfig">
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
<a href="#operator.gardener.cloud/v1alpha1.KubeControllerManagerConfig">
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the semantic Kubernetes version to use for the virtual garden cluster.</p>
</td>
</tr>
</tbody>
</table>
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
<h3 id="operator.gardener.cloud/v1alpha1.Networking">Networking
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.VirtualCluster">VirtualCluster</a>)
</p>
<p>
<p>Networking defines networking parameters for the virtual garden cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>services</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Services are the CIDRs of the service network. Elements can be appended to this list, but not removed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ProjectQuotaConfiguration">ProjectQuotaConfiguration
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerControllerManagerConfig">GardenerControllerManagerConfig</a>)
</p>
<p>
<p>ProjectQuotaConfiguration defines quota configurations.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>config</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#resourcequota-v1-core">
Kubernetes core/v1.ResourceQuota
</a>
</em>
</td>
<td>
<p>Config is the corev1.ResourceQuota specification used for the project set-up.</p>
</td>
</tr>
<tr>
<td>
<code>projectSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProjectSelector is an optional setting to select the projects considered for quotas.
Defaults to empty LabelSelector, which matches all projects.</p>
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
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Region is the region the cluster is deployed to.</p>
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
<p>Zones is the list of availability zones the cluster is deployed to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ResourceAdmissionConfiguration">ResourceAdmissionConfiguration
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.GardenerAdmissionControllerConfig">GardenerAdmissionControllerConfig</a>)
</p>
<p>
<p>ResourceAdmissionConfiguration contains settings about arbitrary kinds and the size each resource should have at most.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>limits</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ResourceLimit">
[]ResourceLimit
</a>
</em>
</td>
<td>
<p>Limits contains configuration for resources which are subjected to size limitations.</p>
</td>
</tr>
<tr>
<td>
<code>unrestrictedSubjects</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#subject-v1-rbac">
[]Kubernetes rbac/v1.Subject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UnrestrictedSubjects contains references to users, groups, or service accounts which aren&rsquo;t subjected to any resource size limit.</p>
</td>
</tr>
<tr>
<td>
<code>operationMode</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.ResourceAdmissionWebhookMode">
ResourceAdmissionWebhookMode
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OperationMode specifies the mode the webhooks operates in. Allowed values are &ldquo;block&rdquo; and &ldquo;log&rdquo;. Defaults to &ldquo;block&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.ResourceAdmissionWebhookMode">ResourceAdmissionWebhookMode
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ResourceAdmissionConfiguration">ResourceAdmissionConfiguration</a>)
</p>
<p>
<p>ResourceAdmissionWebhookMode is an alias type for the resource admission webhook mode.</p>
</p>
<h3 id="operator.gardener.cloud/v1alpha1.ResourceLimit">ResourceLimit
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.ResourceAdmissionConfiguration">ResourceAdmissionConfiguration</a>)
</p>
<p>
<p>ResourceLimit contains settings about a kind and the size each resource should have at most.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiGroups</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIGroups is the name of the APIGroup that contains the limited resource. WildcardAll represents all groups.</p>
</td>
</tr>
<tr>
<td>
<code>apiVersions</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>APIVersions is the version of the resource. WildcardAll represents all versions.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Resources is the name of the resource this rule applies to. WildcardAll represents all resources.</p>
</td>
</tr>
<tr>
<td>
<code>size</code></br>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<p>Size specifies the imposed limit.</p>
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
<code>ingress</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Ingress">
Ingress
</a>
</em>
</td>
<td>
<p>Ingress configures Ingress specific settings for the Garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>networking</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeNetworking">
RuntimeNetworking
</a>
</em>
</td>
<td>
<p>Networking defines the networking configuration of the runtime cluster.</p>
</td>
</tr>
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
<tr>
<td>
<code>volume</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Volume">
Volume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains settings for persistent volumes created in the runtime cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.RuntimeNetworking">RuntimeNetworking
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">RuntimeCluster</a>)
</p>
<p>
<p>RuntimeNetworking defines the networking configuration of the runtime cluster.</p>
</p>
<table>
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
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Nodes are the CIDRs of the node network. Elements can be appended to this list, but not removed.</p>
</td>
</tr>
<tr>
<td>
<code>pods</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Pods are the CIDRs of the pod network. Elements can be appended to this list, but not removed.</p>
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
<p>Services are the CIDRs of the service network. Elements can be appended to this list, but not removed.</p>
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
<p>BlockCIDRs is a list of network addresses that should be blocked.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.SNI">SNI
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.KubeAPIServerConfig">KubeAPIServerConfig</a>)
</p>
<p>
<p>SNI contains configuration options for the TLS SNI settings.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretName</code></br>
<em>
string
</em>
</td>
<td>
<p>SecretName is the name of a secret containing the TLS certificate and private key.</p>
</td>
</tr>
<tr>
<td>
<code>domainPatterns</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DomainPatterns is a list of fully qualified domain names, possibly with prefixed wildcard segments. The domain
patterns also allow IP addresses, but IPs should only be used if the apiserver has visibility to the IP address
requested by a client. If no domain patterns are provided, the names of the certificate are extracted.
Non-wildcard matches trump over wildcard matches, explicit domain patterns trump over extracted names.</p>
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
<h3 id="operator.gardener.cloud/v1alpha1.SettingTopologyAwareRouting">SettingTopologyAwareRouting
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.Settings">Settings</a>)
</p>
<p>
<p>SettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the cluster.
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
<p>Enabled controls whether certain Services deployed in the cluster should be topology-aware.
These Services are virtual-garden-etcd-main-client, virtual-garden-etcd-events-client and virtual-garden-kube-apiserver.
Additionally, other components that are deployed to the runtime cluster via other means can read this field and
according to its value enable/disable topology-aware routing for their Services.</p>
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
<tr>
<td>
<code>topologyAwareRouting</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.SettingTopologyAwareRouting">
SettingTopologyAwareRouting
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TopologyAwareRouting controls certain settings for topology-aware traffic routing in the cluster.
See <a href="https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md">https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md</a>.</p>
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
<code>dns</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.DNS">
DNS
</a>
</em>
</td>
<td>
<p>DNS holds information about DNS settings.</p>
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
<code>gardener</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Gardener">
Gardener
</a>
</em>
</td>
<td>
<p>Gardener contains the configuration options for the Gardener control plane components.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetes</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Kubernetes">
Kubernetes
</a>
</em>
</td>
<td>
<p>Kubernetes contains the version and configuration options for the Kubernetes components of the virtual garden
cluster.</p>
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
<tr>
<td>
<code>networking</code></br>
<em>
<a href="#operator.gardener.cloud/v1alpha1.Networking">
Networking
</a>
</em>
</td>
<td>
<p>Networking contains information about cluster networking such as CIDRs, etc.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.Volume">Volume
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.RuntimeCluster">RuntimeCluster</a>)
</p>
<p>
<p>Volume contains settings for persistent volumes created in the runtime cluster.</p>
</p>
<table>
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
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinimumSize defines the minimum size that should be used for PVCs in the runtime cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operator.gardener.cloud/v1alpha1.WorkloadIdentityKeyRotation">WorkloadIdentityKeyRotation
</h3>
<p>
(<em>Appears on:</em>
<a href="#operator.gardener.cloud/v1alpha1.CredentialsRotation">CredentialsRotation</a>)
</p>
<p>
<p>WorkloadIdentityKeyRotation contains information about the workload identity key credential rotation.</p>
</p>
<table>
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
github.com/gardener/gardener/pkg/apis/core/v1beta1.CredentialsRotationPhase
</em>
</td>
<td>
<p>Phase describes the phase of the workload identity key credential rotation.</p>
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
<p>LastCompletionTime is the most recent time when the workload identity key credential rotation was successfully
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
<p>LastInitiationTime is the most recent time when the workload identity key credential rotation was initiated.</p>
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
<p>LastInitiationFinishedTime is the recent time when the workload identity key credential rotation initiation was
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
<p>LastCompletionTriggeredTime is the recent time when the workload identity key credential rotation completion was
triggered.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
