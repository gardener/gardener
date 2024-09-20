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
Accepted resources are core/v1.Secret and security.gardener.cloud/v1alpha1.WorkloadIdentity
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
<h3 id="security.gardener.cloud/v1alpha1.ContextObject">ContextObject
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.TokenRequestSpec">TokenRequestSpec</a>)
</p>
<p>
<p>ContextObject identifies the object the token is requested for.</p>
</p>
<table>
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
<p>Kind of the object the token is requested for. Valid kinds are &lsquo;Shoot&rsquo;, &lsquo;Seed&rsquo;, etc.</p>
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
<p>API version of the object the token is requested for.</p>
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
<p>Name of the object the token is requested for.</p>
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
<p>Namespace of the object the token is requested for.</p>
</td>
</tr>
<tr>
<td>
<code>uid</code></br>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
<p>UID of the object the token is requested for.</p>
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
<h3 id="security.gardener.cloud/v1alpha1.TokenRequest">TokenRequest
</h3>
<p>
<p>TokenRequest is a resource that is used to request WorkloadIdentity tokens.</p>
</p>
<table>
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
<a href="#security.gardener.cloud/v1alpha1.TokenRequestSpec">
TokenRequestSpec
</a>
</em>
</td>
<td>
<p>Spec holds configuration settings for the requested token.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>contextObject</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.ContextObject">
ContextObject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContextObject identifies the object the token is requested for.</p>
</td>
</tr>
<tr>
<td>
<code>expirationSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationSeconds specifies for how long the requested token should be valid.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.TokenRequestStatus">
TokenRequestStatus
</a>
</em>
</td>
<td>
<p>Status bears the issued token with additional information back to the client.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="security.gardener.cloud/v1alpha1.TokenRequestSpec">TokenRequestSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.TokenRequest">TokenRequest</a>)
</p>
<p>
<p>TokenRequestSpec holds configuration settings for the requested token.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>contextObject</code></br>
<em>
<a href="#security.gardener.cloud/v1alpha1.ContextObject">
ContextObject
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContextObject identifies the object the token is requested for.</p>
</td>
</tr>
<tr>
<td>
<code>expirationSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationSeconds specifies for how long the requested token should be valid.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="security.gardener.cloud/v1alpha1.TokenRequestStatus">TokenRequestStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#security.gardener.cloud/v1alpha1.TokenRequest">TokenRequest</a>)
</p>
<p>
<p>TokenRequestStatus bears the issued token with additional information back to the client.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>token</code></br>
<em>
string
</em>
</td>
<td>
<p>Token is the issued token.</p>
</td>
</tr>
<tr>
<td>
<code>expirationTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>ExpirationTimestamp is the time of expiration of the returned token.</p>
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
<tr>
<td>
<code>issuer</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Issuer is the issuer URL of the ID token.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
