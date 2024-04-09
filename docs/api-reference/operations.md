<p>Packages:</p>
<ul>
<li>
<a href="#operations.gardener.cloud%2fv1alpha1">operations.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="operations.gardener.cloud/v1alpha1">operations.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#operations.gardener.cloud/v1alpha1.Bastion">Bastion</a>
</li></ul>
<h3 id="operations.gardener.cloud/v1alpha1.Bastion">Bastion
</h3>
<p>
<p>Bastion holds details about an SSH bastion for a shoot cluster.</p>
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
operations.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Bastion</code></td>
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
<code>spec</code></br>
<em>
<a href="#operations.gardener.cloud/v1alpha1.BastionSpec">
BastionSpec
</a>
</em>
</td>
<td>
<p>Specification of the Bastion.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>shootRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ShootRef defines the target shoot for a Bastion. The name field of the ShootRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the seed to which this Bastion is currently scheduled. This field is populated
at the beginning of a create/reconcile operation.</p>
</td>
</tr>
<tr>
<td>
<code>providerType</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderType is cloud provider used by the referenced Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>sshPublicKey</code></br>
<em>
string
</em>
</td>
<td>
<p>SSHPublicKey is the user&rsquo;s public key. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
<a href="#operations.gardener.cloud/v1alpha1.BastionIngressPolicy">
[]BastionIngressPolicy
</a>
</em>
</td>
<td>
<p>Ingress controls from where the created bastion host should be reachable.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#operations.gardener.cloud/v1alpha1.BastionStatus">
BastionStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Most recently observed status of the Bastion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.gardener.cloud/v1alpha1.BastionIngressPolicy">BastionIngressPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#operations.gardener.cloud/v1alpha1.BastionSpec">BastionSpec</a>)
</p>
<p>
<p>BastionIngressPolicy represents an ingress policy for SSH bastion hosts.</p>
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
<code>ipBlock</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#ipblock-v1-networking">
Kubernetes networking/v1.IPBlock
</a>
</em>
</td>
<td>
<p>IPBlock defines an IP block that is allowed to access the bastion.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.gardener.cloud/v1alpha1.BastionSpec">BastionSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#operations.gardener.cloud/v1alpha1.Bastion">Bastion</a>)
</p>
<p>
<p>BastionSpec is the specification of a Bastion.</p>
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
<code>shootRef</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ShootRef defines the target shoot for a Bastion. The name field of the ShootRef is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>seedName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SeedName is the name of the seed to which this Bastion is currently scheduled. This field is populated
at the beginning of a create/reconcile operation.</p>
</td>
</tr>
<tr>
<td>
<code>providerType</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderType is cloud provider used by the referenced Shoot.</p>
</td>
</tr>
<tr>
<td>
<code>sshPublicKey</code></br>
<em>
string
</em>
</td>
<td>
<p>SSHPublicKey is the user&rsquo;s public key. This field is immutable.</p>
</td>
</tr>
<tr>
<td>
<code>ingress</code></br>
<em>
<a href="#operations.gardener.cloud/v1alpha1.BastionIngressPolicy">
[]BastionIngressPolicy
</a>
</em>
</td>
<td>
<p>Ingress controls from where the created bastion host should be reachable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="operations.gardener.cloud/v1alpha1.BastionStatus">BastionStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#operations.gardener.cloud/v1alpha1.Bastion">Bastion</a>)
</p>
<p>
<p>BastionStatus holds the most recently observed status of the Bastion.</p>
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#loadbalanceringress-v1-core">
Kubernetes core/v1.LoadBalancerIngress
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ingress holds the public IP and/or hostname of the bastion instance.</p>
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
<em>(Optional)</em>
<p>Conditions represents the latest available observations of a Bastion&rsquo;s current state.</p>
</td>
</tr>
<tr>
<td>
<code>lastHeartbeatTimestamp</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastHeartbeatTimestamp is the time when the bastion was last marked as
not to be deleted. When this is set, the ExpirationTimestamp is advanced
as well.</p>
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
<em>(Optional)</em>
<p>ExpirationTimestamp is the time after which a Bastion is supposed to be
garbage collected.</p>
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
<em>(Optional)</em>
<p>ObservedGeneration is the most recent generation observed for this Bastion. It corresponds to the
Bastion&rsquo;s generation, which is updated on mutation by the API Server.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
