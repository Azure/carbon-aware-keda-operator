// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package controllers

import (
	"fmt"
	"sort"

	carbonawarev1alpha1 "github.com/azure/carbon-aware-keda-operator/api/v1alpha1"
)

func getMaxReplicas(forecast *CarbonForecast, configs []carbonawarev1alpha1.CarbonIntensityConfig) (*int32, error) {
	// if there is no forecast for the current hour, revert back to the original maxReplicaCount from the operand
	if forecast == nil {
		return nil, fmt.Errorf("no forecast data")
	} else {
		ci := forecast.Value

		// sort to ensure that configured carbon intensity thresholds are sorted is in ascending order to better evaluate lower and upper bounds
		sort.Slice(configs, func(i, j int) bool {
			return configs[i].CarbonIntensityThreshold < configs[j].CarbonIntensityThreshold
		})

		// loop through the carbon intensity configs and find where the current carbon intensity falls within the range
		for index, element := range configs {
			var lowerBound float64 = 0

			// if this is not the first element in the list, set the lower bound to the previous element's CarbonIntensityThreshold
			if index > 0 {
				lowerBound = float64(configs[index-1].CarbonIntensityThreshold)
			}

			// set the upper bound to the current element's CarbonIntensityThreshold
			var upperBound float64 = float64(element.CarbonIntensityThreshold)

			// if the carbon intensity is within range, set the max replica count and return it
			if ci > lowerBound && ci <= upperBound {
				return element.MaxReplicas, nil
			}
		}

		// if the carbon intensity is not within range, return the max replica count from the last element in the list
		return configs[len(configs)-1].MaxReplicas, nil
	}
}
