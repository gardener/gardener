---
title: Nginx Ingress Retirement and Migration Guide
description: Guide for landscape operators and shoot cluster owners on how to deal with the nginx-ingress retirement
---

# Nginx Ingress Retirement and Migration Guide

Gardener has used nginx-ingress as the ingress controller for a long time successfully.
However, after [its retirement](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/), it is a good idea to switch to an alternative solution.
The Kubernetes project announced that best-effort maintenance continues until March 2026, after which no further releases, bugfixes, or security vulnerability updates will be provided.

Gardener itself moved to [Istio](https://istio.io) for this purpose (see [#13448](https://github.com/gardener/gardener/issues/13448) for details).
As nginx-ingress was available for a long period of time, other components deployed in Gardener clusters, such as extensions, might rely on an ingress controller.
This is why Gardener still deploys nginx-ingress by default, but it can be disabled if not needed.

This document describes the different options to disable nginx-ingress and how to migrate workloads.

## Landscape Operator Guide

This section is intended for landscape operators who manage Gardener installations and want to disable nginx-ingress across their infrastructure for security or compliance reasons.

### Feature Gates

Three feature gates control the nginx-ingress deployment across the different cluster types. All three are currently in **Alpha** state (introduced in `v1.142.0`), per default disabled and must be explicitly enabled.

| Feature Gate | Component | Effect                                                                         |
|---|---|--------------------------------------------------------------------------------|
| `DisableNginxIngressInGarden` | `gardener-operator` | Disables and removes nginx-ingress in the Garden runtime cluster               |
| `DisableNginxIngressInSeed` | `gardenlet` | Disables and removes nginx-ingress in Seed clusters                            |
| `DisableNginxIngressInShoot` | `gardener-apiserver`, `gardener-controller-manager` | Disables and removes nginx-ingress addon in Shoot clusters (see details below) |

The `DisableNginxIngressInShoot` feature gate has differentiated behavior depending on the component it is set for:

- **`gardener-apiserver`**: Blocks creation of new Shoot clusters with the nginx-ingress addon enabled. Existing Shoot clusters can only disable the addon, not enable it again.
- **`gardener-controller-manager`**: During the next scheduled maintenance window, automatically sets `.spec.addons.nginxIngress.enabled: false` on all Shoot clusters that still have the addon enabled. The shoot status will reflect the change with the message: `.spec.addons.nginxIngress was disabled. Reason: nginx ingress addon disallowed by landscape operator`.

### Enabling the Feature Gates

**`gardener-operator` (Garden runtime cluster):**

Set the feature gate in the [`gardener-operator` component configuration](../../example/operator/10-componentconfig.yaml):

```yaml
featureGates:
  DisableNginxIngressInGarden: true
```

**`gardenlet` (Seed clusters):**

In the [`Gardenlet` resource](../../example/55-gardenlet.yaml) or the [gardenlet component configuration](../../example/20-componentconfig-gardenlet.yaml):

```yaml
featureGates:
  DisableNginxIngressInSeed: true
```

**`gardener-apiserver` and `gardener-controller-manager` (Shoot clusters):**

In the respective component configurations in the [`Garden` resource](../../example/operator/20-garden.yaml):

```yaml
apiVersion: operator.gardener.cloud/v1alpha1
kind: Garden
metadata:
   name: local
spec:
  ...
  virtualCluster:
    gardener:
      gardenerAPIServer:
        featureGates:
          DisableNginxIngressInShoot: true
      gardenerControllerManager:
        featureGates:
          DisableNginxIngressInShoot: true
```

### How to Check if Nginx Ingress Can Be Disabled

Before disabling nginx-ingress, verify that no components in your landscape depend on it.
The following checklist helps identify remaining dependencies.

#### Garden Runtime Cluster

1. **List all `Ingress` resources** in the Garden runtime cluster and check if any still rely on on nginx-ingress, e.g. by checking the `ingressClassName`:

   ```bash
   kubectl get ingress --all-namespaces -o wide
   kubectl get ingress --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}: {.spec.ingressClassName}{"\n"}{end}'
   ```

   Ingresses using `nginx-ingress-gardener` (the Garden/Seed ingress class) depend on nginx-ingress.

2. **Check installed extensions** in the Garden cluster for any that create Ingress resources. Known extensions that may use ingress resources include:
   - [gardener-extension-shoot-falco](https://github.com/gardener/gardener-extension-shoot-falco)
   - [oidc-apps-controller](https://github.com/gardener/oidc-apps-controller)

   Contact the extension owners or check their documentation if they have migrated away from nginx-ingress.

3. **Check for annotations** referencing the nginx ingress class:

   ```bash
   kubectl get ingress --all-namespaces -o json | jq '.items[] | select(.metadata.annotations["kubernetes.io/ingress.class"] == "nginx-ingress-gardener") | "\(.metadata.namespace)/\(.metadata.name)"'
   ```

4. After confirming no active Ingresses use `nginx-ingress-gardener`, enable `DisableNginxIngressInGarden` for the gardener-operator.

#### Seed Clusters

The same checks apply to each Seed cluster. The nginx ingress class for Seeds is `nginx-ingress-gardener`.

1. **List all Ingress resources** across relevant namespaces in the Seed:

   ```bash
   kubectl get ingress --all-namespaces -o wide
   kubectl get ingress --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}: {.spec.ingressClassName}{"\n"}{end}'
   ```

2. After confirming no active Ingresses use `nginx-ingress-gardener`, enable `DisableNginxIngressInSeed` for the gardenlet on that Seed.

#### Shoot Clusters (Addon)

1. **List all Shoot clusters** with the nginx-ingress addon still enabled:

   ```bash
   kubectl get shoots --all-namespaces -o json | \
     jq -r '.items[] | select(.spec.addons.nginxIngress.enabled == true) | "\(.metadata.namespace)/\(.metadata.name)"'
   ```

2. **Communicate with shoot owners** that they need to migrate their ingress workloads before you enforce the disablement. See the [Shoot Cluster Owner Guide](#shoot-cluster-owner-guide) for migration instructions.

3. **Set the feature gate in `gardener-apiserver`** first. This prevents new shoots from re-enabling the addon and blocks enabling it on existing shoots.

4. **Set the feature gate in `gardener-controller-manager`** at a potentially later point in time for triggering the removal of nginx-ingress from all shoot clusters. This will disable the addon during the next maintenance window for all shoots that still have it enabled. The change appears in `.status.lastMaintenance` of each affected shoot.

### Recommended Migration Order

To minimize risk, follow this order when disabling nginx-ingress across a landscape:

1. **Identify dependencies**: Run the checks above for all cluster types.
2. **Migrate extensions**: Ensure all extensions that create Ingress resources have been updated to use an alternative (e.g., Istio `VirtualService`, Traefik `IngressRoute`, or Gateway API resources).
3. **Communicate with shoot owners**: Give shoot owners time to migrate their workloads (see [Shoot Cluster Owner Guide](#shoot-cluster-owner-guide)).
4. **Disable in Garden runtime**: Enable `DisableNginxIngressInGarden` for `gardener-operator` after verifying no Garden-level Ingress resources remain.
5. **Disable in Seeds**: Enable `DisableNginxIngressInSeed` for the gardenlet after verifying no Seed-level Ingress resources remain.
6. **Disable in Shoots for new cluster**: Enable `DisableNginxIngressInShoot` for `gardener-apiserver`. 
7. **Disable in Shoots for all clusters**: Enable `DisableNginxIngressInShoot` for `gardener-controller-manager`.
   - Wait for all shoots to go through maintenance.
   - Verify no shoots have the addon enabled anymore.

---

## Shoot Cluster Owner Guide

This section is intended for users who own Shoot clusters with the nginx-ingress addon enabled and need to migrate their workloads.

> [!NOTE]
> The nginx-ingress addon can only be enabled on Shoot clusters with `spec.purpose: evaluation`. It is deprecated and will be forbidden starting with Kubernetes version 1.35.

### Understanding Your Current Setup

When the nginx-ingress addon is enabled, Gardener:

1. Deploys an `nginx-ingress-controller` as a `Deployment` in the `kube-system` namespace of your Shoot cluster.
2. Creates a `LoadBalancer` Service that exposes ports 80 (HTTP) and 443 (HTTPS).
3. Creates a wildcard DNS record `*.ingress.<shoot-domain>` pointing to the load balancer's IP or hostname.

Your Ingress resources use ingress class `nginx` (either via `spec.ingressClassName: nginx` or the annotation `kubernetes.io/ingress.class: nginx`).

### Migration to gardener-extension-shoot-traefik

The recommended migration path for shoot cluster owners is to use the [gardener-extension-shoot-traefik](https://github.com/gardener/gardener-extension-shoot-traefik), which deploys [Traefik v3](https://traefik.io) as a replacement ingress controller.

Traefik offers an NGINX-compatible mode (`ingressProvider: KubernetesIngressNGINX`) that allows most existing Ingress resources to work without modification.

While Traefik is almost compatible with nginx-ingress, some advanced nginx annotations may not be supported. Review the [Traefik documentation](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress-nginx/) for any specific features/annotations you rely on.

> [!NOTE]
> The `gardener-extension-shoot-traefik` extension must be enabled by your landscape operator before you can use it. Check with your Gardener administrator if it is available.

#### Step 1: Verify the Extension Is Available

In the virtual Garden cluster, check if the Traefik extension is registered:

```bash
kubectl get controllerregistrations
```

Look for a registration with `extension-shoot-traefik` or similar in the name.

#### Step 2: Plan for Zero-Downtime Migration

A zero-downtime migration runs both nginx-ingress (existing) and Traefik (new) in parallel during the transition. Traffic is cut over per-Ingress resource by changing the `ingressClassName`.

The high-level steps are:

1. Enable the `shoot-traefik` extension (Traefik is deployed alongside nginx-ingress).
2. Migrate Ingress resources one by one or in batches by changing their ingress class.
3. Verify each migrated service is working correctly with Traefik.
4. Disable the nginx-ingress addon once all Ingress resources are migrated.

#### Step 3: Enable the shoot-traefik Extension

Add the extension to your Shoot spec. Using NGINX-compatible mode is recommended for easier migration:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: garden-my-project
spec:
  extensions:
  - type: shoot-traefik
    providerConfig:
      apiVersion: traefik.extensions.gardener.cloud/v1alpha1
      kind: TraefikConfig
      replicas: 2
      # KubernetesIngressNGINX enables NGINX annotation compatibility
      ingressProvider: KubernetesIngressNGINX
  # Keep nginx-ingress enabled during transition
  addons:
    nginxIngress:
      enabled: true
```

Apply the change and wait for the Shoot to reconcile:

```bash
kubectl apply -f shoot.yaml
# Wait for reconciliation
kubectl -n garden-my-project get shoot my-shoot -w
```

After reconciliation, verify Traefik is running in your Shoot cluster:

```bash
# Using shoot kubeconfig
kubectl -n kube-system get pods -l app=traefik
kubectl -n kube-system get svc -l app=traefik
```

Note the Traefik LoadBalancer IP or hostname — you will need it for DNS during the migration.

#### Step 4: Handle DNS

The nginx-ingress addon creates a wildcard DNS record `*.ingress.<shoot-domain>` automatically. Traefik gets its own LoadBalancer Service with a different IP or hostname.

**Option A: Update the wildcard DNS record (cutover approach)**

If you want all `*.ingress.<shoot-domain>` traffic to move to Traefik at once, you need to update the wildcard DNS record. This is typically managed by Gardener — disabling the nginx-ingress addon will remove the record, and you will need to re-create it pointing to Traefik using the `shoot-dns-service` extension or your cloud DNS provider directly.

If your landscape has the [shoot-dns-service extension](https://github.com/gardener/gardener-extension-shoot-dns-service) available, you can create a `DNSEntry` resource:

```yaml
apiVersion: dns.gardener.cloud/v1alpha1
kind: DNSEntry
metadata:
  name: ingress-wildcard
  namespace: default  # in shoot cluster
spec:
  dnsName: "*.ingress.<your-shoot-domain>"
  ttl: 120
  targets:
  - <traefik-loadbalancer-ip-or-hostname>
```

**Option B: Use per-Ingress hostnames (gradual approach)**

For a more gradual migration without changing the wildcard DNS record, use different hostnames for migrated services:

- Keep existing Ingresses at `*.ingress.<shoot-domain>` using nginx.
- Create new Ingresses for migrated services at explicit hostnames managed via `shoot-dns-service`.

#### Step 5: Handle Certificates

**If you use cert-manager or the shoot-cert-service extension:**

Both cert-manager and the [shoot-cert-service](https://github.com/gardener/gardener-extension-shoot-cert-service) watch Ingress resources for certificate requests. After changing the `ingressClassName`, the certificate controller should continue to manage the certificates automatically, since they watch all Ingress resources regardless of class.

Verify that your certificate annotations are present on the Ingress resources after migration:

```yaml
metadata:
  annotations:
    cert.gardener.cloud/purpose: managed   # for shoot-cert-service
    # OR
    cert-manager.io/cluster-issuer: my-issuer  # for cert-manager
```

**If you use TLS secrets referenced in Ingress resources:**

No change is needed — the TLS secret reference in `spec.tls` works the same way with Traefik.

```yaml
spec:
  tls:
  - hosts:
    - my-app.ingress.my-shoot.example.com
    secretName: my-app-tls
```

#### Step 6: Migrate Ingress Resources

For each Ingress resource, change the ingress class from `nginx` to `traefik`.

**Before (nginx-ingress):**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: my-namespace
spec:
  ingressClassName: nginx
  rules:
  - host: my-app.ingress.my-shoot.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
  tls:
  - hosts:
    - my-app.ingress.my-shoot.example.com
    secretName: my-app-tls
```

**After (Traefik with NGINX-compatible mode):**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: my-namespace
spec:
  ingressClassName: traefik  # Changed from nginx
  rules:
  - host: my-app.ingress.my-shoot.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
  tls:
  - hosts:
    - my-app.ingress.my-shoot.example.com
    secretName: my-app-tls
```

> [!TIP]
> With `ingressProvider: KubernetesIngressNGINX`, most nginx-specific annotations are supported. However, some advanced nginx annotations (especially `nginx.ingress.kubernetes.io/server-snippet` and `nginx.ingress.kubernetes.io/configuration-snippet`) may not be compatible. Review [Traefik's documentation](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress-nginx/) for alternatives.

Apply the updated Ingress and verify the route is working via Traefik:

```bash
kubectl apply -f ingress.yaml
# Test the service via the Traefik load balancer
curl -H "Host: my-app.ingress.my-shoot.example.com" http://<traefik-lb-ip>/
```

Repeat for each Ingress resource.

#### Step 7: Disable the Nginx Ingress Addon

Once all Ingress resources have been migrated to Traefik and verified:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: garden-my-project
spec:
  extensions:
  - type: shoot-traefik
    providerConfig:
      apiVersion: traefik.extensions.gardener.cloud/v1alpha1
      kind: TraefikConfig
      replicas: 2
      ingressProvider: KubernetesIngressNGINX
  addons:
    nginxIngress:
      enabled: false  # Disable nginx-ingress
```

Apply and wait for reconciliation. Gardener will remove the nginx-ingress controller and its LoadBalancer Service. The wildcard DNS record `*.ingress.<shoot-domain>` will also be removed.

### Zero-Downtime Migration: Step-by-Step Summary

| Step | Action | Expected Downtime |
|---|---|---|
| 1 | Enable `shoot-traefik` extension alongside nginx-ingress | None |
| 2 | Update DNS to point to Traefik LoadBalancer (if using wildcard cutover) | Brief (TTL-dependent) |
| 3 | Migrate Ingress resources to `ingressClassName: traefik` one by one | None per Ingress (both controllers run in parallel) |
| 4 | Verify certificate issuance and HTTPS still works | None |
| 5 | Disable nginx-ingress addon | None (Traefik already serving traffic) |

### Nginx Ingress Annotations Reference

Common nginx annotations and their Traefik equivalents when using `ingressProvider: KubernetesIngressNGINX`:

| Nginx Annotation | Traefik Equivalent |
|---|---|
| `nginx.ingress.kubernetes.io/rewrite-target` | Supported in NGINX-compatible mode |
| `nginx.ingress.kubernetes.io/ssl-redirect` | Supported in NGINX-compatible mode |
| `nginx.ingress.kubernetes.io/proxy-body-size` | Supported in NGINX-compatible mode |
| `nginx.ingress.kubernetes.io/proxy-read-timeout` | Supported in NGINX-compatible mode |
| `nginx.ingress.kubernetes.io/proxy-send-timeout` | Supported in NGINX-compatible mode |
| `nginx.ingress.kubernetes.io/auth-type` | Use Traefik `Middleware` CRD |
| `nginx.ingress.kubernetes.io/server-snippet` | Use Traefik `Middleware` CRD |
| `nginx.ingress.kubernetes.io/configuration-snippet` | Use Traefik `Middleware` CRD |

For complex nginx configurations without direct Traefik equivalents, consider migrating to [Traefik's Gateway API support](https://doc.traefik.io/traefik/providers/kubernetes-gateway/) or Traefik-specific `IngressRoute` CRDs.

### Frequently Asked Questions

**Q: What happens to my Ingress resources if the landscape operator forcibly disables the addon?**

When `DisableNginxIngressInShoot` is set for `gardener-controller-manager`, the addon is disabled during your next maintenance window. The nginx-ingress controller is then removed by the gardenlet. Your Ingress resources remain in the cluster, but they will no longer be served. Applications using those Ingress resources will become unreachable.

Make sure to migrate before your landscape operator enforces the disable, or contact your landscape operator to request more time.

**Q: Can I use the nginx-ingress addon on production or development purpose shoots?**

No. The nginx-ingress addon (and all Shoot addons) can only be enabled on shoots with `spec.purpose: evaluation`. For production workloads, use the `shoot-traefik` extension or deploy your own ingress controller.

**Q: My Kubernetes version is 1.35 or newer. Can I still use nginx-ingress?**

No. The Shoot addons field (which includes nginx-ingress) is forbidden for Kubernetes >= 1.35. You must use the `shoot-traefik` extension or another ingress solution.

**Q: What if the shoot-traefik extension is not available on my landscape?**

Contact your landscape operator to request it. Alternatively, you can deploy Traefik or another ingress controller (e.g., HAProxy Ingress, Contour, NGINX from F5) directly into your Shoot cluster as a regular workload, bypassing the Gardener extension mechanism. Manage the LoadBalancer Service and DNS records yourself in that case.

**Q: How do I handle the DNS wildcard record during migration?**

The wildcard record `*.ingress.<shoot-domain>` is managed by Gardener and tied to the nginx-ingress addon. During parallel operation (both nginx and Traefik running), all `*.ingress.<shoot-domain>` traffic goes to nginx. To cut over, you either:
- Update DNS to point the wildcard to Traefik's LoadBalancer (if using `shoot-dns-service`).
- Move services to new hostnames that you manage yourself via `shoot-dns-service` or cloud DNS.
- Accept a brief DNS TTL-length outage during the final cutover when you disable nginx.
