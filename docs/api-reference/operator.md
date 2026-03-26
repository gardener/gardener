<p>Packages:</p>
<ul>
<li>
<a href="#operator.gardener.cloud%2fv1alpha1">operator.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="operator.gardener.cloud/v1alpha1">operator.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#extension">Extension</a>
</li>
<li>
<a href="#garden">Garden</a>
</li>
</ul>

<h3 id="admissiondeploymentspec">AdmissionDeploymentSpec
</h3>


<p>
(<em>Appears on:</em><a href="#deployment">Deployment</a>)
</p>

<p>
AdmissionDeploymentSpec contains the deployment specification for the admission controller of an extension.
</p>

<table>
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
<a href="#deploymentspec">DeploymentSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RuntimeCluster is the deployment configuration for the admission in the runtime cluster. The runtime deployment<br />is responsible for creating the admission controller in the runtime cluster.</p>
</td>
</tr>
<tr>
<td>
<code>virtualCluster</code></br>
<em>
<a href="#deploymentspec">DeploymentSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VirtualCluster is the deployment configuration for the admission deployment in the garden cluster. The garden deployment<br />installs necessary resources in the virtual garden cluster e.g. RBAC that are necessary for the admission controller.</p>
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
<p>Values are the deployment values. The values will be applied to both admission deployments.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="auditwebhook">AuditWebhook
</h3>


