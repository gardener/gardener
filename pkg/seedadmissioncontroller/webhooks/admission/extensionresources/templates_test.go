package extensionresources_test

var (
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
}
`
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
}
`
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
}
`

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
}
`

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
}
`

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
}
`
)
