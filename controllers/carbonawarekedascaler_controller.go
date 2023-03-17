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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	carbonawarev1alpha1 "github.com/azure/carbon-aware-keda-operator/api/v1alpha1"
)

// CarbonAwareKedaScalerReconciler reconciles a CarbonAwareKedaScaler object
type CarbonAwareKedaScalerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	CarbonForecastFetcher
}

//+kubebuilder:rbac:groups=carbonaware.kubernetes.azure.com,resources=carbonawarekedascalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=carbonaware.kubernetes.azure.com,resources=carbonawarekedascalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=carbonaware.kubernetes.azure.com,resources=carbonawarekedascalers/finalizers,verbs=update
//+kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=keda.sh,resources=scaledjobs,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CarbonAwareKedaScaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *CarbonAwareKedaScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	kedav1alpha1.AddToScheme(r.Scheme)

	now := time.Now().UTC()

	// default interval the controller should requeue at
	requeueInterval := int32(5)

	setStatusCondition := func(c *carbonawarev1alpha1.CarbonAwareKedaScaler, status metav1.ConditionStatus, reason string, msg string) {
		meta.SetStatusCondition(&c.Status.Conditions, metav1.Condition{
			Type:    "OperatorDegraded",
			Status:  status,
			Reason:  reason,
			Message: msg,
		})
		r.Status().Update(ctx, c)
	}

	ecoModeStatus := &EcoModeStatus{
		RequeueAfter: getRequeueDuration(now, requeueInterval),
	}

	// number of replicas to scale to based on carbon intensity or eco mode off configuration
	var maxReplicaCount *int32

	// get the carbonAwareKedaScaler
	carbonAwareKedaScaler := &carbonawarev1alpha1.CarbonAwareKedaScaler{}
	err := r.Get(ctx, req.NamespacedName, carbonAwareKedaScaler)
	if err != nil && errors.IsNotFound(err) {
		logger.Error(err, "unable to find carbonawarekedascaler")
		r.Recorder.Event(carbonAwareKedaScaler, "Warning", "NoCustomResource", fmt.Sprintf("Unable to find carbonawarekedascaler %s", req.NamespacedName))
		return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, client.IgnoreNotFound(err)
	}

	ReconcilesTotal.WithLabelValues(carbonAwareKedaScaler.Name).Inc()

	// set the carbon forecast fetcher if it is not already set
	if r.CarbonForecastFetcher == nil {
		// if mock carbon forecast is enabled use the mock fetcher otherwise, use the configmap fetcher
		if carbonAwareKedaScaler.Spec.CarbonIntensityForecastDataSource.MockCarbonForecast {
			r.CarbonForecastFetcher = &CarbonForecastMockConfigMapFetcher{
				Client: r.Client,
			}
			r.Recorder.Event(carbonAwareKedaScaler, "Normal", "CarbonForecastSource", "Using mock carbon forecast")
		} else if carbonAwareKedaScaler.Spec.CarbonIntensityForecastDataSource.LocalConfigMap != (carbonawarev1alpha1.LocalConfigMap{}) {
			// fetch the carbon forecast from configmap
			r.CarbonForecastFetcher = &CarbonForecastConfigMapFetcher{
				Client:             r.Client,
				ConfigMapName:      carbonAwareKedaScaler.Spec.CarbonIntensityForecastDataSource.LocalConfigMap.Name,
				ConfigMapNamespace: carbonAwareKedaScaler.Spec.CarbonIntensityForecastDataSource.LocalConfigMap.Namespace,
				ConfigMapKey:       carbonAwareKedaScaler.Spec.CarbonIntensityForecastDataSource.LocalConfigMap.Key,
			}
			r.Recorder.Event(carbonAwareKedaScaler, "Normal", "CarbonForecastSource", fmt.Sprintf("Using carbon forecast from configmap %s", carbonAwareKedaScaler.Spec.CarbonIntensityForecastDataSource.LocalConfigMap.Name))
		}
	}

	// fetch the carbon forecast
	forecast, err := r.CarbonForecastFetcher.Fetch(ctx)
	if err != nil {
		ecoModeStatus.IsDisabled = true
		ecoModeStatus.DisableReason = err.Error()
		ecoModeStatus.RequeueAfter = getRequeueDuration(now, requeueInterval)
		maxReplicaCount = &carbonAwareKedaScaler.Spec.EcoModeOff.MaxReplicas
		logger.Error(err, "failed to fetch carbon forecast")
		setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonCarbonDataFetchError, fmt.Sprintf("failed to fetch carbon forecast: %v", err))
		r.Recorder.Event(carbonAwareKedaScaler, "Warning", "CarbonIntensityForecastMissing", "Failed to fetch carbon forecast")
	}

	// get the current carbon forecast
	currentforecast := findCarbonForecast(forecast, now)
	requeueInterval = currentforecast.Duration
	if err != nil {
		requeueInterval = 5
	}

	// check if it should be disabled based on the eco mode off configuration
	if !ecoModeStatus.IsDisabled {
		err = setEcoMode(ecoModeStatus, carbonAwareKedaScaler.Spec.EcoModeOff, forecast)
		if err != nil {
			ecoModeStatus.IsDisabled = true
			ecoModeStatus.DisableReason = err.Error()
			ecoModeStatus.RequeueAfter = getRequeueDuration(now, requeueInterval)
			maxReplicaCount = &carbonAwareKedaScaler.Spec.EcoModeOff.MaxReplicas
			logger.Error(err, "unable to parse eco mode off configs")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonEcoModeDisabledError, fmt.Sprintf("unable to parse eco mode off configs: %v", err))
			r.Recorder.Event(carbonAwareKedaScaler, "Warning", "EcoModeConfigError", "Failed to parse eco mode off configs")
		}
	}

	// get the max replicas for the current hour based on carbon forecast configuration
	if !ecoModeStatus.IsDisabled {
		maxReplicaCount, err = getMaxReplicas(currentforecast, carbonAwareKedaScaler.Spec.MaxReplicasByCarbonIntensity)
		if err != nil {
			ecoModeStatus.IsDisabled = true
			ecoModeStatus.DisableReason = err.Error()
			ecoModeStatus.RequeueAfter = getRequeueDuration(now, requeueInterval)
			maxReplicaCount = &carbonAwareKedaScaler.Spec.EcoModeOff.MaxReplicas
			logger.Error(err, "unable to find max replica count for carbon forecast")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonMaxReplicasCountError, fmt.Sprintf("unable to find max replica count for carbon forecast: %v", err))
			r.Recorder.Event(carbonAwareKedaScaler, "Warning", "MaxReplicaError", fmt.Sprintf("Unable to find max replica count for carbon forecast for %v", currentforecast))
		}
	}

	// scale the keda target
	switch {
	case strings.Contains(string(carbonAwareKedaScaler.Spec.KedaTarget), "scaledobject"):
		scaledObject := &kedav1alpha1.ScaledObject{}
		err = r.Get(ctx, types.NamespacedName{Name: carbonAwareKedaScaler.Spec.KedaTargetRef.Name, Namespace: carbonAwareKedaScaler.Spec.KedaTargetRef.Namespace}, scaledObject)
		if err != nil && errors.IsNotFound(err) {
			logger.Error(err, "unable to find scaledobject")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetNotFound, fmt.Sprintf("unable to find scaledobject: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, client.IgnoreNotFound(err)
		} else if err != nil {
			ReconcileErrorsTotal.WithLabelValues(carbonAwareKedaScaler.Name).Inc()
			logger.Error(err, "failed to find scaledobject")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetFetchError, fmt.Sprintf("failed to find scaledobject: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, err
		}

		// get the hpa associated with the scaledobject
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		err = r.Get(ctx, types.NamespacedName{Name: scaledObject.Status.HpaName, Namespace: scaledObject.Namespace}, hpa)
		if err != nil && errors.IsNotFound(err) {
			logger.Error(err, "unable to find hpa")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetNotFound, fmt.Sprintf("unable to find hpa: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, client.IgnoreNotFound(err)
		} else if err != nil {
			ReconcileErrorsTotal.WithLabelValues(carbonAwareKedaScaler.Name).Inc()
			logger.Error(err, "failed to get hpa")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetNotFound, fmt.Sprintf("failed to get hpa: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, err
		}

		// eco mode is good but don't let performance suffer! if maxReplicaCount is less than what HPA desires then disable the carbon aware scaler
		if maxReplicaCount != nil && *maxReplicaCount < hpa.Status.DesiredReplicas {
			ecoModeStatus.IsDisabled = true
			ecoModeStatus.DisableReason = fmt.Sprintf("Disabling carbon awareness since maxReplicaCount of %d is less than what HPA desires %d", *maxReplicaCount, hpa.Status.DesiredReplicas)
		}

		// log the current and desired replicas
		HpaCurrentReplicasMetric.WithLabelValues(carbonAwareKedaScaler.Name).Set(float64(hpa.Status.CurrentReplicas))
		HpaDesiredReplicasMetric.WithLabelValues(carbonAwareKedaScaler.Name).Set(float64(hpa.Status.DesiredReplicas))

		// set to max replicas configured when eco mode is disabled
		if ecoModeStatus.IsDisabled {
			EcoModeOffMetric.WithLabelValues(carbonAwareKedaScaler.Name, "1").Inc()
			logger.Info("eco mode disabled", "reason", ecoModeStatus.DisableReason)
			r.Recorder.Event(carbonAwareKedaScaler, "Warning", "EcoModeDisabled", fmt.Sprintf("Eco mode disabled due to %s", ecoModeStatus.DisableReason))
			maxReplicaCount = &carbonAwareKedaScaler.Spec.EcoModeOff.MaxReplicas
		} else {
			EcoModeOffMetric.WithLabelValues(carbonAwareKedaScaler.Name, "0").Inc()
		}

		// TODO: or catch "the object has been modified" and retry??
		// get a fresh scaled object to avoid dirty writes
		err := r.Get(ctx, types.NamespacedName{Name: scaledObject.Name, Namespace: scaledObject.Namespace}, scaledObject)
		if err != nil {
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetFetchError, fmt.Sprintf("failed to get scaledobject: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, client.IgnoreNotFound(err)
		}

		// ovewrite the scaledobject.Spec.MaxReplicaCount with the max replica count for the current carbon rating
		scaledObject.Spec.MaxReplicaCount = maxReplicaCount

		// update the scaled object
		err = r.Update(ctx, scaledObject)
		if err != nil {
			ReconcileErrorsTotal.WithLabelValues(carbonAwareKedaScaler.Name).Inc()
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetUpdateFailed, fmt.Sprintf("failed to update scaledobject: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, err
		} else {
			logger.Info("updated scaledobject", "scaledobject", scaledObject.Name, "forecast", currentforecast, "maxReplicas", &scaledObject.Spec.MaxReplicaCount)
			if ecoModeStatus.IsDisabled {
				setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonEcoModeDisabled, "operator successfully reconciling but eco mode is disabled")
			} else {
				setStatusCondition(carbonAwareKedaScaler, metav1.ConditionFalse, carbonawarev1alpha1.ReasonSucceeded, "operator successfully reconciling and eco mode is enabled")
			}
		}
	case strings.Contains(string(carbonAwareKedaScaler.Spec.KedaTarget), "scaledjob"):
		scaledJob := &kedav1alpha1.ScaledJob{}
		err = r.Get(ctx, types.NamespacedName{Name: carbonAwareKedaScaler.Spec.KedaTargetRef.Name, Namespace: carbonAwareKedaScaler.Spec.KedaTargetRef.Namespace}, scaledJob)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("unable to find scaledjob")
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetNotFound, fmt.Sprintf("unable to get scaledjob: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, client.IgnoreNotFound(err)
		} else if err != nil {
			ReconcileErrorsTotal.WithLabelValues(carbonAwareKedaScaler.Name).Inc()
			logger.Info("failed to get scaledjob", "error", err)
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetFetchError, fmt.Sprintf("failed to get scaledjob: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, err
		}

		// set to max replicas configured when eco mode is disabled
		if ecoModeStatus.IsDisabled {
			EcoModeOffMetric.WithLabelValues(carbonAwareKedaScaler.Name, "1").Inc()
			logger.Info("eco mode disabled", "reason", ecoModeStatus.DisableReason)
			r.Recorder.Event(carbonAwareKedaScaler, "Warning", "EcoModeDisabled", fmt.Sprintf("Eco mode disabled due to %s", ecoModeStatus.DisableReason))
			maxReplicaCount = &carbonAwareKedaScaler.Spec.EcoModeOff.MaxReplicas
		} else {
			EcoModeOffMetric.WithLabelValues(carbonAwareKedaScaler.Name, "0").Inc()
		}

		// get a fresh scaled job to avoid dirty writes
		err := r.Get(ctx, types.NamespacedName{Name: scaledJob.Name, Namespace: scaledJob.Namespace}, scaledJob)
		if err != nil {
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetFetchError, fmt.Sprintf("failed to get scaledjob: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, client.IgnoreNotFound(err)
		}

		// ovewrite the scaledobject.Spec.MaxReplicaCount with the max replica count for the current carbon rating
		scaledJob.Spec.MaxReplicaCount = maxReplicaCount

		// update the scaled job
		err = r.Update(ctx, scaledJob)
		if err != nil {
			ReconcileErrorsTotal.WithLabelValues(carbonAwareKedaScaler.Name).Inc()
			setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonTargetUpdateFailed, fmt.Sprintf("failed to update scaledjob: %v", err))
			return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, err
		} else {
			logger.Info("updated scaledjob", "scaledjob", scaledJob.Name, "forecast", currentforecast, "maxReplicas", &scaledJob.Spec.MaxReplicaCount)
			if ecoModeStatus.IsDisabled {
				setStatusCondition(carbonAwareKedaScaler, metav1.ConditionTrue, carbonawarev1alpha1.ReasonEcoModeDisabled, "operator successfully reconciling but eco mode is disabled")
			} else {
				setStatusCondition(carbonAwareKedaScaler, metav1.ConditionFalse, carbonawarev1alpha1.ReasonSucceeded, "operator successfully reconciling")
			}
		}
	}

	// log the current carbon intensity
	CarbonIntensityMetric.WithLabelValues(carbonAwareKedaScaler.Name).Set(currentforecast.Value)

	// log the default max replicas
	DefaultMaxReplicasMetric.WithLabelValues(carbonAwareKedaScaler.Name).Set(float64(carbonAwareKedaScaler.Spec.EcoModeOff.MaxReplicas))

	// log the current max replicas
	MaxReplicasMetric.WithLabelValues(carbonAwareKedaScaler.Name).Set(float64(*maxReplicaCount))

	// record the successful reconcile event
	r.Recorder.Event(carbonAwareKedaScaler, "Normal", "MaxReplicaCountReconciled", fmt.Sprintf("Successfully set max replicas for %s to %d", carbonAwareKedaScaler.Spec.KedaTargetRef.Name, *maxReplicaCount))

	return ctrl.Result{RequeueAfter: getRequeueDuration(now, requeueInterval)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CarbonAwareKedaScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&carbonawarev1alpha1.CarbonAwareKedaScaler{}).
		Complete(r)
}
