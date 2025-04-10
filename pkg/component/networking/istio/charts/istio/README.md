# Istio update to new Istio version

Render current version in gardener repository:
```console
helm template istio pkg/component/networking/istio/charts/istio/istio-crds -n istio-system > istio-crds.yaml

helm template istio pkg/component/networking/istio/charts/istio/istio-istiod -n istio-system \
--set=deployNamespace=true > istio-istiod.yaml

helm template istio pkg/component/networking/istio/charts/istio/istio-ingress -n istio-ingress \
--set=deployNamespace=true \
--set=serviceType=ClusterIP \
--set=portsNames.status=status-port > istio-ingress.yaml
```

Clone istio github repository and checkout desired release tag:
```console
ISTIO_VERSION=1.25.1
git clone https://github.com/istio/istio.git
cd istio
git checkout $ISTIO_VERSION
```

Compare crds:
```console
diff istio-crds.yaml istio/${ISTIO_VERSION}/manifests/charts/base/files/crd-all.gen.yaml
```

Render new version in istio/istio repository:
```console
helm template manifests/charts/istio-control/istio-discovery/ -n istio-system \
--set=global.omitSidecarInjectorConfigMap=true \
--set=global.configValidation=true \
--set=pilot.autoscaleEnabled=false \
--set=global.operatorManageWebhooks=true > istio-istiod-${ISTIO_VERSION}.yaml

helm template manifests/charts/gateways/istio-ingress -n istio-ingress > istio-ingress-${ISTIO_VERSION}.yaml
```

Compare charts:
```console
diff istio-istiod.yaml istio-istiod-${ISTIO_VERSION}.yaml
diff istio-ingress.yaml istio-ingress-${ISTIO_VERSION}.yaml
```
