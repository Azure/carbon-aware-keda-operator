# Building Carbon Awareness with KEDA - KubeConNA 2023 Demo

This walkthrough will enable you to run the demo that was [presented at KubeConNA 2023](https://kccncna2023.sched.com/event/5c8392bdd9871c46921c90493376abfa). The demo will show you how to use the Carbon Aware KEDA Operator to scale a workload based on the carbon intensity forecast of a region.

Before you begin, you will need the following:

- [Docker](https://docs.docker.com/get-docker/)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [helm](https://helm.sh)
- [jq](https://stedolan.github.io/jq/download/)
- [WattTime account](https://www.watttime.org/api-documentation/#register-new-user)

Start by creating a new KIND cluster.

```sh
kind create cluster
```

Install the Carbon Aware KEDA Operator.

```sh
kubectl apply -f "https://github.com/Azure/carbon-aware-keda-operator/releases/download/v0.2.0/carbonawarekedascaler-v0.2.0.yaml"
```
Sign up for a free WattTime account [here](https://www.watttime.org/api-documentation/#register-new-user) and export your WattTime credentials as environment variables.

```sh
export WT_USERNAME=<REPLACE_WITH_YOUR_WATTIME_USERNAME>
export WT_PASSWORD=<REPLACE_WITH_YOUR_WATTIME_PASSWORD>
export REGION=westus
```

Install the Kubernetes Carbon Intensity Exporter.

```sh
helm install carbon-intensity-exporter oci://ghcr.io/azure/kubernetes-carbon-intensity-exporter/charts/carbon-intensity-exporter \
  --version v0.3.0 \
  --set carbonDataExporter.region=$REGION \
  --set wattTime.username=$WT_USERNAME \
  --set wattTime.password=$WT_PASSWORD
```

Deploy a sample workload.

```sh
kubectl apply -f hack/workload/deployment.yaml
```

Install KEDA to scale the sample workload.

```sh
helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm install keda kedacore/keda --namespace keda --create-namespace
```

Deploy a ScaledObject to scale the sample workload.

```sh
kubectl apply -f hack/workload/scaledobject.yaml
```

Check the status of the Kubernetes Carbon Intensity Exporter.

```sh
kubectl get po -n kube-system -lname=api-server-svc
```

Once you see the pod is running, you can check the ConfigMap that contains the carbon intensity data.

```sh
kubectl get cm -n kube-system carbon-intensity -o jsonpath={.data} | jq
```

To view individual data points, you can run the following command.

```sh
kubectl get cm -n kube-system carbon-intensity -o jsonpath={.binaryData.data} | base64 --decode | jq
```

To determine the thresholds for the Carbon Aware KEDA Scaler, you can run the following commands.

```sh
minForecast=$(kubectl get cm -n kube-system carbon-intensity -o jsonpath='{.data}' | jq .minForecast | tr -d '"')
maxForecast=$(kubectl get cm -n kube-system carbon-intensity -o jsonpath='{.data}' | jq .maxForecast | tr -d '"')
thresholdBucketSize=$(echo "scale=0; (${maxForecast} - ${minForecast}) / 3" | bc)
low=$(echo "${minForecast} + ${thresholdBucketSize}" | bc | awk '{print int($1+0.5)}')
medium=$(echo "${low} + ${thresholdBucketSize}" | bc | awk '{print int($1+0.5)}')
high=$(echo "${medium} + ${thresholdBucketSize}" | bc | awk '{print int($1+0.5)}')
echo "minForecast: ${minForecast}\nmaxForecast: ${maxForecast}\nthresholdBucketSize: ${thresholdBucketSize}"
echo "lowThreshold: ${low}\nmediumThreshold: ${medium}\nhighThreshold: ${high}"
```

Finally you can deploy the Carbon Aware KEDA Scaler CRD to make your workload carbon aware.

```sh
kubectl apply -f - <<EOF                                
apiVersion: carbonaware.kubernetes.azure.com/v1alpha1
kind: CarbonAwareKedaScaler
metadata:
  labels:
    app.kubernetes.io/name: carbonawarekedascaler
    app.kubernetes.io/instance: carbonawarekedascaler-sample
    app.kubernetes.io/part-of: carbon-aware-keda-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: carbon-aware-keda-operator
  name: carbon-aware-word-processor-scaler
spec:
  kedaTarget: scaledobjects.keda.sh
  kedaTargetRef:
    name: word-processor-scaler
    namespace: default
  carbonIntensityForecastDataSource:
   localConfigMap:  
      name: carbon-intensity
      namespace: kube-system
      key: data
  maxReplicasByCarbonIntensity:
    - carbonIntensityThreshold: <REPLACE_WITH_YOUR_LOW_THRESHOLD>
      maxReplicas: 110
    - carbonIntensityThreshold: <REPLACE_WITH_YOUR_MEDIUM_THRESHOLD>
      maxReplicas: 60
    - carbonIntensityThreshold: <REPLACE_WITH_YOUR_HIGH_THRESHOLD>
      maxReplicas: 10
  ecoModeOff:
    maxReplicas: 100
    carbonIntensityDuration:
      carbonIntensityThreshold: 440
      overrideEcoAfterDurationInMins: 480
    customSchedule:
     - startTime: "2023-11-10T17:45:00Z"
       endTime: "2023-11-10T18:00:59Z"
    recurringSchedule:
      - "* 23 * * 4-5"
EOF
```

To view the logs of the Carbon Aware KEDA Operator, you can run the following command.

```sh
kubectl logs -n carbon-aware-keda-operator-system -l control-plane=controller-manager
```

You should see max replicas being set on the scaled object based on the carbon intensity forecast and the thresholds you set. To confirm this, you can run the following command.

```sh
kubectl get scaledobject -A
```

To visualize the Carbon Aware KEDA Operator in action, you can run the following commands to install the Prometheus Operator and Grafana.

```sh
kubectl apply --server-side -f hack/prometheus/manifests/setup
kubectl apply -f hack/servicemonitor
kubectl apply --server-side -f hack/prometheus/manifests
```

Wait until the Prometheus Operator and Grafana is running.

```sh
kubectl get po -n monitoring -w
```

Once the Prometheus Operator and Grafana is running, you can port forward Grafana to view the Carbon Aware KEDA Operator in action.

```sh
kubectl port-forward svc/grafana 3000:3000 -n monitoring
```

Open a web browser and navigate to http://localhost:3000. Login into Grafana using the default username and password of `admin` and `admin`. Then dashboard menu item, click on import then import the [`hack/grafana/Carbon Aware KEDA-Dashboard.json`](../hack/grafana/Carbon Aware KEDA-Dashboard.json) file and set Prometheus as the data source.

You should start to see data being populated in the dashboard.

To clean up the demo, you can run the following commands.

```sh
kind delete cluster
```