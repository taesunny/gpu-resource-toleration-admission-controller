#!/usr/bin/env bash
# certificates documentations
# https://kubernetes.io/ko/docs/concepts/cluster-administration/certificates/

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

function red(){
    echo -e "${RED}${1}${NC}"
}
function green(){
    echo -e "${GREEN}${1}${NC}"
}

basedir="$(pwd)"
keydir="$(mktemp -d)"
chmod 0700 "$keydir"
cd "$keydir"

green "Generate the CA cert and private key (ca -days 10000)"
openssl req -nodes -new -x509 -keyout ca.key -out ca.crt -subj "/CN=gpu-resource-toleration-admission-controller CA" -days 10000

green "Generate the private key for the gpu-resource-toleration-webhook-server"
openssl genrsa -out gpu-resource-toleration-webhook-server-tls.key 2048

green "Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA."
openssl req -new -key gpu-resource-toleration-webhook-server-tls.key -subj "/CN=gpu-resource-toleration-admission-controller.kube-system.svc" \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial -out gpu-resource-toleration-webhook-server-tls.crt -days 10000

green "Create the TLS secret for the generated keys"
if kubectl -n kube-system create secret tls gpu-resource-toleration-admission-controller-webhook-certs \
    --cert "${keydir}/gpu-resource-toleration-webhook-server-tls.crt" \
    --key "${keydir}/gpu-resource-toleration-webhook-server-tls.key"
then
    green "TLS secret created"
else
    red "TLS secret exist"
fi

# Read the PEM-encoded CA certificate, base64 encode it, and replace the `<CA_BUNDLE>` placeholder in the YAML
# template with it. Then, create the Kubernetes resources.
green "create admission-controller"
CA_BUNDLE="$(openssl base64 -A <"${keydir}/ca.crt")"
sed -e 's@<CA_BUNDLE>@'"$CA_BUNDLE"'@g' <"${basedir}/gpu-resource-toleration-admission-controller.yaml" \
    | kubectl create -f -

cd "$basedir"
green "Delete the key directory to prevent abuse"
rm -r "$keydir"