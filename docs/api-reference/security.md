<p>Packages:</p>
<ul>
<li>
<a href="#security.gardener.cloud%2fv1alpha1">security.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="security.gardener.cloud/v1alpha1">security.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#credentialsbinding">CredentialsBinding</a>
</li>
<li>
<a href="#tokenrequest">TokenRequest</a>
</li>
<li>
<a href="#workloadidentity">WorkloadIdentity</a>
</li>
</ul>

<h3 id="contextobject">ContextObject
</h3>


<p>
(<em>Appears on:</em><a href="#tokenrequestspec">TokenRequestSpec</a>)
</p>

<p>
ContextObject identifies the object the token is requested for.
</p>

<table>
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
<p>Kind of the object the token is requested for. Valid kinds are 'Shoot', 'Seed', etc.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#uid-types-pkg">UID</a>
</em>
</td>
<td>
<p>UID of the object the token is requested for.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="credentialsbinding">CredentialsBinding
</h3>


<p>
CredentialsBinding represents a binding to credentials in the same or another namespace.
</p>

<table>
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
<code>provider</code></br>
<em>
<a href="#credentialsbindingprovider">CredentialsBindingProvider</a>
</em>
</td>
<td>
<p>Provider defines the provider type of the CredentialsBinding.<br />This field is immutable.</p>
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
<p>CredentialsRef is a reference to a resource holding the credentials.<br />Accepted resources are core/v1.Secret, core.gardener.cloud/v1beta1.InternalSecret, and<br />security.gardener.cloud/v1alpha1.WorkloadIdentity.<br />This field is immutable.</p>
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

</tbody>
</table>


<h3 id="credentialsbindingprovider">CredentialsBindingProvider
</h3>


<p>
(<em>Appears on:</em><a href="#credentialsbinding">CredentialsBinding</a>)
</p>

<p>
CredentialsBindingProvider defines the provider type of the CredentialsBinding.
</p>

<table>
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


<h3 id="targetsystem">TargetSystem
</h3>


<p>
(<em>Appears on:</em><a href="#workloadidentityspec">WorkloadIdentitySpec</a>)
</p>

<p>
TargetSystem represents specific configurations for the system that will accept the JWTs.
</p>

<table>
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


<h3 id="tokenrequest">TokenRequest
</h3>


<p>
TokenRequest is a resource that is used to request WorkloadIdentity tokens.
</p>

<table>
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
<a href="#tokenrequestspec">TokenRequestSpec</a>
</em>
</td>
<td>
<p>Spec holds configuration settings for the requested token.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#tokenrequeststatus">TokenRequestStatus</a>
</em>
</td>
<td>
<p>Status bears the issued token with additional information back to the client.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="tokenrequestspec">TokenRequestSpec
</h3>


<p>
(<em>Appears on:</em><a href="#tokenrequest">TokenRequest</a>)
</p>

<p>
TokenRequestSpec holds configuration settings for the requested token.
</p>

<table>
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
<a href="#contextobject">ContextObject</a>
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationSeconds specifies for how long the requested token should be valid.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="tokenrequeststatus">TokenRequestStatus
</h3>


<p>
(<em>Appears on:</em><a href="#tokenrequest">TokenRequest</a>)
</p>

<p>
TokenRequestStatus bears the issued token with additional information back to the client.
</p>

<table>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#time-v1-meta">Time</a>
</em>
</td>
<td>
<p>ExpirationTimestamp is the time of expiration of the returned token.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workloadidentity">WorkloadIdentity
</h3>


<p>
WorkloadIdentity is resource that allows workloads to be presented before external systems
by giving them identities managed by the Gardener API server.
The identity of such workload is represented by JSON Web Token issued by the Gardener API server.
Workload identities are designed to be used by components running in the Gardener environment,
seed or runtime cluster, that make use of identity federation inspired by the OIDC protocol.
</p>

<table>
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
<a href="#workloadidentityspec">WorkloadIdentitySpec</a>
</em>
</td>
<td>
<p>Spec configures the JSON Web Token issued by the Gardener API server.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#workloadidentitystatus">WorkloadIdentityStatus</a>
</em>
</td>
<td>
<p>Status contain the latest observed status of the WorkloadIdentity.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workloadidentityspec">WorkloadIdentitySpec
</h3>


<p>
(<em>Appears on:</em><a href="#workloadidentity">WorkloadIdentity</a>)
</p>

<p>
WorkloadIdentitySpec configures the JSON Web Token issued by the Gardener API server.
</p>

<table>
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
string array
</em>
</td>
<td>
<p>Audiences specify the list of recipients that the JWT is intended for.<br />The values of this field will be set in the 'aud' claim.</p>
</td>
</tr>
<tr>
<td>
<code>targetSystem</code></br>
<em>
<a href="#targetsystem">TargetSystem</a>
</em>
</td>
<td>
<p>TargetSystem represents specific configurations for the system that will accept the JWTs.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workloadidentitystatus">WorkloadIdentityStatus
</h3>


<p>
(<em>Appears on:</em><a href="#workloadidentity">WorkloadIdentity</a>)
</p>

<p>
WorkloadIdentityStatus contain the latest observed status of the WorkloadIdentity.
</p>

<table>
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
<p>Sub contains the computed value of the subject that is going to be set in JWTs 'sub' claim.</p>
</td>
</tr>

</tbody>
</table>


