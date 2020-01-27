// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/clientv3util"
	"github.com/coreos/etcd/clientv3/snapshot"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/pkg/transport"
	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"
)

const (
	dialTimeout       = 5 * time.Second
	keyExistsErrorMsg = `either key already exists or revision of the old key was changed when doing migration.
Use the "--force" flag to skip this check.
Aborting %+v.
`
)

func main() {
	var (
		oldRegistryPrefix,
		newRegistryPrefix,
		caFile,
		certFile,
		keyFile,
		endpoints,
		backup string
		delete, force bool
	)

	flag.StringVar(&oldRegistryPrefix, "old-registry-prefix", "/registry/garden.sapcloud.io", "default apiserver registry prefix")
	flag.StringVar(&newRegistryPrefix, "new-registry-prefix", "/registry-gardener", "new apiserver registry prefix")
	flag.StringVar(&caFile, "ca", "", "path to etcd's certificate authority file")
	flag.StringVar(&certFile, "cert", "", "path to etcd client public certificate file")
	flag.StringVar(&keyFile, "key", "", "path to etcd client certificate key file")
	flag.StringVar(&endpoints, "endpoints", "http://localhost:2379", "comma-separated list of etcd endpoints")
	flag.BoolVar(&force, "force", false, "forcefully override existing keys")
	flag.BoolVar(&delete, "delete", false, "deletes migrated keys")
	flag.StringVar(&backup, "backup-file", "", "file to store backup to")

	flag.Parse()

	clientv3.SetLogger(grpclog.NewLoggerV2(os.Stdout, os.Stdout, os.Stderr))

	if oldRegistryPrefix == "" {
		log.Fatalln("old-registry-prefix cannot be empty")
	}

	if newRegistryPrefix == "" {
		log.Fatalln("new-refistry-prefix cannot be empty")
	}

	if backup == "" {
		log.Fatalln("backup-file cannot be empty")
	}

	eps := strings.Split(endpoints, ",")
	if len(eps) == 0 {
		log.Fatalln("at least one endpoint must be provided")
	}

	cc := clientv3.Config{
		Endpoints:   eps,
		DialTimeout: dialTimeout,
	}

	switch {
	case caFile != "" && certFile != "" && keyFile != "":
		tlsInfo := transport.TLSInfo{
			TrustedCAFile: caFile,
			CertFile:      certFile,
			KeyFile:       keyFile,
		}

		tls, err := tlsInfo.ClientConfig()
		if err != nil {
			log.Fatal(err)
		}

		cc.TLS = tls
	case caFile == "" && certFile == "" && keyFile == "":
	default:
		log.Fatalf("--ca, --cert and --key must all be specified when one is provided")
	}

	cli, err := clientv3.New(cc)
	if err != nil {
		log.Fatal(err)
	}

	defer cli.Close()

	lg, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}

	dummy, err := cli.Get(context.TODO(), "/dummy-foo-bar", clientv3.WithKeysOnly())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("compacting to revision %d", dummy.Header.Revision)

	_, err = cli.Compact(context.TODO(), dummy.Header.Revision, clientv3.WithCompactPhysical())
	if err != nil {
		if err == rpctypes.ErrCompacted {
			log.Println("etcd is already compacted to the latest revision")
		} else {
			log.Fatal(err)
		}
	}

	log.Printf("compaction successful")

	sp := snapshot.NewV3(lg)

	err = sp.Save(context.TODO(), cc, backup)
	if err != nil {
		log.Fatal(err)
	}

	old, err := cli.Get(context.TODO(), oldRegistryPrefix,
		clientv3.WithPrefix(),
		clientv3.WithKeysOnly(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting from revision %d", old.Header.Revision)

	for _, k := range old.Kvs {
		key := string(k.Key)
		newKey := newRegistryPrefix + strings.TrimPrefix(key, oldRegistryPrefix)
		log.Printf("migrating %q => %q", key, newKey)

		out, err := cli.Get(context.TODO(), key)
		if err != nil {
			log.Fatal(err)
		}

		if len(out.Kvs) != 1 {
			log.Fatalf("key %q is missing", key)
		}

		compares := []clientv3.Cmp{clientv3.Compare(clientv3.ModRevision(key), "=", out.Kvs[0].ModRevision)}
		if !force {
			compares = append(compares, clientv3util.KeyMissing(newKey))
		}

		txn, err := cli.Txn(context.TODO()).
			If(compares...).
			Then(clientv3.OpPut(newKey, string(out.Kvs[0].Value))).
			Commit()
		if err != nil {
			log.Fatal(err)
		}

		if !txn.Succeeded {
			if !force {
				log.Fatalf(keyExistsErrorMsg, *txn.OpResponse().Txn())
			}
		}
	}

	if delete {
		log.Println("Deleting resources. You have 5 seconds to cancel")
		time.Sleep(time.Second * 5)

		// don't explicitly delete the entire key prefix
		for _, k := range old.Kvs {
			key := string(k.Key)
			log.Printf("Deleting %q", key)
			txn, err := cli.Txn(context.TODO()).
				If(clientv3.Compare(clientv3.ModRevision(key), "=", k.ModRevision)).
				Then(clientv3.OpDelete(key)).
				Commit()

			if err != nil {
				log.Fatal(err)
			}

			if !txn.Succeeded {
				log.Fatalf("key's revision was changed during the transaction. Aborting %+v.", *txn.OpResponse().Txn())
			}
		}
	}
}