<p>
(<em>Appears on:</em><a href="#gardenerapiserverconfig">GardenerAPIServerConfig</a>, <a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
AuditWebhook contains settings related to an audit webhook configuration.
</p>

<table>
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
integer
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


<h3 id="authentication">Authentication
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
Authentication contains settings related to authentication.
</p>

<table>
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
<a href="#authenticationwebhook">AuthenticationWebhook</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Webhook contains settings related to an authentication webhook configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="authenticationwebhook">AuthenticationWebhook
</h3>


<p>
(<em>Appears on:</em><a href="#authentication">Authentication</a>)
</p>

<p>
AuthenticationWebhook contains settings related to an authentication webhook configuration.
</p>

<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
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


<h3 id="backup">Backup
</h3>


<p>
(<em>Appears on:</em><a href="#etcdmain">ETCDMain</a>)
</p>

<p>
Backup contains the object store configuration for backups for the virtual garden etcd.
</p>

<table>
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
<p>BucketName is the name of the backup bucket. If not provided, gardener-operator attempts to manage a new bucket.<br />In this case, the cloud provider credentials provided in the SecretRef must have enough privileges for creating<br />and deleting buckets.</p>
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
<p>ProviderConfig is the provider-specific configuration passed to BackupBucket resource.</p>
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
<p>Region is a region name. If undefined, the provider region is used. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing the cloud provider credentials for the object store where<br />backups should be stored. It should have enough privileges to manipulate the objects as well as buckets.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplane">ControlPlane
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
ControlPlane holds information about the general settings for the control plane of the virtual garden cluster.
</p>

<table>
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
<p>HighAvailability holds the configuration settings for high availability settings.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="credentials">Credentials
</h3>


<p>
(<em>Appears on:</em><a href="#gardenstatus">GardenStatus</a>)
</p>

<p>
Credentials contains information about the virtual garden cluster credentials.
</p>

<table>
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
<a href="#credentialsrotation">CredentialsRotation</a>
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
<p>EncryptionAtRest contains information about garden data encryption at rest.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="credentialsrotation">CredentialsRotation
</h3>


<p>
(<em>Appears on:</em><a href="#credentials">Credentials</a>)
</p>

<p>
CredentialsRotation contains information about the rotation of credentials.
</p>

<table>
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
<code>workloadIdentityKey</code></br>
<em>
<a href="#workloadidentitykeyrotation">WorkloadIdentityKeyRotation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WorkloadIdentityKey contains information about the workload identity key credential rotation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dns">DNS
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
DNS holds information about DNS settings.
</p>

<table>
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
<a href="#dnsdomain">DNSDomain</a> array
</em>
</td>
<td>
<p>Domains are the external domains of the virtual garden cluster.<br />The first given domain in this list is immutable.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsdomain">DNSDomain
</h3>


<p>
(<em>Appears on:</em><a href="#dns">DNS</a>, <a href="#gardenerdiscoveryserverconfig">GardenerDiscoveryServerConfig</a>, <a href="#ingress">Ingress</a>)
</p>

<p>
DNSDomain defines a DNS domain with optional provider.
</p>

<table>
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
<p>Provider is the name of the DNS provider as declared in the '.spec.dns.providers' section.<br />It is only optional, if the `.spec.dns` section is not provided at all.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsmanagement">DNSManagement
</h3>


<p>
(<em>Appears on:</em><a href="#gardenspec">GardenSpec</a>)
</p>

<p>
DNSManagement contains specifications of DNS providers.
</p>

<table>
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
<a href="#dnsprovider">DNSProvider</a> array
</em>
</td>
<td>
<p>Providers is a list of DNS providers.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dnsprovider">DNSProvider
</h3>


<p>
(<em>Appears on:</em><a href="#dnsmanagement">DNSManagement</a>)
</p>

<p>
DNSProvider contains the configuration for a DNS provider.
</p>

<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#rawextension-runtime-pkg">RawExtension</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<p>SecretRef is a reference to a Secret object containing the DNS provider credentials.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dashboardgithub">DashboardGitHub
</h3>


<p>
(<em>Appears on:</em><a href="#gardenerdashboardconfig">GardenerDashboardConfig</a>)
</p>

<p>
DashboardGitHub contains configuration for the GitHub ticketing feature.
</p>

<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PollInterval is the interval of how often the GitHub API is polled for issue updates. This field is used as a<br />fallback mechanism to ensure state synchronization, even when there is a GitHub webhook configuration. If a<br />webhook event is missed or not successfully delivered, the polling will help catch up on any missed updates.<br />If this field is not provided and there is no 'webhookSecret' key in the referenced secret, it will be<br />implicitly defaulted to `15m`.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dashboardingress">DashboardIngress
</h3>


<p>
(<em>Appears on:</em><a href="#gardenerdashboardconfig">GardenerDashboardConfig</a>)
</p>

<p>
DashboardIngress contains configuration for the dashboard ingress resource.
</p>

<table>
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
<p>Enabled controls whether the Dashboard Ingress resource will be deployed to the cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dashboardoidc">DashboardOIDC
</h3>


<p>
(<em>Appears on:</em><a href="#gardenerdashboardconfig">GardenerDashboardConfig</a>)
</p>

<p>
DashboardOIDC contains configuration for the OIDC settings.
</p>

<table>
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
<p>ClientIDPublic is the public client ID.<br />Falls back to the API server's OIDC client ID configuration if not set here.</p>
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
<p>The URL of the OpenID issuer, only HTTPS scheme will be accepted. Used to verify the OIDC JSON Web Token (JWT).<br />Falls back to the API server's OIDC issuer URL configuration if not set here.</p>
</td>
</tr>
<tr>
<td>
<code>sessionLifetime</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
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
string array
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<p>SecretRef is the reference to a secret in the garden namespace containing the OIDC client ID and secret for the dashboard.</p>
</td>
</tr>
<tr>
<td>
<code>certificateAuthoritySecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertificateAuthoritySecretRef is the reference to a secret in the garden namespace containing a custom CA certificate under the "ca.crt" key</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dashboardterminal">DashboardTerminal
</h3>


<p>
(<em>Appears on:</em><a href="#gardenerdashboardconfig">GardenerDashboardConfig</a>)
</p>

<p>
DashboardTerminal contains configuration for the terminal settings.
</p>

<table>
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
<a href="#dashboardterminalcontainer">DashboardTerminalContainer</a>
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
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AllowedHosts should consist of permitted hostnames (without the scheme) for terminal connections.<br />It is important to consider that the usage of wildcards follows the rules defined by the content security policy.<br />'*.seed.local.gardener.cloud', or '*.other-seeds.local.gardener.cloud'. For more information, see<br />https://github.com/gardener/dashboard/blob/master/docs/operations/webterminals.md#allowlist-for-hosts.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="dashboardterminalcontainer">DashboardTerminalContainer
</h3>


<p>
(<em>Appears on:</em><a href="#dashboardterminal">DashboardTerminal</a>)
</p>

<p>
DashboardTerminalContainer contains configuration for the dashboard terminal container.
</p>

<table>
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


<h3 id="deployment">Deployment
</h3>


<p>
(<em>Appears on:</em><a href="#extensionspec">ExtensionSpec</a>)
</p>

<p>
Deployment specifies how an extension can be installed for a Gardener landscape. It includes the specification
for installing an extension and/or an admission controller.
</p>

<table>
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
<a href="#extensiondeploymentspec">ExtensionDeploymentSpec</a>
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
<a href="#admissiondeploymentspec">AdmissionDeploymentSpec</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionDeployment contains the deployment configuration for an admission controller.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="deploymentspec">DeploymentSpec
</h3>


<p>
(<em>Appears on:</em><a href="#admissiondeploymentspec">AdmissionDeploymentSpec</a>, <a href="#extensiondeploymentspec">ExtensionDeploymentSpec</a>)
</p>

<p>
DeploymentSpec is the specification for the deployment of a component.
</p>

<table>
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
<a href="#extensionhelm">ExtensionHelm</a>
</em>
</td>
<td>
<p>Helm contains the specification for a Helm deployment.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcd">ETCD
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
ETCD contains configuration for the etcds of the virtual garden cluster.
</p>

<table>
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
<a href="#etcdmain">ETCDMain</a>
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
<a href="#etcdevents">ETCDEvents</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Events contains configuration for the events etcd.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcdevents">ETCDEvents
</h3>


<p>
(<em>Appears on:</em><a href="#etcd">ETCD</a>)
</p>

<p>
ETCDEvents contains configuration for the events etcd.
</p>

<table>
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
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#storage">Storage</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage contains storage configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="etcdmain">ETCDMain
</h3>


<p>
(<em>Appears on:</em><a href="#etcd">ETCD</a>)
</p>

<p>
ETCDMain contains configuration for the main etcd.
</p>

<table>
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
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#backup">Backup</a>
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
<a href="#storage">Storage</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage contains storage configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="encryptionatrest">EncryptionAtRest
</h3>


<p>
(<em>Appears on:</em><a href="#credentials">Credentials</a>)
</p>

<p>
EncryptionAtRest contains information about virtual garden data encryption at rest.
</p>

<table>
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
<p>Resources is the list of resources which are currently encrypted in the virtual garden by the virtual kube-apiserver.<br />Resources which are encrypted by default will not appear here.<br />See https://github.com/gardener/gardener/blob/master/docs/concepts/operator.md#etcd-encryption-config for more details.</p>
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
<p>Provider contains information about virtual garden encryption provider.</p>
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
EncryptionProviderStatus contains information about virtual garden encryption provider.
</p>

<table>
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


<h3 id="extension">Extension
</h3>


<p>
Extension describes a Gardener extension.
</p>

<table>
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
<p>Spec contains the specification of this extension.</p>
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
<p>Status contains the status of this extension.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="extensiondeploymentspec">ExtensionDeploymentSpec
</h3>


<p>
(<em>Appears on:</em><a href="#deployment">Deployment</a>)
</p>

<p>
ExtensionDeploymentSpec specifies how to install the extension in a gardener landscape. The installation is split into two parts:
- installing the extension in the virtual garden cluster by creating the ControllerRegistration and ControllerDeployment
- installing the extension in the runtime cluster (if necessary).
</p>

<table>
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
<a href="#extensionhelm">ExtensionHelm</a>
</em>
</td>
<td>
<p>Helm contains the specification for a Helm deployment.</p>
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
<p>Values are the deployment values used in the creation of the ControllerDeployment in the virtual garden cluster.</p>
</td>
</tr>
<tr>
<td>
<code>runtimeClusterValues</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#json-v1-apiextensions-k8s-io">JSON</a>
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


<h3 id="extensionhelm">ExtensionHelm
</h3>


<p>
(<em>Appears on:</em><a href="#deploymentspec">DeploymentSpec</a>, <a href="#extensiondeploymentspec">ExtensionDeploymentSpec</a>)
</p>

<p>
ExtensionHelm is the configuration for a helm deployment.
</p>

<table>
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
<em>(Optional)</em>
<p>OCIRepository defines where to pull the chart from.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="extensionspec">ExtensionSpec
</h3>


<p>
(<em>Appears on:</em><a href="#extension">Extension</a>)
</p>

<p>
ExtensionSpec contains the specification of a Gardener extension.
</p>

<table>
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
ControllerResource array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of combinations of kinds (DNSRecord, Backupbucket, ...) and their actual types<br />(aws-route53, gcp).</p>
</td>
</tr>
<tr>
<td>
<code>deployment</code></br>
<em>
<a href="#deployment">Deployment</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Deployment contains deployment configuration for an extension and it's admission controller.</p>
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
ExtensionStatus is the status of a Gardener extension.
</p>

<table>
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
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
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
<p>Conditions represents the latest available observations of an Extension's current state.</p>
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


<h3 id="garden">Garden
</h3>


<p>
Garden describes a list of gardens.
</p>

<table>
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
<a href="#gardenspec">GardenSpec</a>
</em>
</td>
<td>
<p>Spec contains the specification of this garden.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#gardenstatus">GardenStatus</a>
</em>
</td>
<td>
<p>Status contains the status of this garden.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenextension">GardenExtension
</h3>


<p>
(<em>Appears on:</em><a href="#gardenspec">GardenSpec</a>)
</p>

<p>
GardenExtension contains type and provider information for Garden extensions.
</p>

<table>
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

</tbody>
</table>


<h3 id="gardenspec">GardenSpec
</h3>


<p>
(<em>Appears on:</em><a href="#garden">Garden</a>)
</p>

<p>
GardenSpec contains the specification of a garden environment.
</p>

<table>
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
<a href="#dnsmanagement">DNSManagement</a>
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
<a href="#gardenextension">GardenExtension</a> array
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
<a href="#runtimecluster">RuntimeCluster</a>
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
<a href="#virtualcluster">VirtualCluster</a>
</em>
</td>
<td>
<p>VirtualCluster contains configuration for the virtual cluster.</p>
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
<p>Resources holds a list of named resource references that can be referred to in extension configs by their names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenstatus">GardenStatus
</h3>


<p>
(<em>Appears on:</em><a href="#garden">Garden</a>)
</p>

<p>
GardenStatus is the status of a garden environment.
</p>

<table>
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
<p>Gardener holds information about the Gardener which last acted on the Garden.</p>
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
<p>Conditions is a list of conditions.</p>
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
<p>LastOperation holds information about the last operation on the Garden.</p>
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
<code>credentials</code></br>
<em>
<a href="#credentials">Credentials</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Credentials contains information about the virtual garden cluster credentials.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardener">Gardener
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
Gardener contains the configuration settings for the Gardener components.
</p>

<table>
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
<a href="#gardenerapiserverconfig">GardenerAPIServerConfig</a>
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
<a href="#gardeneradmissioncontrollerconfig">GardenerAdmissionControllerConfig</a>
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
<a href="#gardenercontrollermanagerconfig">GardenerControllerManagerConfig</a>
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
<a href="#gardenerschedulerconfig">GardenerSchedulerConfig</a>
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
<a href="#gardenerdashboardconfig">GardenerDashboardConfig</a>
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
<a href="#gardenerdiscoveryserverconfig">GardenerDiscoveryServerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DiscoveryServer contains configuration settings for the gardener-discovery-server.<br />Once enabled, the gardener-discovery-server deployment cannot be removed and its domain cannot be changed.<br />Otherwise, workload identity and/or shoot service account tokens referencing the gardener-discovery-server in the<br />issuer URL might become unusable.<br />This field is optional, but once set, it cannot be removed anymore.</p>
</td>
</tr>
<tr>
<td>
<code>gardenerResourceManager</code></br>
<em>
<a href="#gardenerresourcemanagerconfig">GardenerResourceManagerConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceManager contains configuration settings for the gardener-resource-manager.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenerapiserverconfig">GardenerAPIServerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerAPIServerConfig contains configuration settings for the gardener-apiserver.
</p>

<table>
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
AdmissionPlugin array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdmissionPlugins contains the list of user-defined admission plugins (additional to those managed by Gardener),<br />and, if desired, the corresponding configuration.</p>
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
<code>auditWebhook</code></br>
<em>
<a href="#auditwebhook">AuditWebhook</a>
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
<code>watchCacheSizes</code></br>
<em>
<a href="#watchcachesizes">WatchCacheSizes</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>WatchCacheSizes contains configuration of the API server's watch cache sizes.<br />Configuring these flags might be useful for large-scale Garden clusters with a lot of parallel update requests<br />and a lot of watching controllers (e.g. large ManagedSeed clusters). When the API server's watch cache's<br />capacity is too small to cope with the amount of update requests and watchers for a particular resource, it<br />might happen that controller watches are permanently stopped with `too old resource version` errors.<br />Starting from kubernetes v1.19, the API server's watch cache size is adapted dynamically and setting the watch<br />cache size flags will have no effect, except when setting it to 0 (which disables the watch cache).</p>
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
<p>EncryptionConfig contains customizable encryption configuration of the Gardener API server.</p>
</td>
</tr>
<tr>
<td>
<code>goAwayChance</code></br>
<em>
float
</em>
</td>
<td>
<em>(Optional)</em>
<p>GoAwayChance can be used to prevent HTTP/2 clients from getting stuck on a single apiserver, randomly close a<br />connection (GOAWAY). The client's other in-flight requests won't be affected, and the client will reconnect,<br />likely landing on a different apiserver after going through the load balancer again. This field sets the fraction<br />of requests that will be sent a GOAWAY. Min is 0 (off), Max is 0.02 (1/50 requests); 0.001 (1/1000) is a<br />recommended starting point.</p>
</td>
</tr>
<tr>
<td>
<code>shootAdminKubeconfigMaxExpiration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootAdminKubeconfigMaxExpiration is the maximum validity duration of a credential requested to a Shoot by an AdminKubeconfigRequest.<br />If an otherwise valid AdminKubeconfigRequest with a validity duration larger than this value is requested,<br />a credential will be issued with a validity duration of this value.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardeneradmissioncontrollerconfig">GardenerAdmissionControllerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerAdmissionControllerConfig contains configuration settings for the gardener-admission-controller.
</p>

<table>
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
<p>LogLevel is the configured log level for the gardener-admission-controller. Must be one of [info,debug,error].<br />Defaults to info.</p>
</td>
</tr>
<tr>
<td>
<code>resourceAdmissionConfiguration</code></br>
<em>
<a href="#resourceadmissionconfiguration">ResourceAdmissionConfiguration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceAdmissionConfiguration is the configuration for resource size restrictions for arbitrary Group-Version-Kinds.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenercontrollermanagerconfig">GardenerControllerManagerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerControllerManagerConfig contains configuration settings for the gardener-controller-manager.
</p>

<table>
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
<code>defaultProjectQuotas</code></br>
<em>
<a href="#projectquotaconfiguration">ProjectQuotaConfiguration</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DefaultProjectQuotas is the default configuration matching projects are set up with if a quota is not already<br />specified.</p>
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
<p>LogLevel is the configured log level for the gardener-controller-manager. Must be one of [info,debug,error].<br />Defaults to info.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenerdashboardconfig">GardenerDashboardConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerDashboardConfig contains configuration settings for the gardener-dashboard.
</p>

<table>
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>EnableTokenLogin specifies whether it is possible to log into the dashboard with a JWT token. If disabled, OIDC<br />must be configured.</p>
</td>
</tr>
<tr>
<td>
<code>frontendConfigMapRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FrontendConfigMapRef is the reference to a ConfigMap in the garden namespace containing the frontend<br />configuration.</p>
</td>
</tr>
<tr>
<td>
<code>assetsConfigMapRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
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
<a href="#dashboardgithub">DashboardGitHub</a>
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
<p>LogLevel is the configured log level. Must be one of [trace,debug,info,warn,error].<br />Defaults to info.</p>
</td>
</tr>
<tr>
<td>
<code>oidcConfig</code></br>
<em>
<a href="#dashboardoidc">DashboardOIDC</a>
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
<a href="#dashboardterminal">DashboardTerminal</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Terminal contains configuration for the terminal settings.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
<a href="#dashboardingress">DashboardIngress</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ingress contains configuration for the ingress settings.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenerdiscoveryserverconfig">GardenerDiscoveryServerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerDiscoveryServerConfig contains configuration settings for the gardener-discovery-server.
</p>

<table>
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
<a href="#dnsdomain">DNSDomain</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Domain overrides the default ingress domain and optionally the DNS provider for the gardener-discovery-server.<br />This field is optional, but once the gardener-discovery-server is enabled, its domain cannot be changed anymore.<br />Defaults to "discovery.<first-runtime-ingress-domain>".</p>
</td>
</tr>
<tr>
<td>
<code>tlsSecretName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>TLSSecretName is the name of a secret (in the garden namespace) containing<br />a trusted TLS certificate for the domain. If not configured, Gardener falls<br />back to a secret labelled with 'gardener.cloud/role=garden-cert', if in turn not<br />configured it generates a self-signed certificate.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenerresourcemanagerconfig">GardenerResourceManagerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerResourceManagerConfig contains configuration settings for the gardener-resource-manager.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>additionalTargetNamespaces</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalTargetNamespaces allows specifying custom target namespaces for the gardener-resource-manager instance.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="gardenerschedulerconfig">GardenerSchedulerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#gardener">Gardener</a>)
</p>

