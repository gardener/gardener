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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
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
<p>Gardener holds information about the Gardener which last acted on the Shoot.</p>
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
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
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
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
