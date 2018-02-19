# Repositories of required components

The Gardener deploys a lot of helping components which are required to make the Shoot cluster functional. Please find a list of those repositories below.

## Repository list

- [Machine Controller Manager](https://github.com/gardener/machine-controller-manager) - a component which manages VMs/nodes as part of declarative custom resources inside Kubernetes.
- [VPN](https://github.com/gardener/vpn) - a set of components which establish connectivity from a pod running in the Seed cluster to the networks of a Shoot cluster (which are usually private).
- [Terraformer](https://github.com/gardener/terraformer) - a can execute Terraform configuration and is designed to run as a pod inside a Kubernetes cluster.
- [AWS Load Balancer Readvertiser](https://github.com/gardener/aws-lb-readvertiser) - a component which is used to keep AWS Shoot cluster API servers reachable.
- [Ingress Default Backend](https://github.com/gardener/ingress-default-backend) - a component which serves a static HTML page that is shown for all incoming requests to nginx that are not controlled by an Ingress object.
