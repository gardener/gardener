# Deploying the Gardener and a Seed into an AKS cluster

This document demonstrates how to install Gardener into an existing
AKS cluster. We'll use a single cluster to host both Gardener and a Seed to the
same cluster for the sake of simplicity .

Please note that this document is to provide you an example
installation and is not to be used in a production environment since
there are some certificates hardcoded, non-HA and non-TLS-enabled etcd
setup.

# High Level Overview

In this example we'll follow these steps to create a Seed cluster on AKS:
- [Deploying the Gardener and a Seed into an AKS cluster](#deploying-the-gardener-and-a-seed-into-an-aks-cluster)
- [High Level Overview](#high-level-overview)
- [Prerequisites](#prerequisites)
  - [AWS credentials for Route 53 Hosted Zone](#aws-credentials-for-route-53-hosted-zone)
  - [Deploy AKS cluster](#deploy-aks-cluster)
    - [Initialize Helm on the Cluster](#initialize-helm-on-the-cluster)
    - [Deploy stable/nginx-ingress chart to AKS](#deploy-stablenginx-ingress-chart-to-aks)
    - [Create wildcard DNS record for the ingress](#create-wildcard-dns-record-for-the-ingress)
  - [Create Azure Service Principle to get Azure credentials](#create-azure-service-principle-to-get-azure-credentials)
  - [Install gardenctl](#install-gardenctl)
- [Install Gardener](#install-gardener)
  - [Create garden namespace](#create-garden-namespace)
  - [Deploy etcd](#deploy-etcd)
  - [Deploy Gardener Helm Chart](#deploy-gardener-helm-chart)
- [Create a CloudProfile](#create-a-cloudprofile)
- [Define Seed cluster in Gardener](#define-seed-cluster-in-gardener)
  - [Create the Seed resource definition with its Secret](#create-the-seed-resource-definition-with-its-secret)
- [Create a Shoot cluster](#create-a-shoot-cluster)
  - [Create a Project (namespace) for Shoots](#create-a-project-namespace-for-shoots)
  - [Create a SecretBinding and related Secret](#create-a-secretbinding-and-related-secret)
  - [Create the Shoot resource](#create-the-shoot-resource)
    - [Cluster Resources After Shoot is Created](#cluster-resources-after-shoot-is-created)
    - [Troubleshooting Shoot Creation Issues](#troubleshooting-shoot-creation-issues)
- [Access Shoot cluster](#access-shoot-cluster)
- [Delete Shoot cluster](#delete-shoot-cluster)

# Prerequisites

Summary of prerequisites:
- An Azure AKS cluster with:
  - Helm initialized,
  - an ingress controller deployed,
  - a wildcard DNS record pointing the ingress,
  - `az` command line client configured for Azure subscription,
- An Azure service principle to provide Azure credentials to Gardener,
- A Route53 Hosted Zone and AWS account credentials with permissions on that Route53 Zone,
  - `aws` command line client configured for this account,
- `gardenctl` command line client configured for the AKS cluster's kubeconfig

**Note**: Gardener doesn't have support for Azure DNS yet (see
[#494](https://github.com/gardener/gardener/issues/494)). So, we use a Route53 Hosted Zone
even if we are deploying on Azure.

## AWS credentials for Route 53 Hosted Zone

You need to provide credentials for AWS with permission to access Route53
Hosted Zone. In this example we'll assume your domain for the Hosted
Zone is `.your.domain.here`.

```
HOSTED_ZONE_ID=        # place your AWS Route53 hostedZoneID here
```

Create an AWS user, define policy to allow permission for the Hosted
Zone and note the `hostedZoneID`, `accessKeyID` and `secretAccessKey`
for later use.

## Deploy AKS cluster

Here you can find a summary for creating an AKS cluster, if you already
have one, skip this step.

```
az group create --name garden-1 --location eastus
az aks create --resource-group garden-1 --name garden-1 \
  --kubernetes-version 1.11.5 \
  --node-count 2 --node-vm-size Standard_DS4_v2 \
  --generate-ssh-keys
az aks get-credentials --resource-group garden-1 --name garden-1 --admin
```

### Initialize Helm on the Cluster

Since RBAC is enabled by default we need to deploy helm with an RBAC config.

```
kubectl apply -f https://raw.githubusercontent.com/Azure/helm-charts/master/docs/prerequisities/helm-rbac-config.yaml
helm init --service-account tiller
```

### Deploy stable/nginx-ingress chart to AKS

At the moment the `Ingress` resources created by the Gardener are
expecting the nginx-ingress style annotations to work.

```
helm upgrade --install \
  --namespace kube-system \
  nginx-ingress stable/nginx-ingress
```

### Create wildcard DNS record for the ingress

You need to pick a wildcard subdomain matching your Route53 Hosted
Zone here. This ingress wildcard record is supposed to be part of the Seed
cluster rather than Gardener cluster, in our example we'll use
`*.seed-1.your.domain.here`.

Assuming you have the AWS cli for your Route53 Hosted Zone is
configured on your local, here we'll create the wildcard DNS record
using the [`awless`](http://awless.io/). You can also use the AWS
console or any other tool of your choice to create the wildcard
record:

```
HOSTED_ZONE_DOMAIN=$(aws route53 get-hosted-zone --id /hostedzone/${HOSTED_ZONE_ID:?"HOSTED_ZONE_ID is missing"} --query 'HostedZone.Name' --output text)
INGRESS_DOMAIN="seed-1.${HOSTED_ZONE_DOMAIN%%.}"
# Get LB IP address from `kubectl -n kube-system get svc shared-ingress-nginx-ingress-controller`
LB_IP=$(kubectl -n kube-system get svc nginx-ingress-controller --template '{{(index .status.loadBalancer.ingress 0).ip}}')
awless create record \
  zone=$HOSTED_ZONE_ID \
  name="*.$INGRESS_DOMAIN" \
  value=$LB_IP \
  type=A \
  ttl=300
```

## Create Azure Service Principle to get Azure credentials

We need `client_id` and `client_secret` to allow Gardener to reach
Azure services, we can generate a pair by creating a Service Principle
on Azure:

```
$ az ad sp create-for-rbac --role="Contributor"
Retrying role assignment creation: 1/36
{
  "appId": "xxxxxx-xxx-xxxx-xxx-xxxxx",     #az_client_id
  "displayName": "azure-cli-2018-05-23-16-15-49",
  "name": "http://azure-cli-2018-05-23-16-15-49",
  "password": "xxxxxx-xxx-xxxx-xxx-xxxxx",  #az_client_secret
  "tenant": "xxxxxx-xxx-xxxx-xxx-xxxxx"     #az_tenant_id
}
```

Let's define some env variables for later use

```
CLIENT_ID=       # place your Azure Service Principal appId
CLIENT_SECRET=   # place your Azure Service Principal password here
```

## Install gardenctl

In this example we'll be using `gardenctl` to interact with
Gardener. You can install `gardenctl` following instruction in its
repo: https://github.com/gardener/gardenctl

Here is a sample configuration for gardenctl:

```
$ cat ~/.garden/config
gardenClusters:
- name: dev
  kubeConfig: ~/.kube/config
```

# Install Gardener

## Create garden namespace

This is where we deploy Gardener components.

```
kubectl apply -f example/00-namespace-garden.yaml
```

## Deploy etcd

Since Gardener is an extension API Server, it can share the etcd
backing native Kubernetes cluster's API Server, and hence explicit etcd
installation is optional. But in our case we have no access to the
control plane components of the AKS cluster and we have to deploy our
own etcd ourselves for Gardener. Lets deploy an etcd using the
[gardener/etcd-backup-restore](https://github.com/gardener/etcd-backup-restore)
project, which is also used by the Gardener for Shoot control plane.

```
# pull the etcd-backup-restore
git clone https://github.com/gardener/etcd-backup-restore.git

# deploy etcd
helm upgrade --install \
  --namespace garden \
  etcd etcd-backup-restore/chart \
  --set tls=
```

**Note**: This etcd installation doesn't provide HA. But etcd will be
auto recovered by the Deployment. This could be sufficient for some
deployments but may not be suitable for production usage. Also note
that this etcd is not deployed with TLS enabled and doesn't use
certificates for authentication.

Check etcd pod's health, it should have `READY:2/2` and `STATUS:Running`:

```
$ kubectl -n garden get pods
NAME              READY   STATUS    RESTARTS   AGE
etcd-for-test-0   2/2     Running   0          1m
```

## Deploy Gardener Helm Chart

Check (current releases)[https://github.com/gardener/gardener/releases] and
pick a suitable one to install.

```
GARDENER_RELEASE=0.17.1
```

gardener-controller-manager will need to maintain some DNS records for Seed.
So, you need to provide Route53 credentials in the values.yaml file:
* **global.controller.internalDomain.hostedZoneID**
* **global.controller.internalDomain.domain**: Here pick a subdomain for your
  Gardener to maintain DNS records for your Shoot clusters. This domain has
  to be within your Route53 Hosted Zone. e.g. `garden-1.your.domain.here`
* **global.controller.internalDomain.credentials**
* **global.controller.internalDomain.secretAccessKey**

```
HOSTED_ZONE_DOMAIN=$(
  aws route53 get-hosted-zone \
    --id /hostedzone/${HOSTED_ZONE_ID:?"HOSTED_ZONE_ID is missing"} \
    --query 'HostedZone.Name' \
    --output text)
HOSTED_ZONE_DOMAIN=${HOSTED_ZONE_DOMAIN%%.}
GARDENER_DOMAIN="garden-1.${HOSTED_ZONE_DOMAIN}"
ACCESS_KEY_ID=$(aws configure get aws_access_key_id)
SECRET_ACCESS_KEY=$(aws configure get aws_secret_access_key)
cat <<EOF > gardener-values.yaml
global:
  apiserver:
    image:
      tag: ${GARDENER_RELEASE:?"GARDENER_RELEASE is missing"}
    etcd:
      servers: http://etcd-for-test-client:2379
      useSidecar: false
  controller:
    image:
      tag: ${GARDENER_RELEASE:?"GARDENER_RELEASE is missing"}
    internalDomain:
      provider: aws-route53
      hostedZoneID: ${HOSTED_ZONE_ID}
      domain: ${HOSTED_ZONE_DOMAIN}
      credentials:
        AWS_ACCESS_KEY_ID: ${ACCESS_KEY_ID}
        AWS_SECRET_ACCESS_KEY: ${SECRET_ACCESS_KEY}
EOF
```

After creating the `gardener-values.yaml` file, since chart definition in
master branch can have breaking changes after the release, checkout the
gardener tag for that release, and run:

```
git checkout ${GARDENER_RELEASE:?"GARDENER_RELEASE is missing"}
helm upgrade --install \
  --namespace garden \
  garden charts/gardener \
  -f charts/gardener/local-values.yaml \
  -f gardener-values.yaml
```

Validate the Gardener is deployed:

```
helm status garden  # Wait for `STATUS: DEPLOYED`

kubectl -n garden get deploy,pod -l app=gardener

# Better if you leave two terminals open in for below commands, and
# keep an eye on whats going on behind the scenes as you create/delete
# Gardener specific resources (Seed, CloudProfile, SecretBinding, Shoot).
kubectl -n garden logs -f deployment/gardener-apiserver           # confirm no issues
kubectl -n garden logs -f deployment/gardener-controller-manager  # confirm no issues, except some "Failed to list *v1beta1..." messages
```

**Note**: This is not meant to be used in production. You may not want
to use `apiserver.insecureSkipTLSVerify=true`, the hardcoded apiserver
certificates, and insecure (non-tls enabled) etcd. But for the sake of
keeping this example simple you can just keep those values as they
are.

# Create a CloudProfile

We need to create a CloudProfile to be referred from the Shoot
([`example/30-cloudprofile-azure.yaml`](../../example/30-cloudprofile-azure.yaml)):

```
kubectl apply -f example/30-cloudprofile-azure.yaml
```

Validate that CloudProfile is created:

```
kubectl describe -f example/30-cloudprofile-azure.yaml
```

# Define Seed cluster in Gardener

In our setup we'll use the cluster for Gardener also as a Seed, this
saves us from creating a new Kubernetes cluster. But you can also
create an explicit cluster for the Seed. Seed cluster can also be
placed into any other cloud provider or on prem. But keep in mind that
below steps may differ if you use a different cluster for seed.

Currently, a Seed cluster is just a Kubeconfig for the Gardener. The
seed cluster could have been created by any tool, Gardener only cares
about having a valid Kubeconfig to talk to its API.

## Create the Seed resource definition with its Secret

Lets start with the required seed secret first. Here we need to
provide it's cloud provider credentials and kubeconfig in the seed
secret. Update
[`example/40-secret-seed-azure.yaml`](../../example/40-secret-seed-azure.yaml)
and place the secrets for your environment:
* **data.subscriptionID**: you can learn this one with `az account show`
* **data.tenantID**: from `az ad sp create-for-rbac` output as you can see above
* **data.clientID**: from `az ad sp create-for-rbac` output as you can see above
* **data.clientSecret**: from `az ad sp create-for-rbac` output as you can see above
* **data.kubeconfig**: you can get this one with `az aks get-credentials --resource-group garden-1 --name garden-1 -f - | base64`)

**Note**: All of the above values must be base64 encoded. If you skip this it will hurt you later.

```
SUBSCRIPTION_ID=$(az account list -o json | jq -r '.[] | select(.isDefault == true) | .id')
TENANT_ID=$(az account show -o tsv --query 'tenantId')
KUBECONFIG_FOR_SEED_CLUSTER=$(az aks get-credentials --resource-group garden-1 --name garden-1 -f -)
sed -i \
  -e "s@base64(uuid-of-subscription)@$(echo $SUBSCRIPTION_ID | tr -d '\n' | base64)@" \
  -e "s@base64(uuid-of-tenant)@$(echo "$TENANT_ID" | tr -d '\n' | base64)@" \
  -e "s@base64(uuid-of-client)@$(echo "${CLIENT_ID:?"CLIENT_ID is missing"}" | tr -d '\n' | base64)@" \
  -e "s@base64(client-secret)@$(echo "${CLIENT_SECRET:?"CLIENT_SECRET is missing"}" | tr -d '\n' | base64)@" \
  -e "s@base64(kubeconfig-for-seed-cluster)@$(echo "$KUBECONFIG_FOR_SEED_CLUSTER" | base64 -w 0)@" \
  example/40-secret-seed-azure.yaml
```

After updating the fields, create the Seed secret:
```
kubectl apply -f example/40-secret-seed-azure.yaml
```

Before creating Seed, we need to update the
[`example/50-seed-azure.yaml`](../../example/50-seed-azure.yaml) file and
update:
* **spec.networks**: IP ranges used in your AKS cluster.
* **spec.ingressDomain**: Place here the wildcard domain you have for
  the ingress controller (we created this record in prerequisites).
  Gardener doesn't create this DNS records but assumes its created
  ahead of time, Seed clusters are not provisioned by Gardener.
* **spec.cloud.region**: `eastus` (the region of the existing AKS cluster)

```
HOSTED_ZONE_DOMAIN=$(aws route53 get-hosted-zone --id /hostedzone/${HOSTED_ZONE_ID:?"HOSTED_ZONE_ID is missing"} --query 'HostedZone.Name' --output text)
INGRESS_DOMAIN="seed-1.${HOSTED_ZONE_DOMAIN%%.}"
# discover AKS CIDRs
NODE_CIDR=$(az network vnet list -g MC_garden-1_garden-1_eastus -o json | jq -r '.[] | .subnets[] | .addressPrefix')
POD_CIDR=$(kubectl -n kube-system get daemonset/kube-proxy -o yaml | grep cluster-cidr= | grep -v annotations | cut -d = -f2)
SERVICE_CIDR=10.0.0.0/16  # This one is hardcoded for now, not easy to discover
sed -i \
  -e "s/ingressDomain: dev.azure.seed.example.com/ingressDomain: $INGRESS_DOMAIN/" \
  -e "s/region: westeurope/region: eastus/" \
  -e "s@nodes: 10.240.0.0/16@nodes: $NODE_CIDR@" \
  -e "s@pods: 10.241.128.0/17@pods: $POD_CIDR@" \
  -e "s@services: 10.241.0.0/17@services: $SERVICE_CIDR@" \
  example/50-seed-azure.yaml
```

Now we are ready to create the seed:
```
kubectl apply -f example/50-seed-azure.yaml
```

Check the logs in gardener-controller-manager and also wait for seed
to be `Ready: True`. This means gardener-controller-manager is able to
reach the Seed cluster with the credentials you provide.

```
$ gardenctl target garden dev
KUBECONFIG=/Users/user/.kube/config
$ kubectl get seed azure
NAME    CLOUDPROFILE   REGION   INGRESS DOMAIN            AVAILABLE   AGE
azure   azure          eastus   seed-1.your.domain.here   True        1m
$ gardenctl ls seeds
seeds:
- seed: azure
```

If something goes wrong verify that you provided right credentials,
and base64 encoded strings of those in the secret. Also check the
status field in the Seed resource and gardener-controller-manager
logs:

```
$ kubectl get seed azure -o json | jq .status
{
  "conditions": [
    {
      "lastTransitionTime": "2018-05-31T14:56:49Z",
      "message": "all checks passed",
      "reason": "Passed",
      "status": "True",
      "type": "Available"
    }
  ]
}
```

# Create a Shoot cluster

## Create a Project (namespace) for Shoots

In this step we create a namespace in Gardener cluster to keep Shoot
resource definitions. A `project` in Gardener terminology is simply a
namespace that holds group of Shoots, during this example we'll deploy
a single Shoot. (Mind the extra labels defined in
[example/00-namespace-garden-dev.yaml](../../example/00-namespace-garden-dev.yaml)).

```
kubectl apply -f example/05-project-dev.yaml
```

You can check the projects via `gardenctl`:

```
$ gardenctl target garden dev
$ kubectl get project dev
NAME   NAMESPACE    STATUS   OWNER                  CREATOR   AGE
dev    garden-dev   Ready    john.doe@example.com   client    1m
$ kubectl get ns garden-dev
NAME          STATUS   AGE
garden-dev    Active   1m
$ gardenctl ls projects
projects:
- project: garden-dev
```

## Create a SecretBinding and related Secret

We'll use same Azure credentials with
[`example/40-secret-seed-azure.yaml`](../../example/40-secret-seed-azure.yaml),
this is due to the fact that we use the same Azure Subscription for
the Shoot and Seed clusters. Differently from the Seed secret, in this
one we don't need to provide `kubeconfig` since the Shoot cluster will
be provisioned by Gardener, and we need to provide credentials for
Route53 DNS records management.

Update
[`example/70-secret-cloudprovider-azure.yaml`](../../example/70-secret-cloudprovider-azure.yaml)
and place the secrets for your environment:
* **data.subscriptionID**: you can learn this one with `az account show`
* **data.tenantID**: from `az ad sp create-for-rbac` output as you can see above
* **data.clientID**: from `az ad sp create-for-rbac` output as you can see above
* **data.clientSecret**: from `az ad sp create-for-rbac` output as you can see above
* **data.accessKeyID**: You need to add this field for Route53 records to be updated.
* **data.secretAccessKey**: You need to add this field for Route53 records to be updated.

**Note**: All of the above values must be base64 encoded. If you skip this it will hurt you later.

```
SUBSCRIPTION_ID=$(az account list -o json | jq -r '.[] | select(.isDefault == true) | .id')
TENANT_ID=$(az account show -o tsv --query 'tenantId')
ACCESS_KEY_ID=$(aws configure get aws_access_key_id)
SECRET_ACCESS_KEY=$(aws configure get aws_secret_access_key)
sed -i \
  -e "s@base64(uuid-of-subscription)@$(echo $SUBSCRIPTION_ID | tr -d '\n' | base64)@" \
  -e "s@base64(uuid-of-tenant)@$(echo "$TENANT_ID" | tr -d '\n' | base64)@" \
  -e "s@base64(uuid-of-client)@$(echo "${CLIENT_ID:?"CLIENT_ID is missing"}" | tr -d '\n' | base64)@" \
  -e "s@base64(client-secret)@$(echo "${CLIENT_SECRET:?"CLIENT_SECRET is missing"}" | tr -d '\n' | base64)@" \
  -e "\$a\ \ accessKeyID: $(echo $ACCESS_KEY_ID | tr -d '\n' | base64 )" \
  -e "\$a\ \ secretAccessKey: $(echo $SECRET_ACCESS_KEY | tr -d '\n' | base64 )" \
  example/70-secret-cloudprovider-azure.yaml
```

After updating the fields, create the cloud provider secret:

```
kubectl apply -f example/70-secret-cloudprovider-azure.yaml
```

And create the SecretBinding resource to allow Gardener use that
secret
([`example/80-secretbinding-cloudprovider-azure.yaml`](../../example/80-secretbinding-cloudprovider-azure.yaml)):

```
sed -i \
  -e 's/# namespace: .*/  namespace: garden-dev/' \
  example/80-secretbinding-cloudprovider-azure.yaml
kubectl apply -f example/80-secretbinding-cloudprovider-azure.yaml
```

Check the logs in gardener-controller-manager, there should not be any
problems reported.

## Create the Shoot resource

Update the fields in [`example/90-shoot-azure.yaml`](../../example/90-shoot-azure.yaml):
* **spec.cloud.region**: `eastus` (this must match the seed cluster's region)
* **spec.dns.domain**: This is used to specify the base domain for
  your api (and other in the future) endpoint(s). For example when
  `johndoe-azure.garden-dev.your.domain.here` is used as a value, then your
  apiserver is available at `api.johndoe-azure.garden-dev.your.domain.here`
* **spec.dns.hostedZoneID**: This field doesn't exist in the example
  you need to add this field and place the Route53 Hosted Zone ID.
* **spec.addons.kube-lego.email**: This is the email address used when
  using kube-lego. See
  [kube-lego Environment Variables](https://github.com/jetstack/kube-lego#environment-variables)

```
HOSTED_ZONE_DOMAIN=$(aws route53 get-hosted-zone --id /hostedzone/${HOSTED_ZONE_ID:?"HOSTED_ZONE_ID is missing"} --query 'HostedZone.Name' --output text)
SHOOT_DOMAIN="johndoe-azure.garden-dev.${HOSTED_ZONE_DOMAIN%%.}"
KUBE_LEGO_EMAIL=$(git config user.email)
sed -i \
  -e "s/region: westeurope/region: eastus/" \
  -e "s/domain: johndoe-azure.garden-dev.example.com/domain: $SHOOT_DOMAIN/" \
  -e "/domain:/a\ \ \ \ hostedZoneID: $HOSTED_ZONE_ID" \
  -e "s/email: john.doe@example.com/email: $KUBE_LEGO_EMAIL/" \
  example/90-shoot-azure.yaml
```

And let's create the Shoot resource:
```
kubectl apply -f example/90-shoot-azure.yaml
```

After creating the Shoot resource, gardener-controller-manager will
pick it up and start provisioning the Shoot cluster.

```
$ kubectl get -f example/90-shoot-azure.yaml
NAME            CLOUDPROFILE   VERSION   SEED    DOMAIN                                      OPERATION    PROGRESS   APISERVER   CONTROL     NODES       SYSTEM      AGE
johndoe-azure   azure          1.12.3    azure   johndoe-azure.garden-dev.your.domain.here   Processing   15         <unknown>   <unknown>   <unknown>   <unknown>   16s
```

Follow the logs in your console with gardener-controller-manager,
starting like below you'll see plenty of `Waiting` and `Executing`,
etc. logs and many tasks will keep repeating:

```
time="2018-06-09T07:35:45Z" level=info msg="[SHOOT RECONCILE] garden-dev/johndoe-azure"
time="2018-06-09T07:35:46Z" level=info msg="Starting flow Shoot cluster creation" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:46Z" level=info msg="Executing (*Botanist).DeployExternalDomainDNSRecord" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:46Z" level=info msg="Executing (*Botanist).DeployNamespace" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:46Z" level=info msg="Executing (*Botanist).DeployKubeAPIServerService" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:46Z" level=info msg="Executing (*Botanist).DeployBackupNamespaceFromShoot" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:46Z" level=info msg="Waiting for Terraform validation Pod 'johndoe-azure.external-dns.tf-pod-d8f66' to be completed..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:51Z" level=info msg="Waiting for Terraform validation Pod 'johndoe-azure.external-dns.tf-pod-d8f66' to be completed..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:51Z" level=info msg="Executing (*Botanist).MoveBackupTerraformResources" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:52Z" level=info msg="Executing (*Botanist).WaitUntilKubeAPIServerServiceIsReady" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:52Z" level=info msg="Waiting until the kube-apiserver service is ready..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:52Z" level=info msg="Executing (*Botanist).DeployBackupInfrastructure" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:52Z" level=info msg="Executing (*Botanist).WaitUntilBackupInfrastructureReconciled" opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:52Z" level=info msg="Waiting until the backup-infrastructure has been reconciled in the Garden cluster..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:56Z" level=info msg="Waiting for Terraform validation Pod 'johndoe-azure.external-dns.tf-pod-d8f66' to be completed..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:57Z" level=info msg="Waiting until the kube-apiserver service is ready..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:35:57Z" level=info msg="Waiting until the backup-infrastructure has been reconciled in the Garden cluster..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:36:01Z" level=info msg="Waiting for Terraform validation Pod 'johndoe-azure.external-dns.tf-pod-d8f66' to be completed..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:36:02Z" level=info msg="Waiting until the kube-apiserver service is ready..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
time="2018-06-09T07:36:02Z" level=info msg="Waiting until the backup-infrastructure has been reconciled in the Garden cluster..." opid=VIBBBGFx shoot=garden-dev/johndoe-azure
...
```

At this stage you should be waiting for a while until the Shoot
cluster is provisioned and initial resources are deployed.

During the provisioning you can also check output of these commands to
have a better understanding about what's going on in the seed cluster:

```
$ gardenctl ls shoots
projects:
- project: garden-dev
  shoots:
  - johndoe-azure
$ gardenctl ls issues
issues:
- project: garden-dev
  seed: azure
  shoot: johndoe-azure
  health: Unknown
  status:
    lastOperation:
      description: Executing DeployKubeAddonManager, ReconcileMachines.
      lastUpdateTime: 2018-06-09 08:40:20 +0100 IST
      progress: 74
      state: Processing
      type: Create
$ kubectl -n garden-dev get shoot johndoe-azure
NAMESPACE    NAME            SEED      DOMAIN                                       VERSION   CONTROL   NODES     SYSTEM    LATEST
garden-dev   johndoe-azure   azure     johndoe-azure.garden-dev.your.domain.here   1.10.1    True      True      True      Succeeded
$ kubectl -n garden-dev describe shoot johndoe-azure
...
Events:
  Type     Reason          Age   From                         Message
  ----     ------          ----  ----                         -------
  Normal   Reconciling     1h    gardener-controller-manager  [BrXWiztO] Reconciling Shoot cluster state
  Normal   Reconciling     59m   gardener-controller-manager  [rBFsfwU5] Reconciling Shoot cluster state
  Normal   Reconciling     59m   gardener-controller-manager  [2HAbm45D] Reconciling Shoot cluster state
  Warning  ReconcileError  48m   gardener-controller-manager  [2HAbm45D] Failed to reconcile Shoot cluster state: Errors occurred during flow execution: '(*Botanist).EnsureIngressDNSRecord' returned '`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created (is your quota limit exceeded/reached?)'
  Normal   Reconciling     48m   gardener-controller-manager  [S1QA0ksz] Reconciling Shoot cluster state
  Normal   Reconciling     47m   gardener-controller-manager  [lvcSKy1Q] Reconciling Shoot cluster state
  Normal   Reconciling     47m   gardener-controller-manager  [MddMyk8W] Reconciling Shoot cluster state
  Normal   Reconciling     47m   gardener-controller-manager  [XDAAWABd] Reconciling Shoot cluster state
  Normal   Reconciling     46m   gardener-controller-manager  [6HYH9Psz] Reconciling Shoot cluster state
  Normal   Reconciling     46m   gardener-controller-manager  [rhL38ym4] Reconciling Shoot cluster state
  Warning  ReconcileError  35m   gardener-controller-manager  [rhL38ym4] Failed to reconcile Shoot cluster state: Errors occurred during flow execution: '(*Botanist).EnsureIngressDNSRecord' returned '`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created (is your quota limit exceeded/reached?)'
  Normal   Reconciling     35m   gardener-controller-manager  [BOt4Nvso] Reconciling Shoot cluster state
  Normal   Reconciling     35m   gardener-controller-manager  [JPtmXmxD] Reconciling Shoot cluster state
  Normal   Reconciling     34m   gardener-controller-manager  [ldHsVA6G] Reconciling Shoot cluster state
  Normal   Reconciled      31m   gardener-controller-manager  [ldHsVA6G] Reconciled Shoot cluster state
  Normal   Reconciling     26m   gardener-controller-manager  [yBh2IBOF] Reconciling Shoot cluster state
  Normal   Reconciled      24m   gardener-controller-manager  [yBh2IBOF] Reconciled Shoot cluster state
  Normal   Reconciling     16m   gardener-controller-manager  [bqmFtHUA] Reconciling Shoot cluster state
  Normal   Reconciled      14m   gardener-controller-manager  [bqmFtHUA] Reconciled Shoot cluster state
  Normal   Reconciling     6m    gardener-controller-manager  [7QgHE5CH] Reconciling Shoot cluster state
  Normal   Reconciled      3m    gardener-controller-manager  [7QgHE5CH] Reconciled Shoot cluster state
```

Check Shoot cluster:

```
$ gardenctl target garden dev
KUBECONFIG=/Users/user/.kube/config
$ gardenctl target project garden-dev
$ gardenctl target shoot johndoe-azure
KUBECONFIG=/Users/user/.garden/cache/projects/garden-dev/johndoe-azure/kubeconfig.yaml
$ gardenctl kubectl cluster-info
Kubernetes master is running at https://api.johndoe-azure.garden-dev.your.domain.here
CoreDNS is running at https://api.johndoe-azure.garden-dev.your.domain.here/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
kubernetes-dashboard is running at https://api.johndoe-azure.garden-dev.your.domain.here/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy

To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'.
```

### Cluster Resources After Shoot is Created

After the Shoot has been created the summary of the resources in the
AKS cluster handled by Gardener will be something like this:

```
non-namespaced resources
  CloudProfile: azure
  Project: dev
  Namespace: garden-dev
  Seed: azure  # cloud.profile:azure, cloud.region:eastus, secretRef.name:seed-azure, secretRef.namespace: garden

Namespace: garden
  Secret: seed-azure  # aks credentials, kubeconfig
  # No other resources with any kind handled by Gardener
  # Gardener components as well lives in this namespace

Namespace: garden-dev  # maps to "project:dev" in Gardener
  Secret: core-azure         # credentials for aks + aws (for route53)
  SecretBinding: core-azure  # secretRef.name:core-azure
  Shoot: johndoe-azure       # seed:azure, secretBindingRef.name:core-azure

Namespace: shoot--dev--johndoe-azure
  # These are automatically created once Shoot resource is created
  AzureMachineClass: shoot--dev--johndoe-azure-cpu-worker-8506a
  MachineDeployment: shoot--dev--johndoe-azure-cpu-worker
  MachineSet: shoot--dev--johndoe-azure-cpu-worker-849bbbf75
  Machine: shoot--dev--johndoe-azure-cpu-worker-849bbbf75-b42vh
  BackupInfra: shoot--dev--johndoe-azure--c1b3b  # seed:azure, shootUID: shoot.status.UID.
  # Many other resources created as part of shoot cluster,
  #   but only above ones are handled by Gardener

Namespace: backup--shoot--dev--johndoe-azure--c1b3b
  # Secrets and configMap having info related to backup infrastructure
  #   are created by Gardener.
```

### Troubleshooting Shoot Creation Issues

For any issue happening during Shoot provisioning, you can consult the
gardener-controller-manager logs, or the state in the shoot resource,
`gardenctl` also provides a command to check Shoot cluster states:

```
# check gardener-controller-manager logs
kubectl -n garden logs -f deployment/gardener-controller-manager
# kubectl describe can provide you a human readable output of
# same information in below gardenctl command.
kubectl -n garden-dev describe shoot johndoe-azure
# also try cheking the machine-controller-manager logs of the shoot
kubectl logs -n shoot--dev--johndoe-azure deployment/machine-controller-manager
```

With `gardenctl`:

```
$ gardenctl ls issues
issues:
- project: garden-dev
  seed: azure
  shoot:
  health: Ready
  status: johndoe-azure
    lastError: "Failed to reconcile Shoot cluster state: Errors occurred during flow
      execution: '(*Botanist).DeployExternalDomainDNSRecord' returned 'Terraform execution
	  ...
    lastOperation:
      description: "Failed to reconcile Shoot cluster state: Errors occurred during
        flow execution: '(*Botanist).DeployExternalDomainDNSRecord' returned 'Terraform
		...
      lastUpdateTime: 2018-06-03 09:48:00 +0100 IST
      progress: 100
      state: Failed
      type: Reconcile
```

# Access Shoot cluster

The `gardenctl` tool provides a convenient wrapper to operate on both
cluster and cloud providers, here are some commands you can run

```
# select target shoot cluster
gardenctl ls gardens
gardenctl target garden dev
gardenctl ls projects
gardenctl target shoot johndoe-azure

# issue Azure client (az) commands on target shoot
gardenctl az aks list

# issue kubectl commands on target shoot
gardenctl kubectl -- version --short  # '--' is required if you want to
                                      # pass any args starting with '-'

# open prometheus, alertmanager, grafana without having to find
# the user/pass for each
gardenctl show prometheus
gardenctl show grafana
gardenctl show alertmanager
```

Easiest way to obtain `kubeconfig` of the shoot cluster:

```
$ gardenctl target shoot johndoe-azure
KUBECONFIG=/Users/user/.garden/cache/projects/garden-dev/johndoe-azure/kubeconfig.yaml
$ export KUBECONFIG=/Users/user/.garden/cache/projects/garden-dev/johndoe-azure/kubeconfig.yaml
$                        # From now on your local kubectl will be operating on target shoot
$ kubectl cluster-info   # will show your shoot cluster info
$ unset KUBECONFIG       # reset to your default kubectl
```

The shoot cluster's kubeconfig is being kept in a secret in the
project namespace:

```
kubectl -n shoot--dev--johndoe-azure get secret kubecfg -o jsonpath='{.data.kubeconfig}' | base64 -D > /tmp/johndoe-azure-kubeconfig.yaml
export KUBECONFIG=/tmp/johndoe-azure-kubeconfig.yaml
```

# Delete Shoot cluster

Deleting a Shoot cluster is not straight forward, and this is to
protect users from undesired/accidental cluster deletion. One has to
place some special annotations to get a Shoot cluster removed. We use
the [hack/delete](../../hack/delete) script for this purpose.

Please refer to [Creating / Deleting a Shoot
cluster](../usage/shoots.md) document for more details.

```
hack/delete shoot johndoe-azure garden-dev
```