<p>
GardenerSchedulerConfig contains configuration settings for the gardener-scheduler.
</p>

<table>
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
<code>logLevel</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LogLevel is the configured log level for the gardener-scheduler. Must be one of [info,debug,error].<br />Defaults to info.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="groupresource">GroupResource
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
GroupResource contains a list of resources which should be stored in etcd-events instead of etcd-main.
</p>

<table>
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


<h3 id="highavailability">HighAvailability
</h3>


<p>
(<em>Appears on:</em><a href="#controlplane">ControlPlane</a>)
</p>

<p>
HighAvailability specifies the configuration settings for high availability for a resource.
</p>


<h3 id="ingress">Ingress
</h3>


<p>
(<em>Appears on:</em><a href="#runtimecluster">RuntimeCluster</a>)
</p>

<p>
Ingress configures the Ingress specific settings of the runtime cluster.
</p>

<table>
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
<a href="#dnsdomain">DNSDomain</a> array
</em>
</td>
<td>
<p>Domains specify the ingress domains of the cluster pointing to the ingress controller endpoint. They will be used<br />to construct ingress URLs for system applications running in runtime cluster.</p>
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
<p>Controller configures a Gardener managed Ingress Controller listening on the ingressDomain.</p>
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
<code>auditWebhook</code></br>
<em>
<a href="#auditwebhook">AuditWebhook</a>
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
<a href="#authentication">Authentication</a>
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
<a href="#groupresource">GroupResource</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourcesToStoreInETCDEvents contains a list of resources which should be stored in etcd-events instead of<br />etcd-main. The 'events' resource is always stored in etcd-events. Note that adding or removing resources from<br />this list will not migrate them automatically from the etcd-main to etcd-events or vice versa.</p>
</td>
</tr>
<tr>
<td>
<code>sni</code></br>
<em>
<a href="#sni">SNI</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNI contains configuration options for the TLS SNI settings.</p>
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
<code>certificateSigningDuration</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertificateSigningDuration is the maximum length of duration signed certificates will be given. Individual CSRs<br />may request shorter certs by setting `spec.expirationSeconds`.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubernetes">Kubernetes
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
Kubernetes contains the version and configuration options for the Kubernetes components of the virtual garden
cluster.
</p>

