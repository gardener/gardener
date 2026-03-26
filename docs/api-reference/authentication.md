<p>Packages:</p>
<ul>
<li>
<a href="#authentication.gardener.cloud%2fv1alpha1">authentication.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="authentication.gardener.cloud/v1alpha1">authentication.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#adminkubeconfigrequest">AdminKubeconfigRequest</a>
</li>
<li>
<a href="#viewerkubeconfigrequest">ViewerKubeconfigRequest</a>
</li>
</ul>

<h3 id="adminkubeconfigrequest">AdminKubeconfigRequest
</h3>


<p>
AdminKubeconfigRequest can be used to request a kubeconfig with admin credentials
for a Shoot cluster.
</p>

<table>
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
<a href="#adminkubeconfigrequestspec">AdminKubeconfigRequestSpec</a>
</em>
</td>
<td>
<p>Spec is the specification of the AdminKubeconfigRequest.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#adminkubeconfigrequeststatus">AdminKubeconfigRequestStatus</a>
</em>
</td>
<td>
<p>Status is the status of the AdminKubeconfigRequest.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="adminkubeconfigrequestspec">AdminKubeconfigRequestSpec
</h3>


<p>
(<em>Appears on:</em><a href="#adminkubeconfigrequest">AdminKubeconfigRequest</a>)
</p>

<p>
AdminKubeconfigRequestSpec contains the expiration time of the kubeconfig.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>expirationSeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationSeconds is the requested validity duration of the credential. The<br />credential issuer may return a credential with a different validity duration so a<br />client needs to check the 'expirationTimestamp' field in a response.<br />Defaults to 1 hour.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="adminkubeconfigrequeststatus">AdminKubeconfigRequestStatus
</h3>


<p>
(<em>Appears on:</em><a href="#adminkubeconfigrequest">AdminKubeconfigRequest</a>)
</p>

<p>
AdminKubeconfigRequestStatus is the status of the AdminKubeconfigRequest containing
the kubeconfig and expiration of the credential.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>kubeconfig</code></br>
<em>
integer array
</em>
</td>
<td>
<p>Kubeconfig contains the kubeconfig with cluster-admin privileges for the shoot cluster.</p>
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
<p>ExpirationTimestamp is the expiration timestamp of the returned credential.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="viewerkubeconfigrequest">ViewerKubeconfigRequest
</h3>


<p>
ViewerKubeconfigRequest can be used to request a kubeconfig with viewer credentials (excluding Secrets)
for a Shoot cluster.
</p>

<table>
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
<a href="#viewerkubeconfigrequestspec">ViewerKubeconfigRequestSpec</a>
</em>
</td>
<td>
<p>Spec is the specification of the ViewerKubeconfigRequest.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#viewerkubeconfigrequeststatus">ViewerKubeconfigRequestStatus</a>
</em>
</td>
<td>
<p>Status is the status of the ViewerKubeconfigRequest.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="viewerkubeconfigrequestspec">ViewerKubeconfigRequestSpec
</h3>


<p>
(<em>Appears on:</em><a href="#viewerkubeconfigrequest">ViewerKubeconfigRequest</a>)
</p>

<p>
ViewerKubeconfigRequestSpec contains the expiration time of the kubeconfig.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>expirationSeconds</code></br>
<em>
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExpirationSeconds is the requested validity duration of the credential. The<br />credential issuer may return a credential with a different validity duration so a<br />client needs to check the 'expirationTimestamp' field in a response.<br />Defaults to 1 hour.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="viewerkubeconfigrequeststatus">ViewerKubeconfigRequestStatus
</h3>


<p>
(<em>Appears on:</em><a href="#viewerkubeconfigrequest">ViewerKubeconfigRequest</a>)
</p>

<p>
ViewerKubeconfigRequestStatus is the status of the ViewerKubeconfigRequest containing
the kubeconfig and expiration of the credential.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>kubeconfig</code></br>
<em>
integer array
</em>
</td>
<td>
<p>Kubeconfig contains the kubeconfig with viewer privileges (excluding Secrets) for the shoot cluster.</p>
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
<p>ExpirationTimestamp is the expiration timestamp of the returned credential.</p>
</td>
</tr>

</tbody>
</table>


