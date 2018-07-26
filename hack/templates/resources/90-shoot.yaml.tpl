<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")))

  if context.get("cloud", "") == "":
    raise Exception("missing --var cloud={aws,azure,gcp,openstack,local} flag")

  def value(path, default):
    keys=str.split(path, ".")
    root=values
    for key in keys:
      if isinstance(root, dict):
        if key in root:
          root=root[key]
        else:
          return default
      else:
        return default
    return root

  region=""
  kubernetesVersion=""
  if cloud == "aws":
    region="eu-west-1"
    kubernetesVersion="1.11.0"
  elif cloud == "azure" or cloud == "az":
    region="westeurope"
    kubernetesVersion="1.11.0"
  elif cloud == "gcp":
    region="europe-west1"
    kubernetesVersion="1.11.0"
  elif cloud == "openstack" or cloud == "os":
    region="europe-1"
    kubernetesVersion="1.11.0"
  elif cloud == "local":
    region="local"
    kubernetesVersion="1.11.0"
%>---
apiVersion: garden.sapcloud.io/v1beta1
kind: Shoot
metadata:
  name: ${value("metadata.name", "johndoe-" + cloud)}
  namespace: ${value("metadata.namespace", "garden-dev")}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % endif
spec:
  cloud:
    profile: ${value("spec.cloud.profile", cloud)}
    region: ${value("spec.cloud.region", region)}
    secretBindingRef:
      name: ${value("spec.cloud.secretBindingRef.name", "core-" + cloud)}
    % if cloud == "aws":
    aws:
      networks:
        vpc:<% vpcID = value("spec.cloud.aws.networks.vpc.id", ""); vpcCIDR = value("spec.cloud.aws.networks.vpc.cidr", "10.250.0.0/16") %> # specify either 'id' or 'cidr'
          % if vpcID != "":
          id: ${vpcID}
        # cidr: 10.250.0.0/16
          % else:
        # id: vpc-123456
          cidr: ${vpcCIDR}
          % endif
        internal: ${value("spec.cloud.aws.networks.internal", ["10.250.112.0/22"])}
        public: ${value("spec.cloud.aws.networks.public", ["10.250.96.0/22"])}
        workers: ${value("spec.cloud.aws.networks.workers", ["10.250.0.0/19"])}
      workers:<% workers=value("spec.cloud.aws.workers", []) %>
      % if workers != []:
      ${yaml.dump(workers, width=10000)}
      % else:
      - name: cpu-worker
        machineType: m4.large
        volumeType: gp2
        volumeSize: 20Gi
        autoScalerMin: 2
        autoScalerMax: 2
      % endif
      zones: ${value("spec.cloud.aws.zones", ["eu-west-1a"])}
    % endif
    % if cloud == "azure" or cloud == "az":
    azure:<% resourceGroupName = value("spec.cloud.azure.resourceGroup.name", "") %>
      % if resourceGroupName != "":
      resourceGroup:
        name: ${resourceGroup}
      % else:
    # resourceGroup:
    #   name: mygroup
      % endif
      networks:
        vnet:<% vnetName = value("spec.cloud.azure.networks.vnet.name", ""); vnetCIDR = value("spec.cloud.azure.networks.vnet.cidr", "10.250.0.0/16") %> # specify either 'name' or 'cidr'
          % if vnetName != "":
          name: ${vnetName}
        # cidr: 10.250.0.0/16
          % else:
        # name: my-vnet
          cidr: ${vnetCIDR}
          % endif
        workers: ${value("spec.cloud.azure.networks.workers", "10.250.0.0/19")}
      workers:<% workers=value("spec.cloud.azure.workers", []) %>
      % if workers != []:
      ${yaml.dump(workers, width=10000)}
      % else:
      - name: cpu-worker
        machineType: Standard_DS2_v2
        volumeType: standard
        volumeSize: 35Gi # must be at least 35Gi for Azure VMs
        autoScalerMin: 2
        autoScalerMax: 2
      % endif
    % endif
    % if cloud == "gcp":
    gcp:
      networks:<% vpcName = value("spec.cloud.gcp.networks.vpc.name", "") %>
      % if vpcName != "":
        vpc:
          name: ${vpcName}
      % else:
      # vpc:
      #   name: my-vpc
      % endif
        workers: ${value("spec.cloud.gcp.networks.workers", ["10.250.0.0/19"])}
      workers:<% workers=value("spec.cloud.gcp.workers", []) %>
      % if workers != []:
      ${yaml.dump(workers, width=10000)}
      % else:
      - name: cpu-worker
        machineType: n1-standard-4
        volumeType: pd-standard
        volumeSize: 20Gi
        autoScalerMin: 2
        autoScalerMax: 2
      % endif
      zones: ${value("spec.cloud.gcp.zones", ["europe-west1-b"])}
    % endif
    % if cloud == "openstack" or cloud == "os":
    openstack:
      loadBalancerProvider: ${value("spec.cloud.openstack.loadBalancerProvider", "haproxy")}
      floatingPoolName: ${value("spec.cloud.openstack.floatingPoolName", "MY-FLOATING-POOL")}
      networks:<% routerID = value("spec.cloud.openstack.networks.router.id", "") %>
      % if routerID != "":
        router:
          id: ${routerID}
      % else:
      # router:
      #   id: 1234
      % endif
        workers: ${value("spec.cloud.openstack.networks.workers", ["10.250.0.0/19"])}
      workers:<% workers=value("spec.cloud.openstack.workers", []) %>
      % if workers != []:
      ${yaml.dump(workers, width=10000)}
      % else:
      - name: cpu-worker
        machineType: medium_2_4
        autoScalerMin: 2
        autoScalerMax: 2
      % endif
      zones: ${value("spec.cloud.openstack.zones", ["europe-1a"])}
    % endif
    % if cloud == "local":
    local:
      endpoint: ${value("spec.cloud.local.endpoint", "localhost:3777")} # endpoint service pointing to gardener-local-provider
      networks:
        workers: ${value("spec.cloud.local.networks.workers", ["192.168.99.100/24"])}
    % endif
  kubernetes:
    version: ${value("spec.kubernetes.version", kubernetesVersion)}
  dns:
    provider: ${value("spec.dns.provider", "aws-route53") if cloud != "local" else "unmanaged"}
    domain: ${value("spec.dns.domain", value("metadata.name", "johndoe-" + cloud) + "." + value("metadata.namespace", "garden-dev") + ".example.com") if cloud != "local" else "<minikube-ip>.nip.io"}
  maintenance:
    timeWindow:
      begin: ${value("spec.maintenance.timeWindow.begin", "220000+0100")}
      end: ${value("spec.maintenance.timeWindow.end", "230000+0100")}
    autoUpdate:
      kubernetesVersion: ${value("maintenance.autoUpdate.kubernetesVersion", "true")}
  % if cloud != "local":
  backup:
    schedule: ${value("backup.schedule", "\"*/5 * * * *\"")}
    maximum: ${value("backup.maximum", "7")}
  % endif
  addons:
  % if cloud == "aws":
    kube2iam:
      enabled: ${value("spec.addons.kube2iam.enabled", "true")}
      roles:<% roles=value("spec.addons.kube2iam.roles", []) %>
      % if roles != []:
      ${yaml.dump(roles, width=10000)}
      % else:
      - name: ecr
        description: "Allow access to ECR repositories beginning with 'my-images/', and creation of new repositories"
        policy: |
          {
            "Version": "2012-10-17",
            "Statement": [
              {
                "Action": "ecr:*",
                "Effect": "Allow",
                "Resource": "arn:aws:ecr:eu-central-1:<%text>${account_id}</%text>:repository/my-images/*"
              },
              {
                "Action": [
                  "ecr:GetAuthorizationToken",
                  "ecr:CreateRepository"
                ],
                "Effect": "Allow",
                "Resource": "*"
              }
            ]
          }
      % endif
    % endif
    heapster:
      enabled: ${value("spec.addons.heapster.enabled", "true")}
    kubernetes-dashboard:
      enabled: ${value("spec.addons.kubernetes-dashboard.enabled", "true")}
    cluster-autoscaler:
      enabled: ${value("spec.addons.cluster-autoscaler.enabled", "true")}
    nginx-ingress:
      enabled: ${value("spec.addons.nginx-ingress.enabled", "true")}
      loadBalancerSourceRanges: ${value("spec.addons.nginx-ingress.loadBalancerSourceRanges", [])}
    kube-lego:
      enabled: ${value("spec.addons.kube-lego.enabled", "true")}
      email: ${value("spec.addons.kube-lego.email", "john.doe@example.com")}
    monocular:
      enabled: ${value("spec.addons.monocular.enabled", "false")}
