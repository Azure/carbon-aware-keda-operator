# Test using local KIND cluster in GitHub Codespaces

This repo includes [DevContainer configuration](../.devcontainer) so that you can try the operator in a [Visual Studio Code Dev Container](https://code.visualstudio.com/docs/devcontainers/containers) on your local machine or within a [GitHub Codespace](https://github.com/features/codespaces).

> To run this DevContainer on your local machine you must have Docker CLI installed.

## Create a new GitHub Codespace

Browse to the repo and click the **<> Code** button.

Make sure you click on the **Codespaces** tab and click the ellipsis on the right to open a menu. Here, you want to ensure that you are creating an appropriately sized Codespace.

![codespace options](../assets/images/codespace-options.png)

Since you will be running a KIND cluster in your Codespace, you will need to size up the machine type to 4-core, 8GB RAM at minimum.

![codespace machine type](../assets/images/codespace-machine-type.png)

Click the **Create Codespace** button a give it a few minutes for the container to be created.

## Deploy the operator to KIND cluster

The Codespace has all the tools you need to get started. All you need to do is simply open the Terminal in VS Code and run the following command:

```bash
make kind-deploy-prom IMG=carbon-aware-keda-operator:v1
```

The command above will create a new KIND cluster and deploy the operator along with a sample workload. 

The `CarbonAwareKedaScaler` custom resource will be configured to use mock carbon intensity forecast data. If you have WattTime API credentials, you can deploy the open-source **Carbon Intensity Exporter** operator by following the instructions [here](https://github.com/Azure/kubernetes-carbon-intensity-exporter/).

> Remember to update your `CarbonAwareKedaScaler` custom resource and set the `mockCarbonForecast` property to `false`.

## Visualize the operator in action using Grafana

This operator logs custom metrics to the `/metrics` endpoint which can be scraped by Prometheus. The `kube-prometheus` stack has been deployed in the KIND cluster and configured to scrape metrics from the `carbon-aware-keda-operator-system` namespace. You will need to import the sample dashboard to Grafana.

Enable port-forwarding on the Grafana service.

```bash
kubectl port-forward svc/grafana 3000:3000 -n monitoring
```

In your web browser, navigate to http://localhost:3000/ and log in using the default username `admin` with default password `admin`. You will be prompted to create a new password.

Download the sample dashboard [here](https://github.com/Azure/carbon-aware-keda-operator/blob/main/hack/grafana/Carbon%20Aware%20KEDA-Dashboard.json).

Expand the **Dashboards** menu item and click the **+ Import** button.

![grafana dashboard import](../assets/images/grafana-import.png)

Upload the **Carbon Aware KEDA-Dashboard.json** file and select **prometheus** as the data source then click Import.

![grafana dashboard datasource](../assets/images/grafana-dashboard.png)

You will be able to view the default max replicas, and the max replicas ceiling being raised and lowered over time based on the carbon intensity rating.

![carbon aware dashboard](../assets/images/carbon-aware-dashboard.png)
