#!/bin/sh
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


token=$1

get_http_status() {
    wget -S --spider --no-check-certificate -qO- --header "Authorization: Bearer ${token}" "https://127.0.0.1/$1?timeout=10s" 2>&1 | grep "HTTP/" | awk '{print $2}'
}

# Make sure that kube-apiserver process is running and then wait 60 seconds for it to start up.
echo "$(date): Getting pid of kube-apiserver"
pid=$(pgrep kube-apiserver)
retries=0
while [ -z "$pid" ] && [ $retries -lt 12 ]; do
    pid=$(pgrep kube-apiserver)
    retries=$((retries+1))
    sleep 5
done
if [ -z "$pid" ]; then
    echo "$(date): Could not find kube-apiserver process, exiting."
    exit 1
fi
echo "$(date): Found pid ${pid}"

sleep 15

echo "$(date): Starting kube-apiserver health checks"
counter=0
while true; do
    # When shutdown-send-retry-after=true is set on the kube-apiserver,
    # requests will return a 429 status code response when the kube-apiserver is shutting down.
    if get_http_status "version" | grep -q 429; then
        echo "$(date): kube-apiserver is probably not running properly, trying /readyz endpoint to confirm if it is stuck in shutdown"
        # When the kube-apiserver is shutting down it's readiness endpoint returns status code 500.
        # This check is added to make sure that the 429 code above is not returned due to request rate limiting.
        if get_http_status "readyz" | grep -q 500; then
            echo "$(date): /readyz returned status code 500"
            counter=$((counter+1))
        else
            echo "$(date): Could not confirm that kube-apiserver is stuck during shutdown"
            counter=0
        fi
    else
        counter=0
    fi

    # If 5 consecutive probes confirm that the kube-apiserver is shutting down, it is very likely that the kube-apiserver is stuck
    # ref: https://github.com/kubernetes/kubernetes/pull/113741
    if [ $counter -ge 5 ]; then
        echo "$(date): Detected that kube-apiserver is stuck during shutdown"
        pid=$(pgrep kube-apiserver)

        if [ -z "$pid" ]; then
            echo "$(date): could not find kube-apiserver pid"
            sleep 10
            continue
        fi

        kill -s TERM "${pid}"
        echo "$(date): Sent TERM signal to kube-apiserver process"

        new_pid=$(pgrep kube-apiserver)
        retries=0
        while { [ -z "$new_pid" ] || [ "$new_pid" = "$pid" ]; } && [ $retries -lt 12 ]; do
            new_pid=$(pgrep kube-apiserver)
            retries=$((retries+1))
            sleep 5
        done
        if [ -z "$new_pid" ] || [ "$new_pid" = "$pid" ] ; then
            echo "$(date): Could not find new kube-apiserver process, exiting."
            exit 1
        fi
        echo "$(date): New kube-apiserver process has started with pid ${pid}."
        sleep 5
    fi

    sleep 10
done
