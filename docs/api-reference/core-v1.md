<p>Packages:</p>
<ul>
<li>
<a href="#core.gardener.cloud%2fv1">core.gardener.cloud/v1</a>
</li>
</ul>

<h2 id="core.gardener.cloud/v1">core.gardener.cloud/v1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#controllerdeployment">ControllerDeployment</a>
</li>
</ul>

<h3 id="controllerdeployment">ControllerDeployment
</h3>


<p>
ControllerDeployment contains information about how this controller is deployed.
</p>

<table>
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
<code>helm</code></br>
<em>
<a href="#helmcontrollerdeployment">HelmControllerDeployment</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Helm configures that an extension controller is deployed using helm.</p>
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


<h3 id="helmcontrollerdeployment">HelmControllerDeployment
</h3>


<p>
(<em>Appears on:</em><a href="#controllerdeployment">ControllerDeployment</a>)
</p>

<p>
HelmControllerDeployment configures how an extension controller is deployed using helm.
</p>

<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>

<tr>
<td>
<code>rawChart</code></br>
<em>
integer array
</em>
</td>
<td>
<em>(Optional)</em>
<p>RawChart is the base64-encoded, gzip'ed, tar'ed extension controller chart.</p>
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
<p>Values are the chart values.</p>
</td>
</tr>
<tr>
<td>
<code>ociRepository</code></br>
<em>
<a href="#ocirepository">OCIRepository</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OCIRepository defines where to pull the chart.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="ocirepository">OCIRepository
</h3>


<p>
(<em>Appears on:</em><a href="#helmcontrollerdeployment">HelmControllerDeployment</a>)
</p>

<p>
OCIRepository configures where to pull an OCI Artifact, that could contain for example a Helm Chart.
</p>

<table>
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
<p>Digest of the image to pull, takes precedence over tag.<br />The value should be in the format 'sha256:<HASH>'.</p>
</td>
</tr>
<tr>
<td>
<code>pullSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PullSecretRef is a reference to a secret containing the pull secret.<br />The secret must be of type `kubernetes.io/dockerconfigjson` and must be located in the `garden` namespace.<br />For usage in the gardenlet, the secret must have the label `gardener.cloud/role=helm-pull-secret`.</p>
</td>
</tr>
<tr>
<td>
<code>caBundleSecretRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CABundleSecretRef is a reference to a secret containing a PEM-encoded certificate authority bundle.<br />The CA bundle is used to verify the TLS certificate of the OCI registry.<br />The secret must have a data key `bundle.crt` and must be located in the `garden` namespace.<br />For usage in the gardenlet, the secret must have the label `gardener.cloud/role=oci-ca-bundle`.<br />If not provided, the system's default certificate pool is used.</p>
</td>
</tr>

</tbody>
</table>


