#!/bin/sh

token=$1

# Make sure that kube-apiserver process is running and then wait 15 seconds for it to start up.
pid=$(pgrep kube-apiserver)
retries=0
while [ -z "$pid" ] && [ $retries -lt 12 ]; do
    pid=$(pgrep kube-apiserver)
    retries=$((retries+1))
    sleep 5
done
if [ -z "$pid" ]; then
    echo "$(date) Could not find kube-apiserver process, exiting."
    exit 1
fi

sleep 15

counter=0
while true; do
    resp=$(wget --no-check-certificate -qO- --header "Authorization: Bearer ${token}" https://127.0.0.1/version?timeout=10s 2>&1)

    # When shutdown-send-retry-after=true is set on the kube-apiserver,
    # requests will return a 429 status code response when the kube-apiserver is shutting down.
    if echo "${resp}" | grep 429; then
        echo "$(date): kube-apiserver is probably not running properly, trying /readyz endpoint to confirm if it is stuck in shutdown"
        # When the kube-apiserver is shutting down it's readiness endpoint returns status code 500.
        # This check is added to make sure that the 429 code above is not returned due to request rate limiting.
        resp=$(wget --no-check-certificate -qO- --header "Authorization: Bearer ${token}" https://127.0.0.1/readyz?timeout=10s 2>&1)
        if echo ${resp} | grep 500; then
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
            echo "could not find kube-apiserver pid"
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
        echo "$(date): New kube-apiserver process has started."
        sleep 5
    fi

    sleep 10
done
