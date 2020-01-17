# Shoot Cluster Purpose

The `Shoot` resource contains a `.spec.purpose` field indicating how the shoot is used whose allowed values are as follows:

* `evaluation` (default): Indicates that the shoot cluster is for evaluation scenarios.
* `development`: Indicates that the shoot cluster is for development scenarios.
* `testing`: Indicates that the shoot cluster is for testing scenarios.
* `production`: Indicates that the shoot cluster is for production scenarios.
* `infrastructure`: Indicates that the shoot cluster is for infrastructure scenarios (only allowed for shoots in the `garden` namespace).

## Behavioral Differences

So far, the only behavioral difference is that

* `testing` shoot clusters **do not** get a monitoring or a logging stack as part of their control planes.

## Future Steps

We might introduce more behavioral difference depending on the shoot purpose in the future.
As of today, there are no plans yet.