<table>
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


<h3 id="maintenance">Maintenance
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
Maintenance contains information about the time window for maintenance operations.
</p>

<table>
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
<a href="#maintenancetimewindow">MaintenanceTimeWindow</a>
</em>
</td>
<td>
<p>TimeWindow contains information about the time window for maintenance operations.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networking">Networking
</h3>


<p>
(<em>Appears on:</em><a href="#virtualcluster">VirtualCluster</a>)
</p>

<p>
Networking defines networking parameters for the virtual garden cluster.
</p>

<table>
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
string array
</em>
</td>
<td>
<p>Services are the CIDRs of the service network. Elements can be appended to this list, but not removed.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="projectquotaconfiguration">ProjectQuotaConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#gardenercontrollermanagerconfig">GardenerControllerManagerConfig</a>)
</p>

<p>
ProjectQuotaConfiguration defines quota configurations.
</p>

<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcequota-v1-core">ResourceQuota</a>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProjectSelector is an optional setting to select the projects considered for quotas.<br />Defaults to empty LabelSelector, which matches all projects.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="provider">Provider
</h3>


<p>
(<em>Appears on:</em><a href="#runtimecluster">RuntimeCluster</a>)
</p>

<p>
Provider defines the provider-specific information for this cluster.
</p>

<table>
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
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones is the list of availability zones the cluster is deployed to.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="resourceadmissionconfiguration">ResourceAdmissionConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#gardeneradmissioncontrollerconfig">GardenerAdmissionControllerConfig</a>)
</p>

