<p>Packages:</p>
<ul>
<li>
<a href="#resources.gardener.cloud%2fv1alpha1">resources.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="resources.gardener.cloud/v1alpha1">resources.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#managedresource">ManagedResource</a>
</li>
</ul>

<h3 id="managedresource">ManagedResource
</h3>


<p>
ManagedResource describes a list of managed resources.
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
<a href="#managedresourcespec">ManagedResourceSpec</a>
</em>
</td>
<td>
<p>Spec contains the specification of this managed resource.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#managedresourcestatus">ManagedResourceStatus</a>
</em>
</td>
<td>
<p>Status contains the status of this managed resource.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedresourcespec">ManagedResourceSpec
</h3>


<p>
(<em>Appears on:</em><a href="#managedresource">ManagedResource</a>)
</p>

<p>
ManagedResourceSpec contains the specification of this managed resource.
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
<code>class</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Class holds the resource class used to control the responsibility for multiple resource manager instances</p>
</td>
</tr>
<tr>
<td>
<code>secretRefs</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core">LocalObjectReference</a> array
</em>
</td>
<td>
<p>SecretRefs is a list of secret references.</p>
</td>
</tr>
<tr>
<td>
<code>injectLabels</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>InjectLabels injects the provided labels into every resource that is part of the referenced secrets.</p>
</td>
</tr>
<tr>
<td>
<code>forceOverwriteLabels</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceOverwriteLabels specifies that all existing labels should be overwritten. Defaults to false.</p>
</td>
</tr>
<tr>
<td>
<code>forceOverwriteAnnotations</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ForceOverwriteAnnotations specifies that all existing annotations should be overwritten. Defaults to false.</p>
</td>
</tr>
<tr>
<td>
<code>keepObjects</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>KeepObjects specifies whether the objects should be kept although the managed resource has already been deleted.<br />Defaults to false.</p>
</td>
</tr>
<tr>
<td>
<code>equivalences</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#groupkind-v1-meta">GroupKind</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Equivalences specifies possible group/kind equivalences for objects.</p>
</td>
</tr>
<tr>
<td>
<code>deletePersistentVolumeClaims</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>DeletePersistentVolumeClaims specifies if PersistentVolumeClaims created by StatefulSets, which are managed by this<br />resource, should also be deleted when the corresponding StatefulSet is deleted (defaults to false).</p>
</td>
</tr>

</tbody>
</table>


<h3 id="managedresourcestatus">ManagedResourceStatus
</h3>


<p>
(<em>Appears on:</em><a href="#managedresource">ManagedResource</a>)
</p>

<p>
ManagedResourceStatus is the status of a managed resource.
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
<code>conditions</code></br>
<em>
Condition array
</em>
</td>
<td>
<p></p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
integer
</em>
</td>
<td>
<p>ObservedGeneration is the most recent generation observed for this resource.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="#objectreference">ObjectReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Resources is a list of objects that have been created.</p>
</td>
</tr>
<tr>
<td>
<code>secretsDataChecksum</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretsDataChecksum is the checksum of referenced secrets data.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="objectreference">ObjectReference
</h3>


<p>
(<em>Appears on:</em><a href="#managedresourcestatus">ManagedResourceStatus</a>)
</p>

<p>
ObjectReference is a reference to another object.
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
<em>(Optional)</em>
<p>Kind of the referent.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds</p>
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
<p>Namespace of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/</p>
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
<em>(Optional)</em>
<p>Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names</p>
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
<em>(Optional)</em>
<p>UID of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids</p>
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
<em>(Optional)</em>
<p>API version of the referent.</p>
</td>
</tr>
<tr>
<td>
<code>resourceVersion</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Specific resourceVersion to which this reference is made, if any.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency</p>
</td>
</tr>
<tr>
<td>
<code>fieldPath</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If referring to a piece of an object instead of an entire object, this string<br />should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].<br />For example, if the object reference is to a container within a pod, this would take on a value like:<br />"spec.containers\{name\}" (where "name" refers to the name of the container that triggered<br />the event) or if no container name is specified "spec.containers[2]" (container with<br />index 2 in this pod). This syntax is chosen only to have some well-defined way of<br />referencing a part of an object.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<p>Labels is a map of labels that were used during last update of the resource.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
object (keys:string, values:string)
</em>
</td>
<td>
<p>Annotations is a map of annotations that were used during last update of the resource.</p>
</td>
</tr>

</tbody>
</table>


