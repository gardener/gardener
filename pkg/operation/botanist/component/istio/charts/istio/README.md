# Test

Istio helm repository (not required directly):
```console
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update
```

Current version in gardener repository:
```console
helm template istio charts/istio/istio-crds -n istio-system > 01-istio.yaml
helm template istio charts/istio/istio-istiod -n istio-system --set=deployNamespace=true > 02-istio.yaml
helm template istio charts/istio/istio-ingress -n istio-ingress --set=deployNamespace=true --set=serviceType=ClusterIP > 03-istio.yaml
cat 03-istio.yaml | sed -n "`grep -n EnvoyFilter 03-istio.yaml | head -n 1 | awk -F: '{print $1}'`,`cat 03-istio.yaml|wc -l`p" > 04-istio.yaml
```

New upstream version:
```console
ISTIO_VERSION=1.12.5
curl https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/manifests/charts/base/crds/crd-all.gen.yaml -o 01-istio-${ISTIO_VERSION}.yaml
diff 01-istio.yaml 01-istio-${ISTIO_VERSION}.yaml
```

With KUBECONFIG pointing to a cluster without istio crds, i.e. no seed cluster:
```console
ISTIO_VERSION=1.12.5
kubectl apply -f 01-istio-${ISTIO_VERSION}.yaml
curl  https://istio-release.storage.googleapis.com/charts/gateway-${ISTIO_VERSION}.tgz -o gateway-${ISTIO_VERSION}.tgz
curl  https://istio-release.storage.googleapis.com/charts/istiod-${ISTIO_VERSION}.tgz -o istiod-${ISTIO_VERSION}.tgz
helm install istiod istiod-${ISTIO_VERSION}.tgz -n istio-system --dry-run > 02-istio-${ISTIO_VERSION}.yaml
helm install istio-ingress gateway-${ISTIO_VERSION}.tgz -n istio-ingress --dry-run > 03-istio-${ISTIO_VERSION}.yaml
cat 02-istio-${ISTIO_VERSION}.yaml | sed -n "`grep -n EnvoyFilter 02-istio-${ISTIO_VERSION}.yaml | head -n 1 | awk -F: '{print $1}'`,`cat 02-istio-${ISTIO_VERSION}.yaml|wc -l`p" > 04-istio-${ISTIO_VERSION}.yaml
diff 02-istio.yaml 02-istio-${ISTIO_VERSION}.yaml
diff 03-istio.yaml 03-istio-${ISTIO_VERSION}.yaml
diff 04-istio.yaml 04-istio-${ISTIO_VERSION}.yaml
```
