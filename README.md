# Carbon Aware KEDA Operator

This repo provides a Kubernetes operator that aims to reduce carbon emissions by helping KEDA scale Kubernetes workloads based on carbon intensity. Carbon intensity is a measure of how much carbon dioxide is emitted per unit of energy consumed. By scaling workloads according to the carbon intensity of the region or grid where they run, we can optimize the carbon efficiency and environmental impact of our applications.

The operator uses carbon intensity data from third party sources such as [WattTime](https://www.watttime.org/), [Electricity Map](https://www.electricitymap.org/) or any other provider, to dynamically adjust the scaling behavior of KEDA. The operator does not require any application or workload code change, and it works with any KEDA scaler.

To read more about carbon intensity and carbon awareness, please check out the [Green Software Foundation](https://learn.greensoftware.foundation/carbon-awareness/).

## How it works


![image](https://user-images.githubusercontent.com/966110/232306667-7717bb52-fc2e-4564-9c75-d820ab3bf58b.png)

 1 - Getting the carbon intensity data:

The carbon aware KEDA operator retrieves the carbon intensity data from the carbon aware SDK, which is a wrapper for electricity data APIs. 

The operator retrieves 24-hour carbon intensity forecast data every 12 hours. Upon successful data pull, the old configmap will be deleted and a new configmap with the same name will be created. 

Any other Kubernetes operator can read the configmap for utilizing the carbon intensity data.

<br>

 2 – Making carbon aware scaling decisions:

Step 0 – As an admin you create a CarbonAwareKedaScaler spec for targetRef : scaledObject or scaledJob

Then the operator will update KEDA scaledObjects and scaledJob `maxReplicaCount` field, based on the current carbon intensity.



## Current carbon aware scaling logic 

The current logic for carbon aware scaling is based on carbon intensity metric only, which is independent of the workload usage today.

The operator will not compute a desired replicaCount for your scaledObjects or scaledJobs, as this is the responsibility of KEDA and HPA. The operator would define a ceiling for allowed maxReplicas based on carbon intensity of the current time.

In practice, the operator will throttle workloads and prevent them from bursting during high carbon intensity periods, and allow more scaling when intensity is lower.

## Use cases 
 
This operator can be used for low priority and time flexible workloads that support interruptions, for example:

- noncritical data backups
- batch processing jobs
- Processing of data analytics
- ML Training jobs
- (some) CICD jobs
- Dev & Test environments



## How to use it

Once the "carbon aware KEDA operator" installed, you can now deploy a custom resource called `CarbonAwareKedaScaler` to set the max replicas, KEDA can scale up to, based on carbon intensity.

```bash
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
  carbonIntensityForecastDataSource:       # carbon intensity forecast data source 
    mockCarbonForecast: false              # [OPTIONAL] use mock carbon forecast data 
    localConfigMap:                        # [OPTIONAL] use configmap for carbon forecast data 
      name: carbon-intensity 
      namespace: kube-system
      key: data 
  maxReplicasByCarbonIntensity:            # array of carbon intensity values in ascending order; each threshold value represents the upper limit and previous entry represents lower limit 
    - carbonIntensityThreshold: 437        # when carbon intensity is 437 or below 
      maxReplicas: 110                     # do more 
    - carbonIntensityThreshold: 504        # when carbon intensity is >437 and <=504 
      maxReplicas: 60 
    - carbonIntensityThreshold: 571        # when carbon intensity is >504 and <=571 (and beyond) 
      maxReplicas: 10                      # do less 
  ecoModeOff:                              # [OPTIONAL] settings to override carbon awareness; can override based on high intensity duration or schedules 
    maxReplicas: 100                       # when carbon awareness is disabled, use this value 
    carbonIntensityDuration:               # [OPTIONAL] disable carbon awareness when carbon intensity is high for this length of time 
      carbonIntensityThreshold: 555        # when carbon intensity is equal to or above this value, consider it high 
      overrideEcoAfterDurationInMins: 45   # if carbon intensity is high for this many hours disable ecomode 
    customSchedule:                        # [OPTIONAL] disable carbon awareness during specified time periods 
      - startTime: "2023-04-28T16:45:00Z"  # start time in UTC 
        endTime: "2023-04-28T17:00:59Z"    # end time in UTC 
    recurringSchedule:                     # [OPTIONAL] disable carbon awareness during specified recurring time periods 
      - "* 23 * * 1-5"                     # disable every weekday from 11pm to 12am UTC 
EOF
```


## Installation & demo

To install the Carbon Aware KEDA Operator, please check out the following links.
-	[Install on AKS](carbon-aware-keda-operator/azure.md at main · Azure/carbon-aware-keda-operator (github.com))
-	[Install on Kind](carbon-aware-keda-operator/kind.md at main · Azure/carbon-aware-keda-operator (github.com))


## How to set the carbon intensity thresholds

When adding `maxReplicasByCarbonIntensity` entries in the custom resource, it is important to understand what the carbon intensity thresholds are since they vary between regions. It is recommended that you do your best to find the minimum and maximum carbon intensity values and set thresholds accordingly.

> Remember, when energy is dirty (e.g., carbon intensity is high), do less, and when energy is clean (e.g., carbon intensity is low), do more.

To set the thresholds, the idea is to find the range between minimum and maximum carbon intensity ranges and divide them into “buckets”. In the example above, the three thresholds could represent “low”, “medium”, and “high” where a carbon intensity value of 565 and below is considered low, 566 – 635 is medium, and 636 or more is high. Configuring thresholds in an array like this gives you flexibility to create as many thresholds/buckets as needed.

## How to set the allowed maxReplicas per carbon intensity 

make this paragrpah in MD format; don"t add other text,just transform to MD format
 
It’s up to you as an admin or a developer, to decide of the carbon aware scaling behavior for your workload :
- You could decide to enable carbon awareness only when carbon intensity is in the high rates.
- You could scale to zero during high carbon intensity periods, or keep a minimal replicas running for your workload.
- Depending on the nature of the workload and its constraints, you would decide what scaling limits to use for you workload.


# Exported Metrics

The following metrics are exported by the operator:

- `carbon_intensity`: The carbon intensity of the electricity grid region where Kubernetes cluster is deployed
- `MaxReplicas`: The maximum number of replicas that can be scaled up to by the KEDA scaledObject or scaledJob.
- `Default MaxReplicas`: The default value of `MaxReplicas` when carbon awanress is disabled, aka "ecoMode off".


## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft 
trademarks or logos is subject to and must follow 
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
