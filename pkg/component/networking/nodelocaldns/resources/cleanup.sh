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

# Check if there are any iptables rules with the specified comment
if iptables-legacy-save | grep -- "--comment \"$COMMENT\"" > /dev/null 2>&1; then
    # Find and delete all iptables rules with the specified comment
    iptables-legacy-save | grep -- "--comment \"$COMMENT\"" | while read -r line; do
        # Extract the rule specification
        rule=$(echo "$line" | sed -e 's/^-A/-D/')
        # Delete the rule
        iptables $rule
    done

    # Check if the operation was successful
    if [ $? -eq 0 ]; then
        echo "All iptables rules with the comment \"$COMMENT\" have been deleted successfully."
    else
        echo "An error occurred while deleting iptables rules."
        exit 1
    fi
else
    echo "No iptables rules with the comment \"$COMMENT\" found for nodelocaldns. Skipping deletion."
fi
touch /tmp/healthy
sleep infinity