<p>
ResourceAdmissionConfiguration contains settings about arbitrary kinds and the size each resource should have at most.
</p>

<table>
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
<a href="#resourcelimit">ResourceLimit</a> array
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#subject-v1-rbac">Subject</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>UnrestrictedSubjects contains references to users, groups, or service accounts which aren't subjected to any resource size limit.</p>
</td>
</tr>
<tr>
<td>
<code>operationMode</code></br>
<em>
<a href="#resourceadmissionwebhookmode">ResourceAdmissionWebhookMode</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OperationMode specifies the mode the webhooks operates in. Allowed values are "block" and "log". Defaults to "block".</p>
</td>
</tr>

</tbody>
</table>


<h3 id="resourceadmissionwebhookmode">ResourceAdmissionWebhookMode
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#resourceadmissionconfiguration">ResourceAdmissionConfiguration</a>)
</p>

<p>
ResourceAdmissionWebhookMode is an alias type for the resource admission webhook mode.
</p>


<h3 id="resourcelimit">ResourceLimit
</h3>


<p>
(<em>Appears on:</em><a href="#resourceadmissionconfiguration">ResourceAdmissionConfiguration</a>)
</p>

<p>
ResourceLimit contains settings about a kind and the size each resource should have at most.
</p>

<table>
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
string array
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
string array
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
string array
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Size specifies the imposed limit.</p>
</td>
</tr>
<tr>
<td>
<code>count</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Count specifies the maximum number of resources of the given kind. Only cluster-scoped resources are considered.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="runtimecluster">RuntimeCluster
</h3>


