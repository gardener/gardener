# Gardener Scheduler

The Gardener Scheduler is in essence a controller that watches newly created shoots and assigns a Seed cluster to them.
Conceptually, the task of the Gardener Scheduler is very similar to the task of the Kubernetes scheduler: finding a seed for a shoot instead of a node for a pod.

A scheduling strategy hereby determines how the Scheduler is operating. 
The following sections explain the configuration and flow in greater detail.
#### Why is the Gardener Scheduler needed?

**1. Decoupling**
- Previously, an admission plugin in the Gardener API server conducted the scheduling decisions.
This implies changes to the API server whenever adjustments of the scheduling are needed. 
Decoupling the API server and the Scheduler gives greater flexibility to develop these components independently from each other.
 
**2. Extensibility**
- It should be possible to easily extend and tweak the Scheduler in the future. 
Possibly, similar to the Kubernetes scheduler, hooks could be provided that influence the scheduling decisions.
It should be also possible to completely replace the standard Gardener Scheduler with a custom implementation.

#### Configuration

The Gardener Scheduler configuration has to be supplied on startup. It is a mandatory and also the only available flag.
[Here](../../example/20-componentconfig-gardener-scheduler.yaml) is an example scheduler configuration.

Most of the configuration options are the same as in the Gardener Controller Manager (Leader Election, Client Connection, ...).
However, the Gardener Scheduler on the other hand does not need a TLS configuration, because there are currently no Webhooks configurable.
The Scheduling Strategy is defined in the _**candidateDeterminationStrategy**_ and can have the possible values _SameRegion_ and _MinimalDistance_.
The SameRegion Strategy is the default Strategy.

**1. Same Region Strategy**

-  The Gardener Scheduler reads the Spec.Cloud.Profile and Spec.Cloud.Region fields from the shoot resource.
It tries to find a Seed that has the identical Spec.Cloud.Profile and Spec.Cloud.Region fields set.
If it cannot find a suitable Seed, it adds an event to the shoot stating, that it is unschedulable.

**2. Minimal Distance Strategy**

-  As a first step, the _SameRegion_ Strategy is being executed.
If no seed in the same region could be found, the scheduler uses a lexicographical approach to determine a suitable seed cluster.
This leverages the fact that most cloud providers (except from Azure) use geographically aligned region names.
The Scheduler takes into consideration the region names of all available seeds in the cluster of the desired infrastructure and picks the regions that match lexicographically the best (starting from the left letter to right letter of the region name). 
E.g. if the shoots wants a cluster in AWS eu-north-1, the Scheduler picks all Seeds in region AWS eu-central-1, because at least the continent “eu-“ matches (even better with region instances like AWS ap-southeast-1 and AWS ap-southeast-2). 


In the last step, the scheduler picks the one seed having the least shoots currently deployed.

In order to put the scheduling decision into effect, the Scheduler sends an update request for the shoot resource to the API server. After validation, the Gardener Aggregated API server updates the shoot to have the Spec.Cloud.Seed field set. 
Subsequently the Gardener Controller Manager picks up and starts to create the cluster on the specified seed.

**Failure to determine a suitable seed**

In case the scheduler fails to find a suitable seed, the operation is being retried with an exponential backoff - starting with the  _retrySyncPeriod_ (Default of 15 seconds).

#### Current Limitation / Future Plans

- Azure has unfortunately a geographically non-hierarchical naming pattern and does not start with the continent. This is the reason why we will exchange the implementation of the _MinimalRegion_ Strategy with a more suitable one in the future.
- Currently, shoots can only scheduled to seeds from the same cloud provider (Spec.Cloud.Profile), however that is not a technical limitation and might be changed in the future.
