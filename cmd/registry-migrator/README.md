# Etcd K8S prefix migrator

This tool allows for migrating of etcd prefixes. If the keys are stored in `/registry/foo`:

```bash
ETCDCTL_API=3 etcdctl get --keys-only --prefix "/registry/foo/"

/registry/foo/group.foo.bar/someresources/some-name
/registry/foo/group.bar.foo/anotherresource/some-name-two
```

And the the end-user wants to migrate them to `/registry/baz` then:

```bash
go run main.go --backup-file ./etcd.backup --old-registry-prefix "/registry/foo" --new-registry-prefix "/registry/baz"

INFO: 2020/01/22 10:49:46 parsed scheme: "endpoint"
INFO: 2020/01/22 10:49:46 ccResolverWrapper: sending new addresses to cc: [{http://localhost:2379 0  <nil>}]
{"level":"info","ts":1579682986.828646,"caller":"snapshot/v3_snapshot.go:110","msg":"created temporary db file","path":"./etcd.backup.part"}
{"level":"warn","ts":"2020-01-22T10:49:46.842+0200","caller":"clientv3/retry_interceptor.go:116","msg":"retry stream intercept"}
{"level":"info","ts":1579682986.8421938,"caller":"snapshot/v3_snapshot.go:121","msg":"fetching snapshot","endpoint":"http://localhost:2379"}
{"level":"info","ts":1579682990.963499,"caller":"snapshot/v3_snapshot.go:134","msg":"fetched snapshot","endpoint":"http://localhost:2379","took":4.134708947}
{"level":"info","ts":1579682990.964743,"caller":"snapshot/v3_snapshot.go:143","msg":"saved","path":"./etcd.backup"}
WARNING: 2020/01/22 10:50:06 grpc: addrConn.createTransport failed to connect to {http://localhost:2379 0  <nil>}: didn't receive server preface in time. Reconnecting...
WARNING: 2020/01/22 10:50:06 grpc: addrConn.createTransport failed to connect to {http://localhost:2379 0  <nil>}: didn't receive server preface in time. Reconnecting...
2020-01-22 10:50:07.848841 I | Starting from revision 293766
2020-01-22 10:50:07.848964 I | migrating "/registry/foo/group.foo.bar/someresources/some-name" => "/registry/baz/group.foo.bar/someresources/some-name"
2020-01-22 10:50:07.853314 I | migrating "/registry/foo/group.bar.foo/anotherresource/some-name-two" => "/registry/baz/group.bar.foo/anotherresource/some-name-two"
2020-01-22 10:50:08.700987 I | Migration successful.
```

```bash
ETCDCTL_API=3 etcdctl get --keys-only --prefix "/registry/baz/"

/registry/baz/group.foo.bar/someresources/some-name
/registry/baz/group.bar.foo/anotherresource/some-name-two
```

## Prerequisites

To ensure that no data is lost, it's **REQUIRED** to stop all K8S apiservers connected to that etcd cluster.

## What it does

1. Creates a snapshot of the etcd
1. Fetches all keys with the old prefix
1. In a transaction it, it sets the value of the new key to the old, checking the old revision key is not updated.
1. If `--force` is set to `false` it also checks if the new key already exists and stops.
1. If `--delete` is set to `true` it deletes the old keys after waiting for 5 seconds.