<p>
(<em>Appears on:</em><a href="#gardenspec">GardenSpec</a>)
</p>

<p>
RuntimeCluster contains configuration for the runtime cluster.
</p>

<table>
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
<a href="#ingress">Ingress</a>
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
<a href="#runtimenetworking">RuntimeNetworking</a>
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
<a href="#provider">Provider</a>
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
<a href="#settings">Settings</a>
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
<a href="#volume">Volume</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains settings for persistent volumes created in the runtime cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="runtimenetworking">RuntimeNetworking
</h3>


<p>
(<em>Appears on:</em><a href="#runtimecluster">RuntimeCluster</a>)
</p>

<p>
RuntimeNetworking defines the networking configuration of the runtime cluster.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>ipFamilies</code></br>
<em>
IPFamily array
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPFamilies specifies the IP protocol versions to use for the runtime cluster's networking. This field is<br />immutable.<br />Defaults to ["IPv4"].</p>
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
<p>Nodes are the CIDRs of the node network. Elements can be appended to this list, but not removed.</p>
</td>
</tr>
<tr>
<td>
<code>pods</code></br>
<em>
string array
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
string array
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
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>BlockCIDRs is a list of network addresses that should be blocked.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="sni">SNI
</h3>


<p>
(<em>Appears on:</em><a href="#kubeapiserverconfig">KubeAPIServerConfig</a>)
</p>

