<p>Packages:</p>
<ul>
<li>
<a href="#local.provider.extensions.gardener.cloud%2fv1alpha1">local.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="local.provider.extensions.gardener.cloud/v1alpha1">local.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>

</p>

<h3 id="cloudprofileconfig">CloudProfileConfig
</h3>


<p>
CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
resource.
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
<code>machineImages</code></br>
<em>
<a href="#machineimages">MachineImages</a> array
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps<br />logical names and versions to provider-specific identifiers.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancer</code></br>
<em>
<a href="#loadbalancer">LoadBalancer</a>
</em>
</td>
<td>
<p>LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudproviderconfig">CloudProviderConfig
</h3>


<p>
CloudProviderConfig contains the configuration API for cloud-controller-manager-local (used by the
pkg/provider-local/cloud-provider package).
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
<a href="#runtimecluster">RuntimeCluster</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RuntimeCluster configures how cloud-controller-manager-local connects to the runtime cluster (seed) of the shoot<br />cluster, i.e., the kind cluster where the shoot machine pods run.<br />This is only required if the cloud-controller-manager-local is running for a shoot cluster, not for the kind<br />cluster itself.</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancer</code></br>
<em>
<a href="#loadbalancer">LoadBalancer</a>
</em>
</td>
<td>
<p>LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="loadbalancer">LoadBalancer
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>, <a href="#cloudproviderconfig">CloudProviderConfig</a>)
</p>

<p>
LoadBalancer contains the configuration for the service controller of cloud-controller-manager-local.
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
<p>Image is the envoy container image used for starting load balancer containers.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimage">MachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
MachineImage is a mapping from logical names and versions to provider-specific machine image data.
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
<p>Name is the logical name of the machine image.</p>
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
<p>Version is the logical version of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the image for the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities of the machine image.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageflavor">MachineImageFlavor
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
MachineImageFlavor is a provider-specific image identifier with its supported capabilities.
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
<p>Image is the image for the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities that are supported by the identifier in this set.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageversion">MachineImageVersion
</h3>


<p>
(<em>Appears on:</em><a href="#machineimages">MachineImages</a>)
</p>

<p>
MachineImageVersion contains a version and a provider-specific identifier.
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version of the image.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Image is the image for the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>capabilityFlavors</code></br>
<em>
<a href="#machineimageflavor">MachineImageFlavor</a> array
</em>
</td>
<td>
<p>CapabilityFlavors contains provider-specific image identifiers of this version with their capabilities.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimages">MachineImages
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
MachineImages is a mapping from logical names and versions to provider-specific identifiers.
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
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#machineimageversion">MachineImageVersion</a> array
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="runtimecluster">RuntimeCluster
</h3>


<p>
(<em>Appears on:</em><a href="#cloudproviderconfig">CloudProviderConfig</a>)
</p>

<p>
RuntimeCluster configures how cloud-controller-manager-local connects to the runtime cluster (seed) of the shoot
cluster, i.e., the kind cluster where the shoot machine pods run.
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
<code>namespace</code></br>
<em>
string
</em>
</td>
<td>
<p>Namespace configures the namespace of the runtime cluster where the shoot machine pods run.<br />If RuntimeCluster is set, this field is required.</p>
</td>
</tr>
<tr>
<td>
<code>kubeconfig</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kubeconfig configures the path to the kubeconfig file for connecting to the runtime cluster. If not set,<br />cloud-controller-manager-local uses the in-cluster credentials (ServiceAccount).</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerstatus">WorkerStatus
</h3>


<p>
WorkerStatus contains information about created worker resources.
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
<code>machineImages</code></br>
<em>
<a href="#machineimage">MachineImage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller<br />gets the mapping from name/version to the provider-specific machine image data from the CloudProfile. However, if<br />a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`<br />resources that are still using this version. Hence, it stores the used versions in the provider status to ensure<br />reconciliation is possible.</p>
</td>
</tr>

</tbody>
</table>


