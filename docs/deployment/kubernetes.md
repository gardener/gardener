# Deploying the Gardener into a Kubernetes cluster
As already mentioned, the Gardener is designed to run as API server extension in an existing Kubernetes cluster. In order to deploy it, you require valid [Kubernetes manifests](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/). We use [Helm](http://helm.sh/) in order to generate these manifests. The respective Helm chart for the Gardener can be found [here](../../charts/gardener). In order to deploy it, execute

```bash
# Check https://github.com/gardener/gardener/releases for current available releases.
RELEASE=0.19.1

# Since master can have breaking changes after last release, checkout the tag for your release
git checkout $RELEASE

cat <<EOF > gardener.values
global:
  apiserver:
    image:
      tag: $RELEASE
  controller:
    image:
      tag: $RELEASE
EOF

helm install charts/gardener \
  --namespace garden \
  --name gardener \
  -f gardener.values \
  --wait
```

You can check the [default values file](../../charts/gardener/values.yaml) for other configuration values. Please note that all resources and deployments need to be created in the `garden` namespace (not overrideable).

:warning: The Seed Kubernetes clusters need to have a `nginx-ingress-controller` deployed to make the Gardener work properly. Moreover, there should exist a DNS record `*.ingress.<SEED-CLUSTER-DOMAIN>` where `<SEED-CLUSTER-DOMAIN>` is the value of the `ingressDomain` field of [a Seed cluster resource](../../example/50-seed-aws.yaml).
