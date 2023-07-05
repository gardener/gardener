---
categories:
  - Users
---

# Connectivity

## Shoot Connectivity

We measure the connectivity from the shoot to the API Server. This is done via the `blackbox exporter` which is deployed in the shoot's `kube-system` namespace. Prometheus will scrape the `blackbox exporter` and then the exporter will try to access the API Server. Metrics are exposed if the connection was successful or not. This can be seen in the `Kubernetes Control Plane Status` dashboard under the `API Server Connectivity` panel. The `shoot` line represents the connectivity from the shoot.

![image](images/panel.png)

## Seed Connectivity

In addition to the shoot connectivity, we also measure the seed connectivity. This means trying to reach the API Server from the seed via the external fully qualified domain name of the API server. The connectivity is also displayed in the above panel as the `seed` line. Both `seed` and `shoot` connectivity are shown below.

![image](images/connectivity.png)
