<%
  import os, yaml

  values={}
  if context.get("values", "") != "":
    values=yaml.load(open(context.get("values", "")), Loader=yaml.Loader)

  if context.get("cloud", "") == "":
    raise Exception("missing --var cloud={aws,azure,gcp,alicloud,openstack,packet} flag")

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
  elif cloud == "packet":
    region="ewr1"
  elif cloud == "openstack" or cloud == "os":
    region="europe-1"
%>---
apiVersion: garden.sapcloud.io/v1beta1
kind: CloudProfile
metadata:
  name: ${value("spec.profile", cloud)}<% annotations = value("metadata.annotations", {}); labels = value("metadata.labels", {}) %>
  % if annotations != {}:
  annotations: ${yaml.dump(annotations, width=10000, default_flow_style=None)}
  % endif
  % if labels != {}:
  labels: ${yaml.dump(labels, width=10000, default_flow_style=None)}
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
      ${yaml.dump(dnsProviders, width=10000, default_flow_style=None)}
      % else:
      - name: unmanaged
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.aws.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000, default_flow_style=None)}
        % else:
        - 1.15.2
        - 1.14.5
        - 1.13.9
        - 1.12.10
        - 1.11.10
        - 1.10.13
        % endif
      machineImages:<% machineImages=value("spec.aws.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000, default_flow_style=None)}
      % else:
      - name: coreos
        versions:
        - version: 2023.5.0
        # Proper mappings to region-specific AMIs must exist in the `Worker` controller of the provider extension.
      % endif
      machineTypes:<% machineTypes=value("spec.aws.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000, default_flow_style=None)}
      % else:
      - name: m5.large
        cpu: "2"
        gpu: "0"
        memory: 8Gi
        usable: true
      - name: m5.xlarge
        cpu: "4"
        gpu: "0"
        memory: 16Gi
        usable: true
      - name: m5.2xlarge
        cpu: "8"
        gpu: "0"
        memory: 32Gi
        usable: true
      - name: m5.4xlarge
        cpu: "16"
        gpu: "0"
        memory: 64Gi
        usable: true
      - name: m5.12xlarge
        cpu: "48"
        gpu: "0"
        memory: 192Gi
        usable: true
      - name: m5.24xlarge
        cpu: "96"
        gpu: "0"
        memory: 384Gi
        usable: false
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
      ${yaml.dump(volumeTypes, width=10000, default_flow_style=None)}
      % else:
      - name: gp2
        class: standard
        usable: true
      - name: io1
        class: premium
        usable: false
      % endif
      zones:<% zones=value("spec.aws.constraints.zones", []) %>
      % if zones != []:
      ${yaml.dump(zones, width=10000, default_flow_style=None)}
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
      ${yaml.dump(dnsProviders, width=10000, default_flow_style=None)}
      % else:
      - name: unmanaged
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.azure.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000, default_flow_style=None)}
        % else:
        - 1.15.2
        - 1.14.5
        - 1.13.9
        - 1.12.10
        - 1.11.10
        - 1.10.13
        % endif
      machineImages:<% machineImages=value("spec.azure.constraints.machineImages", []) %>
        % if machineImages != []:
        ${yaml.dump(machineImages, width=10000, default_flow_style=None)}
        % else:
      - name: coreos
        versions:
        - version: 2023.5.0
        # Proper mappings to publisher, offer, and SKU names must exist in the `Worker` controller of the provider extension.
      % endif
      machineTypes:<% machineTypes=value("spec.azure.constraints.machineTypes", []) %>
        % if machineTypes != []:
        ${yaml.dump(machineTypes, width=10000, default_flow_style=None)}
        % else:
      - name: Standard_D2_v3
        cpu: "2"
        gpu: "0"
        memory: 8Gi
        usable: true
      - name: Standard_D4_v3
        cpu: "4"
        gpu: "0"
        memory: 16Gi
        usable: true
      - name: Standard_D8_v3
        cpu: "8"
        gpu: "0"
        memory: 32Gi
        usable: true
      - name: Standard_D16_v3
        cpu: "16"
        gpu: "0"
        memory: 64Gi
        usable: false
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
      ${yaml.dump(volumeTypes, width=10000, default_flow_style=None)}
      % else:
      - name: standard
        class: standard
        usable: true
      - name: premium
        class: premium
        usable: false
      % endif
    countUpdateDomains:<% countUpdateDomains=value("spec.azure.countUpdateDomains", []) %>
    % if countUpdateDomains != []:
    ${yaml.dump(countUpdateDomains, width=10000, default_flow_style=None)}
    % else:
    - region: westeurope
      count: 5
    - region: eastus
      count: 5
    % endif
    countFaultDomains:<% countFaultDomains=value("spec.azure.countFaultDomains", []) %>
    % if countFaultDomains != []:
    ${yaml.dump(countFaultDomains, width=10000, default_flow_style=None)}
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
      ${yaml.dump(dnsProviders, width=10000, default_flow_style=None)}
      % else:
      - name: unmanaged
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.gcp.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000, default_flow_style=None)}
        % else:
        - 1.15.2
        - 1.14.5
        - 1.13.9
        - 1.12.10
        - 1.11.10
        - 1.10.13
        % endif
      machineImages:<% machineImages=value("spec.gcp.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000, default_flow_style=None)}
      % else:
      - name: coreos
        versions:
        - version: 2023.5.0
        # Proper mappings to GCP image URLs must exist in the `Worker` controller of the provider extension.
      % endif
      machineTypes:<% machineTypes=value("spec.gcp.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000, default_flow_style=None)}
      % else:
      - name: n1-standard-2
        cpu: "2"
        gpu: "0"
        memory: 7500Mi
        usable: true
      - name: n1-standard-4
        cpu: "4"
        gpu: "0"
        memory: 15Gi
        usable: true
      - name: n1-standard-8
        cpu: "8"
        gpu: "0"
        memory: 30Gi
        usable: true
      - name: n1-standard-16
        cpu: "16"
        gpu: "0"
        memory: 60Gi
        usable: true
      - name: n1-standard-32
        cpu: "32"
        gpu: "0"
        memory: 120Gi
        usable: true
      - name: n1-standard-64
        cpu: "64"
        gpu: "0"
        memory: 240Gi
        usable: false
      % endif
      volumeTypes:<% volumeTypes=value("spec.gcp.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000, default_flow_style=None)}
      % else:
      - name: pd-standard
        class: standard
        usable: true
      - name: pd-ssd
        class: premium
        usable: false
      % endif
      zones:<% zones=value("spec.gcp.constraints.zones", []) %>
      % if zones != []:
      ${yaml.dump(zones, width=10000, default_flow_style=None)}
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
      ${yaml.dump(dnsProviders, width=10000, default_flow_style=None)}
      % else:
      - name: alicloud-dns
      - name: unmanaged
      % endif
      kubernetes:<% kubernetesVersions=value("spec.alicloud.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000, default_flow_style=None)}
        % else:
        versions:
        - 1.15.2
        - 1.14.5
        - 1.13.9
        % endif
      machineImages:<% machineImages=value("spec.alicloud.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000, default_flow_style=None)}
      % else:
      - name: coreos-alicloud
        versions:
        - version: 2023.5.0
        # Proper mappings to Alicloud image VHD IDs must exist in the `Worker` controller of the provider extension.
      % endif
      machineTypes:<% machineTypes=value("spec.alicloud.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000, default_flow_style=None)}
      % else:
      - name: ecs.sn2ne.large
        cpu: "2"
        gpu: "0"
        memory: 8Gi
        usable: true
        zones:
        - cn-beijing-f
      - name: ecs.sn2ne.xlarge
        cpu: "4"
        gpu: "0"
        memory: 16Gi
        usable: true
        zones:
        - cn-beijing-f
      - name: ecs.sn2ne.2xlarge
        cpu: "8"
        gpu: "0"
        memory: 32Gi
        usable: true
        zones:
        - cn-beijing-f
      - name: ecs.sn2ne.3xlarge
        cpu: "12"
        gpu: "0"
        memory: 48Gi
        usable: false
        zones:
        - cn-beijing-f
      % endif
      volumeTypes:<% volumeTypes=value("spec.alicloud.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000, default_flow_style=None)}
      % else:
      - name: cloud_efficiency
        class: standard
        usable: true
        zones:
        - cn-beijing-f
      - name: cloud_ssd
        class: premium
        usable: false
        zones:
        - cn-beijing-f
      % endif
      zones:<% zones=value("spec.alicloud.zones", []) %> # List of availablity zones together with resource contraints in a specific region
      % if zones != []:
      ${yaml.dump(zones, width=10000, default_flow_style=None)}
      % else:
      - region: cn-beijing
        names:
        - cn-beijing-f  # Avalibility zone
      % endif
  % endif
  % if cloud == "packet":
  packet:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.packet.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000, default_flow_style=None)}
      % else:
      - name: aws-route53
      - name: unmanaged
      % endif
      kubernetes:<% kubernetesVersions=value("spec.packet.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000, default_flow_style=None)}
        % else:
        versions:
        - 1.15.2
        - 1.14.5
        - 1.13.9
        % endif
      machineImages:<% machineImages=value("spec.packet.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000, default_flow_style=None)}
      % else:
      - name: coreos
        versions:
        - version: 2079.3.0
        # Proper mappings to Packet image IDs must exist in the `Worker` controller of the provider extension.
      % endif
      machineTypes:<% machineTypes=value("spec.packet.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000, default_flow_style=None)}
      % else:
      - name: t1.small
        cpu: "4"
        gpu: "0"
        memory: 8Gi
        usable: true
      - name: c1.small
        cpu: "4"
        gpu: "0"
        memory: 32Gi
        usable: true
      - name: c2.medium
        cpu: "24"
        gpu: "0"
        memory: 64Gi
        usable: true
      - name: m1.xlarge
        cpu: "24"
        gpu: "0"
        memory: 256Gi
        usable: true
      - name: c1.large.arm
        cpu: "96"
        gpu: "0"
        memory: 128Gi
        usable: true
      - name: g2.large
        cpu: "28"
        gpu: "2"
        memory: 192Gi
        usable: true
      % endif
      volumeTypes:<% volumeTypes=value("spec.packet.constraints.volumeTypes", []) %>
      % if volumeTypes != []:
      ${yaml.dump(volumeTypes, width=10000, default_flow_style=None)}
      % else:
      - name: storage_1
        class: standard
        usable: true
      - name: storage_2
        class: performance
        usable: true
      % endif
      zones:<% zones=value("spec.packet.zones", []) %> # List of availablity zones together with resource contraints in a specific region
      % if zones != []:
      ${yaml.dump(zones, width=10000, default_flow_style=None)}
      % else:
      - region: ewr1
        names:
        - ewr1
      % endif
  % endif
  % if cloud == "openstack" or cloud == "os":
  openstack:
    constraints:
      dnsProviders:<% dnsProviders=value("spec.openstack.constraints.dnsProviders", []) %>
      % if dnsProviders != []:
      ${yaml.dump(dnsProviders, width=10000, default_flow_style=None)}
      % else:
      - name: unmanaged
      % endif
      floatingPools:<% floatingPools=value("spec.openstack.constraints.floatingPools", []) %>
      % if floatingPools != []:
      ${yaml.dump(floatingPools, width=10000, default_flow_style=None)}
      % else:
      - name: MY-FLOATING-POOL
      % endif
      kubernetes:
        versions:<% kubernetesVersions=value("spec.openstack.constraints.kubernetes.versions", []) %>
        % if kubernetesVersions != []:
        ${yaml.dump(kubernetesVersions, width=10000, default_flow_style=None)}
        % else:
        - 1.15.2
        - 1.14.5
        - 1.13.9
        - 1.12.10
        - 1.11.10
        - 1.10.13
        % endif
      loadBalancerProviders:<% loadBalancerProviders=value("spec.openstack.constraints.loadBalancerProviders", []) %>
      % if loadBalancerProviders != []:
      ${yaml.dump(loadBalancerProviders, width=10000, default_flow_style=None)}
      % else:
      - name: haproxy
      % endif
      machineImages:<% machineImages=value("spec.openstack.constraints.machineImages", []) %>
      % if machineImages != []:
      ${yaml.dump(machineImages, width=10000, default_flow_style=None)}
      % else:
      - name: coreos
        versions:
        - version: 2023.5.0
        # Proper mappings to OpenStack Glance image names for this CloudProfile must exist in the `Worker` controller of the provider extension.
      % endif
      machineTypes:<% machineTypes=value("spec.openstack.constraints.machineTypes", []) %>
      % if machineTypes != []:
      ${yaml.dump(machineTypes, width=10000, default_flow_style=None)}
      % else:
      - name: medium_2_4
        cpu: "2"
        gpu: "0"
        memory: 4Gi
        usable: true
        volumeType: default
        volumeSize: 20Gi
      - name: medium_4_8
        cpu: "4"
        gpu: "0"
        memory: 8Gi
        usable: true
        volumeType: default
        volumeSize: 40Gi
      % endif
      zones:<% zones=value("spec.openstack.constraints.zones", []) %>
      % if zones != []:
      ${yaml.dump(zones, width=10000, default_flow_style=None)}
      % else:
      - region: europe-1
        names:
        - europe-1a
        - europe-1b
        - europe-1c
      % endif
    keystoneURL: ${value("spec.openstack.keyStoneURL", "https://url-to-keystone/v3/")}<% dnsServers=value("spec.openstack.dnsServers", []) %><% dhcpDomain=value("spec.openstack.dhcpDomain", "") %><% requestTimeout=value("spec.openstack.requestTimeout", "") %>
    % if dnsServers != []:
    dnsServers: ${dnsServers}
    % endif
    % if dhcpDomain != "":
    dhcpDomain: ${dhcpDomain}
    % else:
  # dhcpDomain: nova.local # DHCP domain of OpenStack system (only meaningful for Kubernetes 1.10.1, see https://github.com/kubernetes/kubernetes/pull/61890 for details)
    % endif
    % if requestTimeout != "":
    requestTimeout: ${requestTimeout}
    % else:
  # requestTimeout: 180s # Kubernetes OpenStack Cloudprovider Request Timeout
    % endif
  % endif
