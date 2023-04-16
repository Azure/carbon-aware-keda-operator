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



## Installation





# TODO: As the maintainer of this project, please make a few updates:

- Improving this README.MD file to provide a great experience
- Updating SUPPORT.MD with content about this project's support experience
- Understanding the security reporting process in SECURITY.MD
- Remove this section from the README

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
