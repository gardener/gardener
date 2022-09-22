# Heartbeat Controller

The heartbeat controller renews a dedicated `Lease` object named `gardener-extension-heartbeat` at regular 30 second intervals by default. This `Lease` is used for heart beats similar to how `gardenlet` uses `Lease` objects for seed heart beats (see [gardenlet heartbeats](../concepts/gardenlet#heartbeats)).

The `gardener-extension-heartbeat` `Lease` can be checked by other controllers to verify that the corresponding extension controller is still running. Currently, `gardenlet` checks this `Lease` when performing shoot health checks and expects to find the `Lease` inside the namespace where the extension controller is deployed by the corresponding `ControllerInstallation`.

