# Shoot maintenance
Shoots configure a maintenance time window in which Gardener performs maintenance related tasks like infrastructure reconciliation, restarting pods, updating K8s or machine image versions.

## Restart control plane controllers
Gardener operators can make Gardener restart/delete certain control plane pods during a shoot's maintenance.
This feature helps to automatically solve service denials of controllers due to stale caches, dead-locks or starving routines.

Please note that these are exceptional cases but they are observed from time to time.
Gardener, for example, takes this precautionary measure for `Kube-Controller-Manager` pods. 

Extension controllers can extend the amount of pods being affected by these restarts.
If your Gardener extension manages pods of a shoot's control plane (shoot namespace in seed) and it could potentially profit from a regular restart please consider labeling it with `maintenance.gardener.cloud/restart: true`.