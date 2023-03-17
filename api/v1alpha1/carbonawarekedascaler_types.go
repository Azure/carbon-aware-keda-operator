/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Reasons why operator is in degraded status
const (
	ReasonSucceeded             = "OperatorSucceeded"
	ReasonTargetUpdateFailed    = "OperatorTargetUpdateFailed"
	ReasonTargetNotFound        = "OperatorTargetNotFound"
	ReasonTargetFetchError      = "OperatorTargetFetchError"
	ReasonCarbonDataFetchError  = "OperatorCarbonDataFetchError"
	ReasonMaxReplicasCountError = "OperatorMaxReplicasCountError"
	ReasonEcoModeDisabledError  = "OperatorEcoModeDisabledError"
	ReasonEcoModeDisabled       = "OperatorEcoModeDisabled"
)

// KedaTargetRef represents the KEDA object to scale
type KedaTargetRef struct {
	// name of the keda target
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// namespace of the keda target
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// EcoModeOff represents the configuration to disable carbon aware scaler
type EcoModeOff struct {
	// default maximum number of replicas when carbon aware scaler is disabled
	// +kubebuilder:validation:Required
	MaxReplicas int32 `json:"maxReplicas"`

	// disable carbon aware scaler when carbon intensity is above a threshold for a specific duration
	// +kubebuilder:validation:Optional
	CarbonIntensityDuration CarbonIntensityDuration `json:"carbonIntensityDuration,omitempty"`

	// disable carbon aware scaler at specific time periods
	// +kubebuilder:validation:Optional
	CustomSchedule []Schedule `json:"customSchedule,omitempty"`

	// disable carbon aware scaler on a recurring schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:Optional
	RecurringSchedule []string `json:"recurringSchedule,omitempty"`
}

// Schedule represents a time period to disable carbon aware scaler
type Schedule struct {
	// start time in utc
	// +kubebuilder:validation:Required
	StartTime string `json:"startTime"`

	// end time in utc
	// +kubebuilder:validation:Required
	EndTime string `json:"endTime"`
}

// CarbonIntensityDuration represents the configuration to disable carbon aware scaler when carbon intensity is above a threshold for a specific duration
type CarbonIntensityDuration struct {
	// carbon intensity threshold to disable carbon aware scaler
	// +kubebuilder:validation:Required
	CarbonIntensityThreshold int32 `json:"carbonIntensityThreshold"`

	// length of time in minutes to disable carbon aware scaler when the carbon intensity threshold meets or exceeds carbonIntensityThreshold
	// +kubebuilder:validation:Required
	OverrideEcoAfterDurationInMins int32 `json:"overrideEcoAfterDurationInMins"`
}

// CarbonIntensityConfig represents the configuration to scale the number of replicas based on carbon intensity
type CarbonIntensityConfig struct {
	// carbon intensity threshold to scale the number of replicas
	// +kubebuilder:validation:Required
	CarbonIntensityThreshold int32 `json:"carbonIntensityThreshold"`

	// maximum number of replicas to scale to when the carbon intensity threshold meets or exceeds carbonIntensityThreshold
	// +kubebuilder:validation:Required
	MaxReplicas *int32 `json:"maxReplicas"`
}

// CarbonIntensityForecastDataSource represents the carbon intensity forecast data source
type CarbonIntensityForecastDataSource struct {
	// local configmap details
	// +kubebuilder:validation:Optional
	LocalConfigMap LocalConfigMap `json:"localConfigMap,omitempty"`
	// mock carbon forecast data
	// +kubebuilder:validation:Optional
	MockCarbonForecast bool `json:"mockCarbonForecast,omitempty"`
}

type LocalConfigMap struct {
	// name of the configmap
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// namespace of the configmap
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// key of the carbon intensity forecast data in the configmap
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// KedaTarget represents the type of the KEDA target
// Only one of the following KEDA targets is supported:
// - scaledobjects.keda.sh
// - scaledjobs.keda.sh
// +kubebuilder:validation:Enum=scaledobjects.keda.sh;scaledjobs.keda.sh
type KedaTarget string

const (
	ScaledObject KedaTarget = "scaledobjects.keda.sh"
	ScaledJob    KedaTarget = "scaledjobs.keda.sh"
)

// CarbonAwareKedaScalerSpec defines the desired state of CarbonAwareKedaScaler
type CarbonAwareKedaScalerSpec struct {
	// type of the keda object to scale
	// +kubebuilder:validation:Required
	KedaTarget KedaTarget `json:"kedaTarget"`

	// namespace of the keda target
	// +kubebuilder:validation:Required
	KedaTargetRef KedaTargetRef `json:"kedaTargetRef"`

	// array of carbon intensity values preferrably in ascending order; each threshold value represents the upper limit and previous entry represents lower limit
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	MaxReplicasByCarbonIntensity []CarbonIntensityConfig `json:"maxReplicasByCarbonIntensity"`

	// configuration to disable carbon aware scaler
	// +kubebuilder:validation:Required
	EcoModeOff EcoModeOff `json:"ecoModeOff"`

	// carbon intensity forecast data source
	// must have at least localConfigMap or mockCarbonForecast set
	// +kubebuilder:validation:Required
	CarbonIntensityForecastDataSource CarbonIntensityForecastDataSource `json:"carbonIntensityForecastDataSource"`
}

// CarbonAwareKedaScalerStatus defines the observed state of CarbonAwareKedaScaler
type CarbonAwareKedaScalerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Conditions is a list of conditions and their status.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CarbonAwareKedaScaler is the Schema for the carbonawarekedascalers API
type CarbonAwareKedaScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CarbonAwareKedaScalerSpec   `json:"spec,omitempty"`
	Status CarbonAwareKedaScalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CarbonAwareKedaScalerList contains a list of CarbonAwareKedaScaler
type CarbonAwareKedaScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CarbonAwareKedaScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CarbonAwareKedaScaler{}, &CarbonAwareKedaScalerList{})
}