<p>
SNI contains configuration options for the TLS SNI settings.
</p>

<table>
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
<em>(Optional)</em>
<p>SecretName is the name of a secret containing the TLS certificate and private key.<br />If not configured, Gardener falls back to a secret labelled with 'gardener.cloud/role=garden-cert'.</p>
</td>
</tr>
<tr>
<td>
<code>domainPatterns</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DomainPatterns is a list of fully qualified domain names, possibly with prefixed wildcard segments. The domain<br />patterns also allow IP addresses, but IPs should only be used if the apiserver has visibility to the IP address<br />requested by a client. If no domain patterns are provided, the names of the certificate are extracted.<br />Non-wildcard matches trump over wildcard matches, explicit domain patterns trump over extracted names.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="settingloadbalancerservices">SettingLoadBalancerServices
</h3>


<p>
(<em>Appears on:</em><a href="#settings">Settings</a>)
</p>

<p>
SettingLoadBalancerServices controls certain settings for services of type load balancer that are created in the
runtime cluster.
</p>

<table>
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

</tbody>
</table>


<h3 id="settingtopologyawarerouting">SettingTopologyAwareRouting
</h3>


<p>
(<em>Appears on:</em><a href="#settings">Settings</a>)
</p>

<p>
SettingTopologyAwareRouting controls certain settings for topology-aware traffic routing in the cluster.
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
<p>Enabled controls whether certain Services deployed in the cluster should be topology-aware.<br />These Services are virtual-garden-etcd-main-client, virtual-garden-etcd-events-client and virtual-garden-kube-apiserver.<br />Additionally, other components that are deployed to the runtime cluster via other means can read this field and<br />according to its value enable/disable topology-aware routing for their Services.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="settingverticalpodautoscaler">SettingVerticalPodAutoscaler
</h3>


