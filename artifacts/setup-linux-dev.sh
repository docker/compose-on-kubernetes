#!/bin/sh
set -e
if [ "$(uname)" == "Darwin" ]; then
    HOST_IP=$(ipconfig getifaddr en0)
elif [ "$OS" == "Windows_NT" ]; then
    HOST_IP=$(powershell -Command "(Test-Connection -ComputerName \$env:COMPUTERNAME -Count 1).IPV4Address.IPAddressToString")
else
    HOST_IP=$(hostname -I | awk '{print $1}')
fi
kubectl apply -f ns.yaml
cat service.dev.linux.yaml | sed -e "s/<host ip>/${HOST_IP}/g" | kubectl apply -f -
kubectl apply -f apiservice.yaml