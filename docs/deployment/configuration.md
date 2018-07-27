# Configuration

In order to establish different configuration settings for the same cloud environment, one has to define [`CloudProfiles`](../../example/30-cloudprofile-aws.yaml). These profiles define configuration and constraints for allowed values in the Shoot manifest as well.

Seed clusters have their [own resource](../../example/50-seed-aws.yaml) as well. These resources contain metadata about the respective Seed cluster and a reference to a secret holding the credentials (see below).

The Gardener requires some secrets in order to work properly. These secrets are:
* *Seed cluster secrets*, contain the credentials of the cloud provider account in which the Seed cluster is deployed, and a Kubeconfig which can be used to authenticate against the Seed cluster's kube-apiserver, please see [this](../../example/40-secret-seed-aws.yaml) for an example.

* *Internal domain secrets* (optional), contain the DNS provider credentials (with appropriate privileges) which will be used to create/delete internal DNS records for the Shoot clusters (e.g., `example.com`), please see [this](../../example/10-secret-internal-domain.yaml) for an example.
  * These secrets are used in order to establish a stable endpoint for Shoot clusters which is used internally by all control plane components.
  * It is forbidden to change the internal domain secret if there are existing Shoot clusters.

* *Default domain secrets* (optional), contain the DNS provider credentials (with appropriate privileges) which will be used to create/delete DNS records for the default domain (e.g., `example.com`), please see [this](../../example/10-secret-default-domain.yaml) for an example.
  * These secrets are used in order to allow not specifying a hosted zone when creating a Shoot cluster in the `.spec.dns.hostedZoneID` field (useful when a user does not have an own domain/hosted zone but want us to manage it). In this case, based on the provided `.spec.dns.domain` value, the Gardener tries to find an appropriate secret holding the credentials for the hosted zone of this domain. It will use them to manage the relevant DNS records. Currently, we have implemented AWS Route53 and Google CloudDNS as DNS providers. For Google CloudDNS you need to provide a service account with the `DNS Administrator` role. For AWS you need to provide a user being assigned to this IAM policy document:
    ```bash
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Action": [
            "route53:GetChange",
            "route53:GetHostedZone",
            "route53:ListResourceRecordSets",
            "route53:ChangeResourceRecordSets"
          ],
          "Resource": "*"
        }
      ]
    }
    ```

* *Alerting SMTP secrets* (optional), contain the SMTP credentials which will be used by the [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) to send emails on alerts, please see [this](../../example/10-secret-alerting-smtp.yaml) for an example.
  * These secrets are used by the Alertmanager which is deployed next to the Kubernetes control plane of a Shoot cluster in Seed clusters. In case there have been alerting SMTP secrets configured, the Gardener will inject the credentials in the configuration of the Alertmanager. It will use them to send mails to the stated email address in case anything is wrong with the Shoot clusters.

* *Cloud provider secrets*, contains the credentials of the cloud provider account in which Shoot clusters can be deployed, please see [this](../../example/70-secret-cloudprovider-aws.yaml) for an example.
  * For each Shoot cluster, the Gardener needs to create infrastructure (networks, security groups, technical users, ...) and worker nodes in the desired cloud provider account.

The described secrets are expected to be stored in the so-called **Garden namespace**. In case the Gardener runs inside a Kubernetes cluster, the Garden namespace is the namespace the Gardener is deployed in (default, can be overwritten). In case it runs outside (local development), the Garden namespace must be specified via a command line flag (see below).
The secrets are determined based on labels with key `garden.sapcloud.io/role`. Please take a look on the above linked examples.

The Seed cluster which is used to deploy the control plane of a Shoot cluster can be specified by the user in the Shoot manifest (see [here](../../example/90-shoot-azure.yaml#L10)). If it is not specified, the Gardener will try to find an adequate Seed cluster (one deployed in the same region at the same cloud provider) automatically.

The cloud provider secrets can be stored in any namespace. With [`SecretBindings`](../../example/80-secretbinding-cloudprovider-aws.yaml) one can reference a secret in the same or in another namespace. These binding objects can also be used to reference `Quotas` for the specific secret.

## Configuration file for Gardener controller manager
The Gardener controller manager does only support one command line flag which should be a path to a valid configuration file.

Please take a look at [this](../../example/20-componentconfig-gardener-controller-manager.yaml) example configuration.
