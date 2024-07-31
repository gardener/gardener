# Shoot Maintenance

There is a general [document about shoot maintenance](../usage/shoot_settings/shoot_maintenance.md) that you might want to read.
Here, we describe how you can influence certain operations that happen during a shoot maintenance.

## Restart Control Plane Controllers

As outlined in the above linked document, Gardener offers to restart certain control plane controllers running in the seed during a shoot maintenance.

Extension controllers can extend the amount of pods being affected by these restarts.
If your Gardener extension manages pods of a shoot's control plane (shoot namespace in seed) and it could potentially profit from a regular restart, please consider labeling it with `maintenance.gardener.cloud/restart=true`.
