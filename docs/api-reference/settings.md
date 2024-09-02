<p>Packages:</p>
<ul>
<li>
<a href="#settings.gardener.cloud%2fv1alpha1">settings.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="settings.gardener.cloud/v1alpha1">settings.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 is a version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#settings.gardener.cloud/v1alpha1.ClusterOpenIDConnectPreset">ClusterOpenIDConnectPreset</a>
</li><li>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPreset">OpenIDConnectPreset</a>
</li></ul>
<h3 id="settings.gardener.cloud/v1alpha1.ClusterOpenIDConnectPreset">ClusterOpenIDConnectPreset
</h3>
<p>
<p>ClusterOpenIDConnectPreset is a OpenID Connect configuration that is applied
to a Shoot objects cluster-wide.</p>
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
settings.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ClusterOpenIDConnectPreset</code></td>
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
<a href="#settings.gardener.cloud/v1alpha1.ClusterOpenIDConnectPresetSpec">
ClusterOpenIDConnectPresetSpec
</a>
</em>
</td>
<td>
<p>Spec is the specification of this OpenIDConnect preset.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>OpenIDConnectPresetSpec</code></br>
<em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPresetSpec">
OpenIDConnectPresetSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>OpenIDConnectPresetSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>projectSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Project decides whether to apply the configuration if the
Shoot is in a specific Project matching the label selector.
Use the selector only if the OIDC Preset is opt-in, because end
users may skip the admission by setting the labels.
Defaults to the empty LabelSelector, which matches everything.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="settings.gardener.cloud/v1alpha1.OpenIDConnectPreset">OpenIDConnectPreset
</h3>
<p>
<p>OpenIDConnectPreset is a OpenID Connect configuration that is applied
to a Shoot in a namespace.</p>
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
settings.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>OpenIDConnectPreset</code></td>
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
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPresetSpec">
OpenIDConnectPresetSpec
</a>
</em>
</td>
<td>
<p>Spec is the specification of this OpenIDConnect preset.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>server</code></br>
<em>
<a href="#settings.gardener.cloud/v1alpha1.KubeAPIServerOpenIDConnect">
KubeAPIServerOpenIDConnect
</a>
</em>
</td>
<td>
<p>Server contains the kube-apiserver&rsquo;s OpenID Connect configuration.
This configuration is not overwriting any existing OpenID Connect
configuration already set on the Shoot object.</p>
</td>
</tr>
<tr>
<td>
<code>client</code></br>
<em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectClientAuthentication">
OpenIDConnectClientAuthentication
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Client contains the configuration used for client OIDC authentication
of Shoot clusters.
This configuration is not overwriting any existing OpenID Connect
client authentication already set on the Shoot object.</p>
<p>Deprecated: The OpenID Connect configuration this field specifies is not used and will be forbidden starting from Kubernetes 1.31.
It&rsquo;s use was planned for genereting OIDC kubeconfig <a href="https://github.com/gardener/gardener/issues/1433">https://github.com/gardener/gardener/issues/1433</a>
TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.30 is dropped.</p>
</td>
</tr>
<tr>
<td>
<code>shootSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootSelector decides whether to apply the configuration if the
Shoot has matching labels.
Use the selector only if the OIDC Preset is opt-in, because end
users may skip the admission by setting the labels.
Default to the empty LabelSelector, which matches everything.</p>
</td>
</tr>
<tr>
<td>
<code>weight</code></br>
<em>
int32
</em>
</td>
<td>
<p>Weight associated with matching the corresponding preset,
in the range 1-100.
Required.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="settings.gardener.cloud/v1alpha1.ClusterOpenIDConnectPresetSpec">ClusterOpenIDConnectPresetSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#settings.gardener.cloud/v1alpha1.ClusterOpenIDConnectPreset">ClusterOpenIDConnectPreset</a>)
</p>
<p>
<p>ClusterOpenIDConnectPresetSpec contains the OpenIDConnect specification and
project selector matching Shoots in Projects.</p>
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
<code>OpenIDConnectPresetSpec</code></br>
<em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPresetSpec">
OpenIDConnectPresetSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>OpenIDConnectPresetSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>projectSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Project decides whether to apply the configuration if the
Shoot is in a specific Project matching the label selector.
Use the selector only if the OIDC Preset is opt-in, because end
users may skip the admission by setting the labels.
Defaults to the empty LabelSelector, which matches everything.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="settings.gardener.cloud/v1alpha1.KubeAPIServerOpenIDConnect">KubeAPIServerOpenIDConnect
</h3>
<p>
(<em>Appears on:</em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPresetSpec">OpenIDConnectPresetSpec</a>)
</p>
<p>
<p>KubeAPIServerOpenIDConnect contains configuration settings for the OIDC provider.
Note: Descriptions were taken from the Kubernetes documentation.</p>
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
<code>caBundle</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If set, the OpenID server&rsquo;s certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host&rsquo;s root CA set will be used.</p>
</td>
</tr>
<tr>
<td>
<code>clientID</code></br>
<em>
string
</em>
</td>
<td>
<p>The client ID for the OpenID Connect client.
Required.</p>
</td>
</tr>
<tr>
<td>
<code>groupsClaim</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This field is experimental, please see the authentication documentation for further details.</p>
</td>
</tr>
<tr>
<td>
<code>groupsPrefix</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.</p>
</td>
</tr>
<tr>
<td>
<code>issuerURL</code></br>
<em>
string
</em>
</td>
<td>
<p>The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
Required.</p>
</td>
</tr>
<tr>
<td>
<code>requiredClaims</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.</p>
</td>
</tr>
<tr>
<td>
<code>signingAlgs</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of allowed JOSE asymmetric signing algorithms. JWTs with a &lsquo;alg&rsquo; header value not in this list will be rejected. Values are defined by RFC 7518 <a href="https://tools.ietf.org/html/rfc7518#section-3.1">https://tools.ietf.org/html/rfc7518#section-3.1</a>
Defaults to [RS256]</p>
</td>
</tr>
<tr>
<td>
<code>usernameClaim</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The OpenID claim to use as the user name. Note that claims other than the default (&lsquo;sub&rsquo;) is not guaranteed to be unique and immutable. This field is experimental, please see the authentication documentation for further details.
Defaults to &ldquo;sub&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>usernamePrefix</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>If provided, all usernames will be prefixed with this value. If not provided, username claims other than &lsquo;email&rsquo; are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value &lsquo;-&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="settings.gardener.cloud/v1alpha1.OpenIDConnectClientAuthentication">OpenIDConnectClientAuthentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPresetSpec">OpenIDConnectPresetSpec</a>)
</p>
<p>
<p>OpenIDConnectClientAuthentication contains configuration for OIDC clients.</p>
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
<code>secret</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The client Secret for the OpenID Connect client.</p>
</td>
</tr>
<tr>
<td>
<code>extraConfig</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extra configuration added to kubeconfig&rsquo;s auth-provider.
Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token</p>
</td>
</tr>
</tbody>
</table>
<h3 id="settings.gardener.cloud/v1alpha1.OpenIDConnectPresetSpec">OpenIDConnectPresetSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectPreset">OpenIDConnectPreset</a>, 
<a href="#settings.gardener.cloud/v1alpha1.ClusterOpenIDConnectPresetSpec">ClusterOpenIDConnectPresetSpec</a>)
</p>
<p>
<p>OpenIDConnectPresetSpec contains the Shoot selector for which
a specific OpenID Connect configuration is applied.</p>
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
<code>server</code></br>
<em>
<a href="#settings.gardener.cloud/v1alpha1.KubeAPIServerOpenIDConnect">
KubeAPIServerOpenIDConnect
</a>
</em>
</td>
<td>
<p>Server contains the kube-apiserver&rsquo;s OpenID Connect configuration.
This configuration is not overwriting any existing OpenID Connect
configuration already set on the Shoot object.</p>
</td>
</tr>
<tr>
<td>
<code>client</code></br>
<em>
<a href="#settings.gardener.cloud/v1alpha1.OpenIDConnectClientAuthentication">
OpenIDConnectClientAuthentication
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Client contains the configuration used for client OIDC authentication
of Shoot clusters.
This configuration is not overwriting any existing OpenID Connect
client authentication already set on the Shoot object.</p>
<p>Deprecated: The OpenID Connect configuration this field specifies is not used and will be forbidden starting from Kubernetes 1.31.
It&rsquo;s use was planned for genereting OIDC kubeconfig <a href="https://github.com/gardener/gardener/issues/1433">https://github.com/gardener/gardener/issues/1433</a>
TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.30 is dropped.</p>
</td>
</tr>
<tr>
<td>
<code>shootSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta">
Kubernetes meta/v1.LabelSelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootSelector decides whether to apply the configuration if the
Shoot has matching labels.
Use the selector only if the OIDC Preset is opt-in, because end
users may skip the admission by setting the labels.
Default to the empty LabelSelector, which matches everything.</p>
</td>
</tr>
<tr>
<td>
<code>weight</code></br>
<em>
int32
</em>
</td>
<td>
<p>Weight associated with matching the corresponding preset,
in the range 1-100.
Required.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
