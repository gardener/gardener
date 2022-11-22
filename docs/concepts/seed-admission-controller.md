# Gardener Seed Admission Controller

The Gardener Seed admission controller is deployed by the Gardenlet as part of its seed bootstrapping phase and, consequently, running in every seed cluster.
It's main purpose is to serve webhooks (validating or mutating) in order to admit or deny certain requests to the seed's API server.
