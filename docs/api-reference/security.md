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
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
