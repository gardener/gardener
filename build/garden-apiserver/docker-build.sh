#!/bin/bash -eu
#
# Copyright 2018 The Gardener Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/bin/bash -e

REL_DIR="$(dirname $0)/../../bin/rel"
REPOSITORY="github.com/gardener/gardener"
BINARY="garden-apiserver"

mkdir -p $REL_DIR

sudo docker run \
    --cidfile=$BINARY-cid \
    -v $PWD:/go/src/$REPOSITORY:ro \
    -w /go/src/$REPOSITORY \
    golang:1.9.2-alpine3.7 \
    /bin/sh -x -c \
    'apk add --no-cache --update alpine-sdk && make apiserver-build-release'

sudo docker cp $(cat $BINARY-cid):/go/bin/$BINARY $REL_DIR/$BINARY
sudo docker rm $(cat $BINARY-cid)
sudo rm $BINARY-cid
