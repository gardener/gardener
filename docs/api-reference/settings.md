<p>Packages:</p>
<ul>
<li>
<a href="#settings.gardener.cloud%2fv1alpha1">settings.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="settings.gardener.cloud/v1alpha1">settings.gardener.cloud/v1alpha1</h2>
<p>

</p>
Resource Types:
<ul>
<li>
<a href="#clusteropenidconnectpreset">ClusterOpenIDConnectPreset</a>
</li>
<li>
<a href="#openidconnectpreset">OpenIDConnectPreset</a>
</li>
</ul>

<h3 id="clusteropenidconnectpreset">ClusterOpenIDConnectPreset
</h3>


<p>
ClusterOpenIDConnectPreset is a OpenID Connect configuration that is applied
to a Shoot objects cluster-wide.
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
<a href="#clusteropenidconnectpresetspec">ClusterOpenIDConnectPresetSpec</a>
</em>
</td>
<td>
<p>Spec is the specification of this OpenIDConnect preset.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="clusteropenidconnectpresetspec">ClusterOpenIDConnectPresetSpec
</h3>


<p>
(<em>Appears on:</em><a href="#clusteropenidconnectpreset">ClusterOpenIDConnectPreset</a>)
</p>

<p>
ClusterOpenIDConnectPresetSpec contains the OpenIDConnect specification and
project selector matching Shoots in Projects.
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
<a href="#kubeapiserveropenidconnect">KubeAPIServerOpenIDConnect</a>
</em>
</td>
<td>
<p>Server contains the kube-apiserver's OpenID Connect configuration.<br />This configuration is not overwriting any existing OpenID Connect<br />configuration already set on the Shoot object.</p>
</td>
</tr>
<tr>
<td>
<code>shootSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootSelector decides whether to apply the configuration if the<br />Shoot has matching labels.<br />Use the selector only if the OIDC Preset is opt-in, because end<br />users may skip the admission by setting the labels.<br />Default to the empty LabelSelector, which matches everything.</p>
</td>
</tr>
<tr>
<td>
<code>weight</code></br>
<em>
integer
</em>
</td>
<td>
<p>Weight associated with matching the corresponding preset,<br />in the range 1-100.<br />Required.</p>
</td>
</tr>
<tr>
<td>
<code>projectSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Project decides whether to apply the configuration if the<br />Shoot is in a specific Project matching the label selector.<br />Use the selector only if the OIDC Preset is opt-in, because end<br />users may skip the admission by setting the labels.<br />Defaults to the empty LabelSelector, which matches everything.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kubeapiserveropenidconnect">KubeAPIServerOpenIDConnect
</h3>


<p>
(<em>Appears on:</em><a href="#clusteropenidconnectpresetspec">ClusterOpenIDConnectPresetSpec</a>, <a href="#openidconnectpresetspec">OpenIDConnectPresetSpec</a>)
</p>

<p>
KubeAPIServerOpenIDConnect contains configuration settings for the OIDC provider.
Note: Descriptions were taken from the Kubernetes documentation.
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
<p>If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.</p>
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
<p>The client ID for the OpenID Connect client.<br />Required.</p>
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
<p>The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).<br />Required.</p>
</td>
</tr>
<tr>
<td>
<code>requiredClaims</code></br>
<em>
object (keys:string, values:string)
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
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1<br />Defaults to [RS256]</p>
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
<p>The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This field is experimental, please see the authentication documentation for further details.<br />Defaults to "sub".</p>
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
<p>If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="openidconnectclientauthentication">OpenIDConnectClientAuthentication
</h3>


<p>
OpenIDConnectClientAuthentication contains configuration for OIDC clients.
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
object (keys:string, values:string)
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extra configuration added to kubeconfig's auth-provider.<br />Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token</p>
</td>
</tr>

</tbody>
</table>


<h3 id="openidconnectpreset">OpenIDConnectPreset
</h3>


<p>
OpenIDConnectPreset is a OpenID Connect configuration that is applied
to a Shoot in a namespace.
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
<a href="#openidconnectpresetspec">OpenIDConnectPresetSpec</a>
</em>
</td>
<td>
<p>Spec is the specification of this OpenIDConnect preset.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="openidconnectpresetspec">OpenIDConnectPresetSpec
</h3>


<p>
(<em>Appears on:</em><a href="#clusteropenidconnectpresetspec">ClusterOpenIDConnectPresetSpec</a>, <a href="#openidconnectpreset">OpenIDConnectPreset</a>)
</p>

<p>
OpenIDConnectPresetSpec contains the Shoot selector for which
a specific OpenID Connect configuration is applied.
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
<a href="#kubeapiserveropenidconnect">KubeAPIServerOpenIDConnect</a>
</em>
</td>
<td>
<p>Server contains the kube-apiserver's OpenID Connect configuration.<br />This configuration is not overwriting any existing OpenID Connect<br />configuration already set on the Shoot object.</p>
</td>
</tr>
<tr>
<td>
<code>shootSelector</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#labelselector-v1-meta">LabelSelector</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ShootSelector decides whether to apply the configuration if the<br />Shoot has matching labels.<br />Use the selector only if the OIDC Preset is opt-in, because end<br />users may skip the admission by setting the labels.<br />Default to the empty LabelSelector, which matches everything.</p>
</td>
</tr>
<tr>
<td>
<code>weight</code></br>
<em>
integer
</em>
</td>
<td>
<p>Weight associated with matching the corresponding preset,<br />in the range 1-100.<br />Required.</p>
</td>
</tr>

</tbody>
</table>


