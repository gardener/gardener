## Etcd backup restorer
Copyright (c) 2017-2018 SAP SE or an SAP affiliate company. All rights reserved.

## Seed Source

The source code of this component was seeded based on a copy of the following files:

### Etcd

https://github.com/coreos/etcd. <br>
Copyright 2017 The etcd Authors.<br>
Apache 2 license (https://github.com/coreos/etcd/blob/v3.3.1/LICENSE).

Release: v3.3.1.<br>
Commit-ID: 28f3f26c0e303392556035b694f75768d449d33d.<br>
Commit-Message: version: 3.3.1.<br>
To the left are the list of copied files -> and to the right the current location they are at.
* etcdctl/ctlv3/command/snapshot_command.go ->  pkg/restorer/restorer.go


### google-cloud-go-testing

https://github.com/googleapis/google-cloud-go-testing.<br>
Copyright 2018 Google LLC.<br>
Apache 2 license (https://github.com/googleapis/google-cloud-go-testing/blob/master/LICENSE).

Commit-ID: f97d15acea6097507e8f3552353e64f7d2b4d4cc.

To the left are the list of copied files -> and to the right the current location they are at.
* storage/stiface/interfaces.go ->  pkg/snapstore/gcs/interfaces.go
* storage/stiface/adapters.go -> pkg/snapstore/gcs/adapters.go
