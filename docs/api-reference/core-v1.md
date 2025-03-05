<p>Packages:</p>
<ul>
<li>
<a href="#core.gardener.cloud%2fv1">core.gardener.cloud/v1</a>
</li>
</ul>
<h2 id="core.gardener.cloud/v1">core.gardener.cloud/v1</h2>
<p>
<p>Package v1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#core.gardener.cloud/v1.ControllerDeployment">ControllerDeployment</a>
</li></ul>
<h3 id="core.gardener.cloud/v1.ControllerDeployment">ControllerDeployment
</h3>
<p>
<p>ControllerDeployment contains information about how this controller is deployed.</p>
</p>
<table>
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
core.gardener.cloud/v1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControllerDeployment</code></td>
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
<p>Standard object metadata.</p>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>helm</code></br>
<em>
<a href="#core.gardener.cloud/v1.HelmControllerDeployment">
HelmControllerDeployment
</a>
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
<h3 id="core.gardener.cloud/v1.HelmControllerDeployment">HelmControllerDeployment
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1.ControllerDeployment">ControllerDeployment</a>)
</p>
<p>
<p>HelmControllerDeployment configures how an extension controller is deployed using helm.</p>
</p>
<table>
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
[]byte
</em>
</td>
<td>
<em>(Optional)</em>
<p>RawChart is the base64-encoded, gzip&rsquo;ed, tar&rsquo;ed extension controller chart.</p>
</td>
</tr>
<tr>
<td>
<code>values</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#json-v1-apiextensions-k8s-io">
Kubernetes apiextensions/v1.JSON
</a>
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
<a href="#core.gardener.cloud/v1.OCIRepository">
OCIRepository
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>OCIRepository defines where to pull the chart.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="core.gardener.cloud/v1.OCIRepository">OCIRepository
</h3>
<p>
(<em>Appears on:</em>
<a href="#core.gardener.cloud/v1.HelmControllerDeployment">HelmControllerDeployment</a>)
</p>
<p>
<p>OCIRepository configures where to pull an OCI Artifact, that could contain for example a Helm Chart.</p>
</p>
<table>
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
<p>Digest of the image to pull, takes precedence over tag.
The value should be in the format &lsquo;sha256:<HASH>&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
