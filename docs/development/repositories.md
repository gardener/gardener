# Repositories of required components

The Gardener deploys a lot of helping components which are required to make the Shoot cluster functional. Please find a list of those repositories below.

## Repository list

- [VPN](https://github.com/gardener/vpn) - a set of components which establish connectivity from a pod running in the Seed cluster to the networks of a Shoot cluster (which are usually private).
- [Garden Terraformer](https://github.com/gardener/terraformer) - a can execute Terraform configuration and is designed to run as a pod inside a Kubernetes cluster.
- [Garden Readvertiser](https://github.com/gardener/aws-lb-readvertiser) - a component which is used to keep AWS Shoot cluster API servers reachable.
- [Nginx-Ingress Default Backend](https://github.com/gardener/ingress-default-backend) - a component which serves a static HTML page that is shown for all incoming requests to nginx that are not controlled by an Ingress object.
- [Auto Node Repair](https://github.com/gardener/auto-node-repair) - a component which removes broken AWS nodes from a cluster.
- [ETCD Operator](https://github.com/gardener/etcd-operator) - fork of https://github.com/coreos/etcd-operator - a component which manages and backups etcd clusters for Shoots.
