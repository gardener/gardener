{{- define "clean-sandbox-script" -}}
#!/bin/bash -eu
sandbox_name=$1
echo "Cleaning sandbox $sandbox_name"
SANDBOX_ID=$(/usr/local/bin/crictl pods --name $sandbox_name --output json | jq -r .items[0].id)
/usr/local/bin/crictl stopp $SANDBOX_ID
/usr/local/bin/crictl rmp $SANDBOX_ID
{{- end -}}