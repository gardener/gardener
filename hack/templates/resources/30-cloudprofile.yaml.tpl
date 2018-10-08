<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")))

  if context.get("cloud", "") == "":
    raise Exception("missing --var cloud={aws,azure,gcp,alicloud,openstack,local} flag")

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
  if cloud == "aws":
    region="eu-west-1"
  elif cloud == "azure" or cloud == "az":
    region="westeurope"
  elif cloud == "gcp":
    region="europe-west1"
  elif cloud == "alicloud":
    region="cn-beijing"
  elif cloud == "openstack" or cloud == "os":
    region="europe-1"
  elif cloud == "local":
    region="local"
%>---
apiVersion: garden.sapcloud.io/v1beta1
kind: CloudProfile
metadata:
  name: ${value("spec.profile", cloud)}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000)}
  % endif
spec:<% caBundle=value("spec.caBundle", "") %>
  % if caBundle != "":
  caBundle: ${caBundle}
  % else:
# caBundle: |
#   -----BEGIN CERTIFICATE-----
#   ...
#   -----END CERTIFICATE-----
  % endif
  % if cloud == "aws":
  aws:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.aws.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000)}
      % else:
      - name: aws-route53
      - name: unmanaged
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.aws.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000)}
        % else:
        - 1.12.1
        - 1.11.3
        - 1.10.8
        - 1.9.11
        % endif
      machineImages:<% machineImages=value("spec.aws.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000)}
      % else:
      - name: CoreOS
        regions:
        - name: eu-west-1
          ami: ami-1ed8d467
        - name: us-east-1
          ami: ami-f6ecac89
      % endif
      machineTypes:<% machineTypes=value("spec.aws.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000)}
      % else:
      - name: m4.large
        cpu: "2"
        gpu: "0"
        memory: 8Gi
      - name: m4.xlarge
        cpu: "4"
        gpu: "0"
        memory: 16Gi
      - name: m4.2xlarge
        cpu: "8"
        gpu: "0"
        memory: 32Gi
      - name: m4.4xlarge
        cpu: "16"
        gpu: "0"
        memory: 64Gi
      - name: m4.10xlarge
        cpu: "40"
        gpu: "0"
        memory: 160Gi
      - name: m4.16xlarge
        cpu: "64"
        gpu: "0"
        memory: 256Gi
      - name: p2.xlarge
        cpu: "4"
        gpu: "1"
        memory: 61Gi
      - name: p2.8xlarge
        cpu: "32"
        gpu: "8"
        memory: 488Gi
      - name: p2.16xlarge
        cpu: "64"
        gpu: "16"
        memory: 732Gi
      % endif
      volumeTypes:<% volumeTypes=value("spec.aws.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000)}
      % else:
      - name: gp2
        class: standard
      - name: io1
        class: premium
      % endif
      zones:<% zones=value("spec.aws.constraints.zones", []) %>
      % if zones != []:
      ${yaml.dump(zones, width=10000)}
      % else:
      - region: eu-west-1
        names:
        - eu-west-1a
        - eu-west-1b
        - eu-west-1c
      - region: us-east-1
        names:
        - us-east-1a
        - us-east-1b
        - us-east-1c
      % endif
  % endif
  % if cloud == "azure" or cloud == "az":
  azure:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.azure.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000)}
      % else:
      - name: aws-route53
      - name: unmanaged
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.azure.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000)}
        % else:
        - 1.12.1
        - 1.11.3
        - 1.10.8
        - 1.9.11
        % endif
      machineImages:<% machineImages=value("spec.azure.constraints.machineImages", []) %>
        % if machineImages != []:
        ${yaml.dump(machineImages, width=10000)}
        % else:
      - name: CoreOS
        publisher: CoreOS
        offer: CoreOS
        sku: Stable
        version: 1745.7.0
      % endif
      machineTypes:<% machineTypes=value("spec.azure.constraints.machineTypes", []) %>
        % if machineTypes != []:
        ${yaml.dump(machineTypes, width=10000)}
        % else:
      - name: Standard_DS2_v2
        cpu: "2"
        gpu: "0"
        memory: 7Gi
      - name: Standard_DS3_v2
        cpu: "4"
        gpu: "0"
        memory: 14Gi
      - name: Standard_DS4_v2
        cpu: "8"
        gpu: "0"
        memory: 28Gi
      - name: Standard_DS5_v2
        cpu: "16"
        gpu: "0"
        memory: 56Gi
      - name: Standard_F2s
        cpu: "2"
        gpu: "0"
        memory: 4Gi
      - name: Standard_F4s
        cpu: "4"
        gpu: "0"
        memory: 8Gi
      - name: Standard_F8s
        cpu: "8"
        gpu: "0"
        memory: 16Gi
      - name: Standard_F16s
        cpu: "16"
        gpu: "0"
        memory: 32Gi
      % endif
      volumeTypes:<% volumeTypes=value("spec.azure.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000)}
      % else:
      - name: standard
        class: standard
      - name: premium
        class: premium
      % endif
    countUpdateDomains:<% countUpdateDomains=value("spec.azure.countUpdateDomains", []) %>
    % if countUpdateDomains != []:
    ${yaml.dump(countUpdateDomains, width=10000)}
    % else:
    - region: westeurope
      count: 5
    - region: eastus
      count: 5
    % endif
    countFaultDomains:<% countFaultDomains=value("spec.azure.countFaultDomains", []) %>
    % if countFaultDomains != []:
    ${yaml.dump(countFaultDomains, width=10000)}
    % else:
    - region: westeurope
      count: 2
    - region: eastus
      count: 2
    % endif
  % endif
  % if cloud == "gcp":
  gcp:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.gcp.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000)}
      % else:
      - name: aws-route53
      - name: unmanaged
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.gcp.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000)}
        % else:
        - 1.12.1
        - 1.11.3
        - 1.10.8
        - 1.9.11
        % endif
      machineImages:<% machineImages=value("spec.gcp.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000)}
      % else:
      - name: CoreOS
        image: projects/coreos-cloud/global/images/coreos-stable-1745-7-0-v20180614
      % endif
      machineTypes:<% machineTypes=value("spec.gcp.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000)}
      % else:
      - name: n1-standard-2
        cpu: "2"
        gpu: "0"
        memory: 7500Mi
      - name: n1-standard-4
        cpu: "4"
        gpu: "0"
        memory: 15Gi
      - name: n1-standard-8
        cpu: "8"
        gpu: "0"
        memory: 30Gi
      - name: n1-standard-16
        cpu: "16"
        gpu: "0"
        memory: 60Gi
      - name: n1-standard-32
        cpu: "32"
        gpu: "0"
        memory: 120Gi
      - name: n1-standard-64
        cpu: "64"
        gpu: "0"
        memory: 240Gi
      % endif
      volumeTypes:<% volumeTypes=value("spec.gcp.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000)}
      % else:
      - name: pd-standard
        class: standard
      - name: pd-ssd
        class: premium
      % endif
      zones:<% zones=value("spec.gcp.constraints.zones", []) %>
      % if zones != []:
      ${yaml.dump(zones, width=10000)}
      % else:
      - region: europe-west1
        names:
        - europe-west1-b
        - europe-west1-c
        - europe-west1-d
      - region: us-east1
        names:
        - us-east1-b
        - us-east1-c
        - us-east1-d
    % endif
  % endif
  % if cloud == "alicloud":
  alicloud:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.alicloud.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000)}
      % else:
      - name: aws-route53
      - name: unmanaged
      % endif
      kubernetes:<% kubernetesVersions=value("spec.alicloud.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000)}
        % else:
        versions:
        - 1.12.1
        - 1.11.3
        - 1.10.8
        % endif
      machineImages:<% machineImages=value("spec.alicloud.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000)}
      % else:
      - name: CoreOS
        id: coreos_1745_7_0_64_30G_alibase_20180705.vhd
      % endif
      machineTypes:<% machineTypes=value("spec.alicloud.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000)}
      % else:
      - name: ecs.sn2ne.large
        cpu: "2"
        gpu: "0"
        memory: 8Gi
        zones:
        - cn-beijing-f
      - name: ecs.sn2ne.xlarge
        cpu: "4"
        gpu: "0"
        memory: 16Gi
        zones:
        - cn-beijing-f
      - name: ecs.sn2ne.2xlarge
        cpu: "8"
        gpu: "0"
        memory: 32Gi
        zones:
        - cn-beijing-f
      - name: ecs.sn2ne.3xlarge
        cpu: "12"
        gpu: "0"
        memory: 48Gi
        zones:
        - cn-beijing-f
      % endif
      volumeTypes:<% volumeTypes=value("spec.alicloud.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000)}
      % else:
      - name: cloud_efficiency
        class: standard
        zones:
        - cn-beijing-f
      - name: ssd
        class: premium
        zones:
        - cn-beijing-f
      % endif
      zones:<% zones=value("spec.alicloud.zones", []) %> # List of availablity zones together with resource contraints in a specific region
      % if zones != []:
      ${yaml.dump(zones, width=10000)}
      % else:
      - region: cn-beijing
        names:
        - cn-beijing-f  # Avalibility zone
      % endif
  % endif
  % if cloud == "openstack" or cloud == "os":
  openstack:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.openstack.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000)}
      % else:
      - name: aws-route53
      - name: unmanaged
      % endif
      floatingPools:<% floatingPools=value("spec.openstack.constraints.floatingPools", []) %>
      % if floatingPools != []:
      ${yaml.dump(floatingPools, width=10000)}
      % else:
      - name: MY-FLOATING-POOL
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.openstack.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000)}
        % else:
        - 1.12.1
        - 1.11.3
        - 1.10.8
        - 1.9.11
        % endif
      loadBalancerProviders:<% loadBalancerProviders=value("spec.openstack.constraints.loadBalancerProviders", []) %>
      % if loadBalancerProviders != []:
      ${yaml.dump(loadBalancerProviders, width=10000)}
      % else:
      - name: haproxy
      % endif
      machineImages:<% machineImages=value("spec.openstack.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000)}
      % else:
      - name: CoreOS
        image: coreos-1745.7.0
      % endif
      machineTypes:<% machineTypes=value("spec.openstack.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000)}
      % else:
      - name: medium_2_4
        cpu: "2"
        gpu: "0"
        memory: 4Gi
        volumeType: default
        volumeSize: 20Gi
      - name: medium_4_8
        cpu: "4"
        gpu: "0"
        memory: 8Gi
        volumeType: default
        volumeSize: 40Gi
      % endif
      zones:<% zones=value("spec.openstack.constraints.zones", []) %>
      % if zones != []:
      ${yaml.dump(zones, width=10000)}
      % else:
      - region: europe-1
        names:
        - europe-1a
        - europe-1b
        - europe-1c
      % endif
    keystoneURL: ${value("spec.openstack.keyStoneURL", "https://url-to-keystone/v3/")}<% dnsServers=value("spec.openstack.dnsServers", []) %><% dhcpDomain=value("spec.openstack.dhcpDomain", "") %>
    % if dnsServers != []:
    dnsServers: ${dnsServers}
    % endif
    % if dhcpDomain != "":
    dhcpDomain: ${dhcpDomain}
    % else:
  # dhcpDomain: nova.local # DHCP domain of OpenStack system (only meaningful for Kubernetes 1.10.1, see https://github.com/kubernetes/kubernetes/pull/61890 for details)
    % endif
  % endif
  % if cloud == "local":
  local:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.local.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000)}
      % else:
      - name: unmanaged
      % endif
  % endif
