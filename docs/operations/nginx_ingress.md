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
- **`gardener-controller-manager`**: During the next scheduled maintenance window, automatically sets `.spec.addons.nginxIngress.enabled: false` on all Shoot clusters that still have the addon enabled. The shoot status will reflect the change with the message: `.spec.addons.nginxIngress was disabled. Reason: nginx ingress addon disallowed by landscape operator`. Existing `Ingress` resources remain in the cluster.

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

1. **List all `Ingress` resources** in the Garden runtime cluster and check if any still rely on nginx-ingress, e.g. by checking the `ingressClassName`:

   ```bash
   kubectl get ingress --all-namespaces -o wide
   kubectl get ingress --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}: {.spec.ingressClassName}{"\n"}{end}'
   ```

   Ingresses using `nginx-ingress-gardener` (the Garden/Seed ingress class) depend on nginx-ingress.

2. **Check installed extensions** in the Garden cluster for any that create `Ingress` resources. Known extensions that may use `Ingress` resources include:
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

1. **List all `Ingress` resources** across relevant namespaces in the Seed:

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
2. **Migrate extensions**: Ensure all extensions that create `Ingress` resources have been updated to use an alternative (e.g., Istio `VirtualService`, Traefik `IngressRoute`, or Gateway API resources).
3. **Communicate with shoot owners**: Give shoot owners time to migrate their workloads (see [Shoot Cluster Owner Guide](#shoot-cluster-owner-guide)).
4. **Disable in Garden runtime**: Enable `DisableNginxIngressInGarden` for `gardener-operator` after verifying no Garden-level `Ingress` resources remain.
5. **Disable in Seeds**: Enable `DisableNginxIngressInSeed` for the gardenlet after verifying no Seed-level `Ingress` resources remain.
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

Your `Ingress` resources use ingress class `nginx` (either via `spec.ingressClassName: nginx` or the annotation `kubernetes.io/ingress.class: nginx`).

### Migration to `gardener-extension-shoot-traefik`

The recommended migration path for shoot cluster owners is to use the [`gardener-extension-shoot-traefik`](https://github.com/gardener/gardener-extension-shoot-traefik), which deploys [Traefik v3](https://traefik.io) as a replacement ingress controller.

Traefik offers an NGINX-compatible mode (`ingressProvider: KubernetesIngressNGINX`) that allows most existing `Ingress` resources to work without modification.

While Traefik is almost compatible with nginx-ingress, some advanced nginx annotations may not be supported. Review the [Traefik documentation](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress-nginx/) for any specific features/annotations you rely on.

> [!IMPORTANT]
> **Migrating to Traefik via this extension cannot be done with zero downtime - in either mode.** You must disable and remove the nginx-ingress addon *first*, and only then install the Traefik extension.
>
> Therefore, for **both** modes: **disable the nginx-ingress addon and wait for it to be removed, then enable the Traefik extension.** Expect a downtime window between removing nginx-ingress and Traefik being ready plus DNS TTL/propagation. See [Step 3: Disable the Nginx Ingress Addon First](#step-3-disable-the-nginx-ingress-addon-first) below.

> [!NOTE]
> The `gardener-extension-shoot-traefik` extension must be enabled by your landscape operator before you can use it. Check with your Gardener administrator if it is available.

#### Step 1: Verify the Extension Is Available

In the virtual Garden cluster, check if the Traefik extension is registered:

```bash
kubectl get controllerregistrations
```

Look for a registration with `extension-shoot-traefik` or similar in the name.

#### Step 2: Choose an `ingressProvider` Mode

The Traefik extension supports two `ingressProvider` modes. This choice affects how much you need to touch your `Ingress` resources, but **not** the migration order — in both cases you must remove the nginx-ingress addon before installing Traefik (see the note above).

- **`KubernetesIngressNGINX` (NGINX-compatible mode)** — Traefik reuses the `nginx` `IngressClass`, so your existing `Ingress` resources keep `ingressClassName: nginx` and need **no changes**. Most `nginx.ingress.kubernetes.io/*` annotations are honored. It is recommended when you rely on nginx annotations or want the least churn.
- **`KubernetesIngress`** — Traefik uses its own `traefik` `IngressClass`. You must change `ingressClassName: nginx` → `ingressClassName: traefik` on each `Ingress`. nginx-specific annotations are **not** translated; convert them to Traefik `Middleware`/`IngressRoute` equivalents.

In both modes the extension serves the same wildcard domain `*.ingress.<shoot-domain>` via its own `DNSRecord`, so your hostnames stay the same.

> [!WARNING]
> Do **not** enable the `shoot-traefik` extension while the nginx-ingress addon is still enabled. Both create a wildcard `DNSRecord` for `*.ingress.<shoot-domain>` (and, in `KubernetesIngressNGINX` mode, both manage the `nginx` `IngressClass`). Running them together causes conflicts and prevents the addon from being cleanly removed.

#### Step 3: Disable the Nginx Ingress Addon

Set `spec.addons.nginxIngress.enabled: false` and wait for the Shoot to reconcile. Gardener removes the nginx-ingress controller, its `LoadBalancer` Service, the `nginx` `IngressClass`, and the wildcard DNS record `*.ingress.<shoot-domain>`.

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: my-shoot
  namespace: garden-my-project
spec:
  addons:
    nginxIngress:
      enabled: false
```

Apply the change and wait for the Shoot to reconcile:

```bash
kubectl apply -f shoot.yaml
kubectl -n garden-my-project get shoot my-shoot -w
```

> [!NOTE]
> From this point until Traefik is ready and DNS points to it, ingress traffic is interrupted. This is the downtime window (bounded at the end by DNS TTL/propagation once Traefik recreates the wildcard record).

#### Step 4: Enable the shoot-traefik Extension

Once the nginx-ingress addon has been removed, add the extension to your Shoot spec. Pick the `ingressProvider` you chose in Step 2 (`KubernetesIngressNGINX` shown here):

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
```

Apply the change and wait for the Shoot to reconcile:

```bash
kubectl apply -f shoot.yaml
kubectl -n garden-my-project get shoot my-shoot -w
```

After reconciliation, verify Traefik is running in your Shoot cluster:

```bash
# Using shoot kubeconfig
kubectl -n kube-system get pods -l app=traefik
kubectl -n kube-system get svc -l app=traefik
```

#### Step 5: DNS and Ingress Class

The Traefik extension creates the wildcard DNS record `*.ingress.<shoot-domain>` (now pointing to Traefik's LoadBalancer). Your domain names therefore stay the same in both modes.

- **`KubernetesIngressNGINX` mode:** Traefik also provides the `nginx` `IngressClass`, so **your existing `Ingress` resources do not need to change** — they keep `ingressClassName: nginx` and are served by Traefik once DNS has propagated.
- **`KubernetesIngress` mode:** Traefik uses the `traefik` `IngressClass`, so you must update each `Ingress` — see [Step 6](#step-6-update-ingress-resources-kubernetesingress-mode-only).

> [!NOTE]
> The switch is complete once the wildcard record resolves to Traefik's LoadBalancer. Expect a delay bounded by the DNS TTL and propagation time; combined with Step 3 this is the total downtime window.

#### Step 6: Update `Ingress` Resources (`KubernetesIngress` mode only)

Skip this step if you chose `KubernetesIngressNGINX` mode.

For each `Ingress` resource, change the ingress class from `nginx` to `traefik`:

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

**After (Traefik with its own ingress class):**

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
> The `KubernetesIngress` provider does not translate nginx-specific annotations. If you rely on `nginx.ingress.kubernetes.io/*` annotations, either use `KubernetesIngressNGINX` mode instead or convert them to Traefik `Middleware`/`IngressRoute` equivalents. Review [Traefik's documentation](https://doc.traefik.io/traefik/).

Apply the updated Ingress and verify the route is working via Traefik:

```bash
kubectl apply -f ingress.yaml
curl -H "Host: my-app.ingress.my-shoot.example.com" https://<your-shoot-domain-or-lb>/
```

Repeat for each `Ingress` resource.

#### Step 7: Handle Certificates

**If you use [`cert-manager`](https://cert-manager.io/) or the [`gardener-extension-shoot-cert-service`](https://github.com/gardener/gardener-extension-shoot-cert-service):**

Both [`cert-manager`](https://cert-manager.io/) and the [`gardener-extension-shoot-cert-service`](https://github.com/gardener/gardener-extension-shoot-cert-service) watch `Ingress` resources for certificate requests. They watch all `Ingress` resources regardless of class, so certificates continue to be managed automatically after migration.

Verify that your certificate annotations are present on the `Ingress` resources after migration:

```yaml
metadata:
  annotations:
    cert.gardener.cloud/purpose: managed   # for shoot-cert-service
    # OR
    cert-manager.io/cluster-issuer: my-issuer  # for cert-manager
```

**If you use TLS secrets referenced in `Ingress` resources:**

No change is needed — the TLS secret reference in `spec.tls` works the same way with Traefik.

Once DNS has propagated, verify your applications are reachable via Traefik over HTTPS:

```bash
curl -H "Host: my-app.ingress.my-shoot.example.com" https://<your-shoot-domain-or-lb>/
```

### Migration Summary

The migration order is the same for both modes: **remove the nginx-ingress addon, then install Traefik.** There is no zero-downtime path with this extension, because Traefik always creates its own wildcard `DNSRecord` for `*.ingress.<shoot-domain>`, which conflicts with the addon's.

| | `KubernetesIngressNGINX` | `KubernetesIngress` |
|---|---|---|
| `IngressClass` | `nginx` (reused) | `traefik` |
| `Ingress` changes needed | None | `ingressClassName` → `traefik` (per `Ingress`) |
| nginx annotations honored | Most | No (convert to Traefik CRDs) |
| Wildcard `DNSRecord` conflict with addon | Yes | Yes |
| `IngressClass` conflict with addon | Yes | No |
| Must remove addon before installing Traefik | Yes | Yes |
| Downtime | Yes | Yes |

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

**Q: What happens to my `Ingress` resources if the landscape operator forcibly disables the addon?**

When `DisableNginxIngressInShoot` is set for `gardener-controller-manager`, the addon is disabled during your next maintenance window. The nginx-ingress controller is then removed by the gardenlet. Your `Ingress` resources remain in the cluster, but they will no longer be served. Applications using those `Ingress` resources will become unreachable.

Make sure to migrate before your landscape operator enforces the disable, or contact your landscape operator to request more time.

**Q: Can I use the nginx-ingress addon on production or development purpose shoots?**

No. The nginx-ingress addon (and all Shoot addons) can only be enabled on shoots with `spec.purpose: evaluation`. For production workloads, i.e, clusters with `spec.purpose: production`, use the `shoot-traefik` extension or deploy your own ingress controller.

**Q: My Kubernetes version is 1.35 or newer. Can I still use nginx-ingress?**

No. The Shoot addons field (which includes nginx-ingress) is forbidden for Kubernetes >= 1.35. You must use the `shoot-traefik` extension or another ingress solution.

**Q: What if the shoot-traefik extension is not available on my landscape?**

Contact your landscape operator to request it. Alternatively, you can deploy Traefik or another ingress controller (e.g., HAProxy Ingress, Contour, NGINX from F5) directly into your Shoot cluster as a regular workload, bypassing the Gardener extension mechanism. Manage the LoadBalancer Service and DNS records yourself in that case.

**Q: Can I run the shoot-traefik extension alongside the nginx-ingress addon (in parallel)?**

No — in **neither** mode. The Traefik extension always creates a wildcard `DNSRecord` for `*.ingress.<shoot-domain>` in the control plane, which is the same domain the nginx-ingress addon manages, so the two collide regardless of `ingressProvider`. In `ingressProvider: KubernetesIngressNGINX` mode there is an additional conflict: both also manage a `ManagedResource` for the `IngressClass` named `nginx`, which then flaps (continuously deleted and re-created) and prevents the addon from being cleanly removed. In both modes you must disable/remove the nginx-ingress addon **first**, then install Traefik. See [Step 3: Disable the Nginx Ingress Addon First](#step-3-disable-the-nginx-ingress-addon-first).

**Q: Does the migration cause downtime?**

Yes. Because the Traefik extension cannot run in parallel with the nginx-ingress addon (see above), you must remove the addon before installing Traefik. Ingress traffic is interrupted from the moment nginx-ingress is removed until Traefik is ready and the wildcard DNS record `*.ingress.<shoot-domain>` resolves to Traefik's LoadBalancer (bounded by the DNS TTL and propagation time). Keeping a low DNS TTL beforehand shortens the tail of this window.

**Q: How do I handle the DNS wildcard record during migration?**

The wildcard record `*.ingress.<shoot-domain>` is managed by Gardener and tied to the ingress controller. When you disable the nginx-ingress addon, Gardener removes it; when you then enable the Traefik extension, the extension recreates the same wildcard record pointing to Traefik's LoadBalancer — in **both** `ingressProvider` modes. You do not create or manage this record yourself. The gap between removal and recreation (plus DNS TTL/propagation) is the downtime window.
