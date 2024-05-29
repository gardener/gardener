<p>Packages:</p>
<ul>
<li>
<a href="#security.gardener.cloud%2fv1alpha1">security.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="security.gardener.cloud/v1alpha1">security.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#security.gardener.cloud/v1alpha1.CredentialsBinding">CredentialsBinding</a>
</li><li>
<a href="#security.gardener.cloud/v1alpha1.WorkloadIdentity">WorkloadIdentity</a>
</li></ul>
<h3 id="security.gardener.cloud/v1alpha1.CredentialsBinding">CredentialsBinding
</h3>
<p>
<p>CredentialsBinding represents a binding to credentials in the same or another namespace.</p>
</p>
<table>
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
security.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>CredentialsBinding</code></td>
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
<code>provider</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.CredentialsBindingProvider">
CredentialsBindingProvider
</a>
</em>
</td>
<td>
<p>Provider defines the provider type of the CredentialsBinding.
This field is immutable.</p>
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
<p>CredentialsRef is a reference to a resource holding the credentials.
Accepted resources are core/v1.Secret and security.gardener.cloud/v1alpha1.WorkloadIdentity</p>
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
</tbody>
</table>
<h3 id="security.gardener.cloud/v1alpha1.WorkloadIdentity">WorkloadIdentity
</h3>
<p>
<p>WorkloadIdentity is resource that allows workloads to be presented before external systems
by giving them identities managed by the Gardener API server.
The identity of such workload is represented by JSON Web Token issued by the Gardener API server.
Workload identities are designed to be used by components running in the Gardener environment,
seed or runtime cluster, that make use of identity federation inspired by the OIDC protocol.</p>
</p>
<table>
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
security.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkloadIdentity</code></td>
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
<a href="#security.gardener.cloud/v1alpha1.WorkloadIdentitySpec">
WorkloadIdentitySpec
</a>
</em>
</td>
<td>
<p>Spec configures the JSON Web Token issued by the Gardener API server.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>audiences</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Audiences specify the list of recipients that the JWT is intended for.
The values of this field will be set in the &lsquo;aud&rsquo; claim.</p>
</td>
</tr>
<tr>
<td>
<code>targetSystem</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.TargetSystem">
TargetSystem
</a>
</em>
</td>
<td>
<p>TargetSystem represents specific configurations for the system that will accept the JWTs.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.WorkloadIdentityStatus">
WorkloadIdentityStatus
</a>
</em>
</td>
<td>
<p>Status contain the latest observed status of the WorkloadIdentity.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="security.gardener.cloud/v1alpha1.CredentialsBindingProvider">CredentialsBindingProvider
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.CredentialsBinding">CredentialsBinding</a>)
</p>
<p>
<p>CredentialsBindingProvider defines the provider type of the CredentialsBinding.</p>
</p>
<table>
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
</td>
</tr>
</tbody>
</table>
<h3 id="security.gardener.cloud/v1alpha1.TargetSystem">TargetSystem
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.WorkloadIdentitySpec">WorkloadIdentitySpec</a>)
</p>
<p>
<p>TargetSystem represents specific configurations for the system that will accept the JWTs.</p>
</p>
<table>
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
<p>Type is the type of the target system.</p>
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
<h3 id="security.gardener.cloud/v1alpha1.WorkloadIdentitySpec">WorkloadIdentitySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.WorkloadIdentity">WorkloadIdentity</a>)
</p>
<p>
<p>WorkloadIdentitySpec configures the JSON Web Token issued by the Gardener API server.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>audiences</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Audiences specify the list of recipients that the JWT is intended for.
The values of this field will be set in the &lsquo;aud&rsquo; claim.</p>
</td>
</tr>
<tr>
<td>
<code>targetSystem</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.TargetSystem">
TargetSystem
</a>
</em>
</td>
<td>
<p>TargetSystem represents specific configurations for the system that will accept the JWTs.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="security.gardener.cloud/v1alpha1.WorkloadIdentityStatus">WorkloadIdentityStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.WorkloadIdentity">WorkloadIdentity</a>)
</p>
<p>
<p>WorkloadIdentityStatus contain the latest observed status of the WorkloadIdentity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sub</code></br>
<em>
string
</em>
</td>
<td>
<p>Sub contains the computed value of the subject that is going to be set in JWTs &lsquo;sub&rsquo; claim.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
