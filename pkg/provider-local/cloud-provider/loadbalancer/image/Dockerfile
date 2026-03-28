FROM envoyproxy/envoy:v1.37.0

# Load balancers for local shoots need to route traffic to shoot nodes. Shoot nodes are machine pods in the kind cluster
# but are not directly reachable by the load balancer container (in the kind network). To route traffic to shoot nodes,
# we configure IP routes via the kind nodes hosting the corresponding shoot machine pods. This requires the `ip` command
# to be available in the load balancer container.
RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends iproute2 && \
    rm -rf /var/lib/apt/lists/*
