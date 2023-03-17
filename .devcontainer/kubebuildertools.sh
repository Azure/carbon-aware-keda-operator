#!/bin/bash​
# install kubectl​
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
rm kubectl

# install kubebuilder​
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)  
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

# install kind​
LATEST_VERSION=$(curl https://api.github.com/repos/kubernetes-sigs/kind/releases/latest | jq -r '.tag_name')
OS_ARCH=$(dpkg --print-architecture)
curl -Lo ./kind https://kind.sigs.k8s.io/dl/${LATEST_VERSION}/kind-linux-${OS_ARCH}
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind