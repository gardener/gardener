# Access the kubernetes apiserver from your tailnet

If you would like to strengthen the security of your kubernetes cluster even further, this blog post explains how this can be achieved.

The most common way to secure a kubernetes cluster which was created with gardener is to apply ACLs described [here](https://github.com/stackitcloud/gardener-extension-acl),
or to use [ExposureClass](https://gardener.cloud/docs/gardener/exposureclasses/) which exposes the kubernetes apiserver in a cooporate network which is not exposed to the public internet.

Managing the ACL extension becomes fairly difficult with the growing number of participants, especially in a dynamic environment and work from home scenarios.

ExposureClass might be not possible because you do not have a cooporate network which is suitable for this purpose.

But there is a solution which bridges the gap between these two approaches by the use of a mesh vpn based on the [WireGuard](https://www.wireguard.com/)

## Tailscale

Tailscale is a mesh vpn network which uses wireguard under the hood, but automates the key exchange procedure.
Please consult the official [tailscale documentation](https://tailscale.com/kb/1151/what-is-tailscale) for a detailed explanation.

## Target Architecture

![architecture](images/tailscale.drawio.svg)

### Installation

In order to be able to access the kubernetes apiserver only from a tailscale vpn, we call it tailnet,

1. Create a tailscale account, ensure [MagicDNS](https://tailscale.com/kb/1081/magicdns?q=magic) is enabled.
2. Create a OAuth ClientID and Secret [OAuth](https://tailscale.com/kb/1236/kubernetes-operator#prerequisites), don't forget to create the tags mentioned there.
3. Install the tailscale operator [Operator](https://tailscale.com/kb/1236/kubernetes-operator#installation).

You can check if all went well after the operator installation by running `tailscale status`, the tailscale operator must be seen there:

```bash
# tailscale status
...
100.83.240.121  tailscale-operator   tagged-devices linux   -
...
```

### Expose the kubernetes apiserver

Now you are ready to expose the kubernetes apiserver in the tailnet by annotating the service which was created by gardener:

```bash
kubectl annotate -n default kubernetes tailscale.com/expose=true tailscale.com/hostname=kubernetes
```

It is required to `kubernetes` as the hostname, because this is part of the certificate common name of the kubernetes apiserver.

After annotating the service, it will be exposed in the tailnet and can be shown with `tailscale status`:

```bash
# tailscale status
...
100.83.240.121  tailscale-operator   tagged-devices linux   -
100.96.191.87   kubernetes           tagged-devices linux   idle, tx 19548 rx 71656
...
```

### Modify the kubeconfig

In order to access the cluster via the vpn, you must modify the kubeconfig to point to the kubernetes service exposed in the tailnet, by changing the `server` entry to `https://kubernetes`.

```yaml
---
apiVersion: v1
clusters:
  - cluster:
      certificate-authority-data: <base64 encoded secret>
      server: https://kubernetes
    name: my-cluster
...
```

### Enable ACLs which blocks all IPs.

Now you are ready to use your cluster from every device which is part of your tailnet. Therefore you can now block all access to the kubernetes apiserver with the ACL extension.

## Further improvements

Right now the tailscale operator can not be used if a installation of the open source coordination server [headscale](https://github.com/juanfont/headscale) should be used.
This is currently not an easy task, because headscale does not implement all required API endpoints for the tailscale operator. The details can be found in this [Issue](https://github.com/juanfont/headscale/issues/1202).