<p>
(<em>Appears on:</em><a href="#settings">Settings</a>)
</p>

<p>
SettingVerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the
cluster.
</p>

<table>
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
<p>Enabled controls whether the VPA components shall be deployed into this cluster. It is true by default because<br />the operator (and Gardener) heavily rely on a VPA being deployed. You should only disable this if your runtime<br />cluster already has another, manually/custom managed VPA deployment. If this is not the case, but you still<br />disable it, then reconciliation will fail.</p>
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


<h3 id="settings">Settings
</h3>


<p>
(<em>Appears on:</em><a href="#runtimecluster">RuntimeCluster</a>)
</p>

<p>
Settings contains certain settings for this cluster.
</p>

<table>
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
<a href="#settingloadbalancerservices">SettingLoadBalancerServices</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LoadBalancerServices controls certain settings for services of type load balancer that are created in the runtime<br />cluster.</p>
</td>
</tr>
<tr>
<td>
<code>verticalPodAutoscaler</code></br>
<em>
<a href="#settingverticalpodautoscaler">SettingVerticalPodAutoscaler</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VerticalPodAutoscaler controls certain settings for the vertical pod autoscaler components deployed in the<br />cluster.</p>
</td>
</tr>
<tr>
<td>
<code>topologyAwareRouting</code></br>
<em>
<a href="#settingtopologyawarerouting">SettingTopologyAwareRouting</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>TopologyAwareRouting controls certain settings for topology-aware traffic routing in the cluster.<br />See https://github.com/gardener/gardener/blob/master/docs/operations/topology_aware_routing.md.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="storage">Storage
</h3>


<p>
(<em>Appears on:</em><a href="#etcdevents">ETCDEvents</a>, <a href="#etcdmain">ETCDMain</a>)
</p>

<p>
Storage contains storage configuration.
</p>

<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api">Quantity</a>
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


<h3 id="virtualcluster">VirtualCluster
</h3>


<p>
(<em>Appears on:</em><a href="#gardenspec">GardenSpec</a>)
</p>

<p>
VirtualCluster contains configuration for the virtual cluster.
</p>

<table>
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
<a href="#controlplane">ControlPlane</a>
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
<a href="#dns">DNS</a>
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
<a href="#etcd">ETCD</a>
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
<a href="#gardener">Gardener</a>
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
<a href="#kubernetes">Kubernetes</a>
</em>
</td>
<td>
<p>Kubernetes contains the version and configuration options for the Kubernetes components of the virtual garden<br />cluster.</p>
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
<p>Maintenance contains information about the time window for maintenance operations.</p>
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
<p>Networking contains information about cluster networking such as CIDRs, etc.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="volume">Volume
</h3>


<p>
(<em>Appears on:</em><a href="#runtimecluster">RuntimeCluster</a>)
</p>

<p>
Volume contains settings for persistent volumes created in the runtime cluster.
</p>

<table>
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
<p>MinimumSize defines the minimum size that should be used for PVCs in the runtime cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workloadidentitykeyrotation">WorkloadIdentityKeyRotation
</h3>


<p>
(<em>Appears on:</em><a href="#credentialsrotation">CredentialsRotation</a>)
</p>

<p>
WorkloadIdentityKeyRotation contains information about the workload identity key credential rotation.
</p>

<table>
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
<p>Phase describes the phase of the workload identity key credential rotation.</p>
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
<p>LastCompletionTime is the most recent time when the workload identity key credential rotation was successfully<br />completed.</p>
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
<p>LastInitiationTime is the most recent time when the workload identity key credential rotation was initiated.</p>
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
<p>LastInitiationFinishedTime is the recent time when the workload identity key credential rotation initiation was<br />completed.</p>
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
<p>LastCompletionTriggeredTime is the recent time when the workload identity key credential rotation completion was<br />triggered.</p>
</td>
</tr>

</tbody>
</table>


