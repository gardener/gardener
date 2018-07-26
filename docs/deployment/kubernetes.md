# Deploying the Gardener into a Kubernetes cluster
As already mentioned, the Gardener is designed to run as API server extension in an existing Kubernetes cluster. In order to deploy it, you require valid [Kubernetes manifests](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/). We use [Helm](http://helm.sh/) in order to generate these manifests. The respective Helm chart for the Gardener can be found [here](../../charts/gardener). In order to deploy it, execute


```bash
$ helm install charts/gardener --name gardener --wait
```

You can configure the Helm chart by modifying the [allowed configuration values](../../charts/gardener/values.yaml). Please note that all resources and deployments need to be created in the `garden` namespace (not overrideable).

:warning: The Seed Kubernetes clusters need to have a `nginx-ingress-conroller` deployed to make the Gardener work properly. Moreover, there should exist a DNS record `*.ingress.<SEED-CLUSTER-DOMAIN>` where `<SEED-CLUSTER-DOMAIN>` is the value of the `domain` field of [a Seed cluster resource](../../example/50-seed-aws.yaml).
