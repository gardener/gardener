// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package extensionresources_test

var (
	backupBucketFmt = `
{ "apiVersion": "extensions.gardener.cloud/v1alpha1",
"kind": "BackupBucket",
"metadata": {
  "name": "cloud--gcp--fg2d6",
  %s
  "namespace": "prjswebhooks"
  },
"spec":{
  "type": %q,
  "region": "eu-west-1",
  "secretRef": {
    "name": %q,
    "namespace": "garden"
  }
}
}
`

	backupEntryFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "BackupEntry",
  "metadata": {
    "name": "shoot--foobar--gcp--sd34f",
	%s
    "namespace": "prjwebhooks"
  },
  "spec": {
    "type": %q,
    "region": "eu-west-1",
    "bucketName": "cloud--gcp--fg2d6",
    "secretRef": {
      "name": %q,
      "namespace": "garden"
    }
  }
}`

	bastionFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "Bastion",
  "metadata": {
    "generateName": "cli-",
    %s
    "name": "cli-abcdef",
    "namespace": "garden-myproject"
  },
  "spec": {
    "type": %q,
    "shootRef": {
      "name": %q
    },
    "userData": "data",
    "seedName": "aws-eu2",
    "sshPublicKey": "c3NoLXJzYSAuLi4K",
    "providerType": "gcp",
    "ingress": [
      {
        "ipBlock": {
          "cidr": "1.2.3.4/32"
        }
      }
    ]
  }
}`

	controlPlaneFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "ControlPlane",
  "metadata": {
    "name": "control-plane",
	%s
    "namespace": "shoot--foobar--gcp"
  },
  "spec": {
    "type": %q,
    "region": "europe-west1",
    "secretRef": {
      "name": %q,
      "namespace": "shoot--foobar--gcp"
    }
  }
}`

	dnsrecordFmt = `
{ "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "DNSRecord",
  "metadata": {
    "name": "dnsrecord-external",
	%s
    "namespace": "prjswebhooks"
  },
  "spec": {
    "type": "google-clouddns",
    "secretRef": {
      "name": %q,
      "namespace": "prjswebhooks"
    },
    "name": "api.gcp.foobar.shoot.example.com",
    "recordType": %q,
    "values": [
      "1.2.3.4"
    ]
  }
}`

	extensionsFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "Extension",
  "metadata": {
    "name": "extension-name",
	%s
    "namespace": "prjwebhooks"
  },
  "spec": {
    "type": %q,
    "purpose": "provision",
	"secretRef":{
    	"name": %q,
    	"namespace": "garden"
	}
  }
}
`

	infrastructureFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "Infrastructure",
  "metadata": {
    "name": "gcp-infra",
	%s
    "namespace": "shoot--foobar--gcp"
  },
  "spec": {
    "type": %q,
    "region": "europe-west1",
    "secretRef": {
      "namespace": "shoot--foobar--gcp",
      "name": %q
    },
    "providerConfig": {
      "apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1",
      "kind": "InfrastructureConfig",
      "networks": {
        "workers": "10.242.0.0/19"
      }
    }
  }
}`

	networksFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "Network",
  "metadata": {
    "name": "gcp-networks",
	%s
    "namespace": "shoot--foo--bar"
  },
  "spec": {
    "podCIDR": "100.96.0.0/11",
    "serviceCIDR": "100.64.0.0/13",
    "type": %q,
    "secretRef": {
       "namespace": "shoot--foobar--gcp",
       "name": %q
    }
  }
}`

	operatingsysconfigFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "OperatingSystemConfig",
  "metadata": {
    "name": "gcp-osc",
	%s
    "namespace": "prjwebhooks"
  },
  "spec": {
    "type": %q,
    "purpose": "provision",
   "secretRef": {
      "namespace": "shoot--foobar--gcp",
      "name": %q
    }
  }
}`

	workerFmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "Worker",
  "metadata": {
    "name": "worker",
	%s
    "namespace": "shoot--foobar--gcp"
  },
  "spec": {
    "type": %q,
    "region": "europe-west1",
    "secretRef": {
      "name": %q,
      "namespace": "shoot--foobar--gcp"
    }
  }
}`
)
