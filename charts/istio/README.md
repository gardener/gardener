# Test

```console
helm template istio charts/istio/istio-crds -n istio-system > 01-istio.yaml
helm template istio charts/istio/istio-istiod -n istio-system --set=deployNamespace=true > 02-istio.yaml
helm template istio charts/istio/istio-ingress -n istio-ingress --set=deployNamespace=true --set=serviceType=ClusterIP > 03-istio.yaml
```
