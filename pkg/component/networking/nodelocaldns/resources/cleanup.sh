#!/bin/sh
# Check if the nodelocaldns interface exists
if ip link show nodelocaldns > /dev/null 2>&1; then
    ip link delete nodelocaldns
    # Check if the operation was successful
    if [ $? -eq 0 ]; then
        echo "Nodelocaldns interface deleted successfully."
    else
        echo "An error occurred while deleting nodelocaldns interface."
        exit 1
    fi
else
    echo "Nodelocaldns interface does not exist. Skipping deletion."
fi

# Define the comment to search for
COMMENT="NodeLocal DNS Cache:"

# Delete all iptables rules with the specified comment
for cmd in iptables-legacy-save ip6tables-legacy-save; do
    rules=$($cmd | grep -- "--comment \"$COMMENT\"")
    if [ -n "$rules" ]; then
        echo "$rules" | while read -r line; do
            rule=$(echo "$line" | sed -e 's/^-A/-D/')
            if ! ${cmd%-save} $rule; then
                echo "Failed to delete rule in $cmd: $rule" >&2
                exit 1
            fi
        done
        echo "All rules with the comment \"$COMMENT\" have been deleted successfully from $cmd."
    else
        echo "No rules with the comment \"$COMMENT\" found for nodelocaldns in $cmd. Skipping deletion."
    fi
done
touch /tmp/healthy
sleep infinity
