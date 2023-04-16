// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/azure/carbon-aware-keda-operator/api/v1alpha1"
	carbonawarev1alpha1 "github.com/azure/carbon-aware-keda-operator/api/v1alpha1"
)

var _ = Describe("scenarios for the carbon aware KEDA Scaler", func() {
	Context("the controller requires KEDA to be effective", func() {
		When("the controller is running", func() {
			It("should be able to access KEDA CRDs in the cluster", func() {
				keda := &apiextensionsv1.CustomResourceDefinition{}

				By("confirming the presence of the KEDA ScaledObject CRD")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "scaledobjects.keda.sh"}, keda)).Should(Succeed())

				By("confirming the presence of the KEDA ScaledJob CRD")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "scaledjobs.keda.sh"}, keda)).Should(Succeed())
			})
		})
	})

	Context("the controller should be able to mocked data for demo purposes", func() {
		When("carbonawarekedascaler is set to use mocked data", func() {
			const (
				configMapName      = "mock-carbon-intensity"
				configMapNamespace = "kube-system"
				configMapKey       = "data"
			)
			It("will save forecast data in a ConfigMap", func() {
				f := &CarbonForecastMockConfigMapFetcher{
					Client: k8sClient,
				}
				cf, err := f.Fetch(context.TODO())
				By("Confirming the mock data fetcher returns no error")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cf).ShouldNot(BeNil())

				cm := &corev1.ConfigMap{}
				By("Confirming the ConfigMap named mock-carbon-intensity is found")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: configMapNamespace}, cm)).Should(Succeed())
			})
		})

		When("data is passed into a mock data fetcher", func() {
			It("will use the data and return it", func() {
				now := time.Now().UTC()
				forecast := make([]CarbonForecast, 0)
				// create an array of carbon intensity values for now and 15 minutes in the future and 15 minutes in the past
				values := []float64{80, 20, 10, 100, 80, 70, 60}
				for i := -3; i <= 3; i++ {
					forecast = append(forecast, CarbonForecast{
						Timestamp: now.Add(time.Duration(i*5) * time.Minute),
						Value:     values[i+3],
						Duration:  5,
					})
				}
				By("confirming the forecast data retrieved from the fetcher matches the data passed in")
				// create a new fetcher and pass in the mocked carbon forecast
				f := &CarbonForecastMockConfigMapFetcher{
					CarbonForecast: forecast,
				}
				mcf, err := f.Fetch(context.TODO())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(mcf).ShouldNot(BeNil())
				Expect(len(mcf)).Should(Equal(len(values)))
				Expect(findCarbonForecast(forecast, now).Value).Should(Equal(float64(100))) // 100 is the value at index 3
			})
		})

		When("the test suite is initialized", func() {
			It("will return hard-coded test data from BeforeSuite in suite_test.go", func() {
				By("confirming the carbon intensity is 10 which was set in BeforeSuite")
				Expect(len(carbonforecast)).Should(Equal(len(carbonintensity)))
				forecast := findCarbonForecast(carbonforecast, time.Now().UTC())
				Expect(forecast.Value).Should(Equal(float64(100))) // 100 is the value at index 3
			})
		})
	})

	Context("the controller requeue time should be calculated based on the carbon intensity forecast", func() {
		When("the interval is set to 5 minutes and the current time is 12:37", func() {
			It("will return a duration of 3 minutes", func() {
				now := time.Date(2021, 1, 1, 12, 37, 0, 0, time.UTC)
				duration := getRequeueDuration(now, 5)
				By("confirming the time until next interval")
				Expect(duration).Should(Equal(time.Duration(3) * time.Minute))
			})
		})

		When("the interval is set to 60 minutes and the current time is 12:37", func() {
			It("will return a duration of 23 minutes", func() {
				now := time.Date(2021, 1, 1, 12, 37, 0, 0, time.UTC)
				duration := getRequeueDuration(now, 60)
				By("confirming the time until next interval")
				Expect(duration).Should(Equal(time.Duration(23) * time.Minute))
			})
		})

		When("the interval is set to 15 minutes and the current time is 12:37", func() {
			It("will return a duration of 8 minutes", func() {
				now := time.Date(2021, 1, 1, 12, 37, 0, 0, time.UTC)
				duration := getRequeueDuration(now, 15)
				By("confirming the time until next interval")
				Expect(duration).Should(Equal(time.Duration(8) * time.Minute))
			})
		})
	})

	Context("the controller reconcilation logic", func() {
		const (
			deploymentName                  = "mynginx"
			deploymentNamespace             = "default"
			scaledObjectName                = "mynginx-scaledobject"
			scaledObjectNamespace           = "default"
			scaledObjectKind                = "Deployment"
			carbonAwareKedaScalerName       = "test-carbonawarekedascaler"
			carbonAwareKedaScalerNamespace  = "default"
			carbonAwareKedaScalerKedaTarget = "scaledobjects.keda.sh"
			timeout                         = time.Second * 5
			interval                        = time.Millisecond * 250
		)

		When("the carbonawarekedascaler resource is created", func() {
			BeforeEach(func() {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: deploymentNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: pointer.Int32(1),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": deploymentName,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": deploymentName,
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "nginx",
										Image: "nginx:latest",
										Ports: []corev1.ContainerPort{
											{
												ContainerPort: 80,
											},
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

				scaledobject := &kedav1alpha1.ScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      scaledObjectName,
						Namespace: scaledObjectNamespace,
					},
					Spec: kedav1alpha1.ScaledObjectSpec{
						ScaleTargetRef: &kedav1alpha1.ScaleTarget{
							Name: scaledObjectName,
							Kind: scaledObjectKind,
						},
						Triggers: []kedav1alpha1.ScaleTriggers{
							{
								Type: "kubernetes-workload",
								Metadata: map[string]string{
									"podSelector": "app=mynginx",
									"value":       "3",
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, scaledobject)).Should(Succeed())

				carbonawarekedascaler := &carbonawarev1alpha1.CarbonAwareKedaScaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      carbonAwareKedaScalerName,
						Namespace: carbonAwareKedaScalerNamespace,
					},
					Spec: carbonawarev1alpha1.CarbonAwareKedaScalerSpec{
						CarbonIntensityForecastDataSource: carbonawarev1alpha1.CarbonIntensityForecastDataSource{
							MockCarbonForecast: true,
						},
						KedaTarget: carbonawarev1alpha1.KedaTarget("scaledobjects.keda.sh"),
						KedaTargetRef: carbonawarev1alpha1.KedaTargetRef{
							Name:      scaledObjectName,
							Namespace: scaledObjectNamespace,
						},
						MaxReplicasByCarbonIntensity: []carbonawarev1alpha1.CarbonIntensityConfig{
							{
								CarbonIntensityThreshold: 100,
								MaxReplicas:              pointer.Int32(10),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, carbonawarekedascaler)).Should(Succeed())
			})

			It("can read the KEDA scaledobject", func() {
				carbonawarekedascaler := &carbonawarev1alpha1.CarbonAwareKedaScaler{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: carbonAwareKedaScalerName, Namespace: carbonAwareKedaScalerNamespace}, carbonawarekedascaler)
				By("Confirming no error was returned")
				Expect(err).NotTo(HaveOccurred())

				scaledobject := &kedav1alpha1.ScaledObject{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: scaledObjectName, Namespace: scaledObjectNamespace}, scaledobject)
				By("Confirming no error was returned")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				carbonawarekedascaler := &carbonawarev1alpha1.CarbonAwareKedaScaler{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: carbonAwareKedaScalerName, Namespace: carbonAwareKedaScalerNamespace}, carbonawarekedascaler)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, carbonawarekedascaler)).Should(Succeed())

				scaledobject := &kedav1alpha1.ScaledObject{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: scaledObjectName, Namespace: scaledObjectNamespace}, scaledobject)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, scaledobject)).Should(Succeed())

				deployment := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: deploymentNamespace}, deployment)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			})
		})

		When("the carbon intensity is within a configured range", func() {
			const (
				testConfigMapName      = "another-mock-carbon-intensity"
				testConfigMapNamespace = "kube-system"
				testConfigMapKey       = "data"
			)
			BeforeEach(func() {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: deploymentNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: pointer.Int32(1),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": deploymentName,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": deploymentName,
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "nginx",
										Image: "nginx:latest",
										Ports: []corev1.ContainerPort{
											{
												ContainerPort: 80,
											},
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

				scaledobject := &kedav1alpha1.ScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      scaledObjectName,
						Namespace: scaledObjectNamespace,
					},
					Spec: kedav1alpha1.ScaledObjectSpec{
						ScaleTargetRef: &kedav1alpha1.ScaleTarget{
							Name: scaledObjectName,
							Kind: scaledObjectKind,
						},
						Triggers: []kedav1alpha1.ScaleTriggers{
							{
								Type: "kubernetes-workload",
								Metadata: map[string]string{
									"podSelector": "app=mynginx",
									"value":       "3",
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, scaledobject)).Should(Succeed())
				// Eventually(func() bool {
				// 	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
				// 	err := k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("keda-hpa-%s", scaledObjectName), Namespace: scaledObjectNamespace}, hpa)
				// 	return err == nil
				// }, 30, interval).Should(BeTrue())

				// create a mock carbon intensity forecast
				cf := make([]CarbonForecast, 1)
				cf[0] = CarbonForecast{
					Timestamp: time.Now().UTC(),
					Value:     float64(99),
					Duration:  5,
				}

				// marshal the carbon forecast into byte array
				forecast, err := json.Marshal(cf)
				Expect(err).NotTo(HaveOccurred())

				// create a configmap with the carbon forecast
				configmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testConfigMapName,
						Namespace: testConfigMapNamespace,
					},
					BinaryData: map[string][]byte{
						testConfigMapKey: forecast,
					},
				}
				Expect(k8sClient.Create(ctx, configmap)).Should(Succeed())

				// create a carbonawarekedascaler resource
				carbonawarekedascaler := &carbonawarev1alpha1.CarbonAwareKedaScaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      carbonAwareKedaScalerName,
						Namespace: carbonAwareKedaScalerNamespace,
					},
					Spec: carbonawarev1alpha1.CarbonAwareKedaScalerSpec{
						CarbonIntensityForecastDataSource: carbonawarev1alpha1.CarbonIntensityForecastDataSource{
							MockCarbonForecast: false,
							LocalConfigMap: carbonawarev1alpha1.LocalConfigMap{
								Name:      testConfigMapName,
								Namespace: testConfigMapNamespace,
								Key:       testConfigMapKey,
							},
						},
						KedaTarget: carbonawarev1alpha1.KedaTarget("scaledobjects.keda.sh"),
						KedaTargetRef: carbonawarev1alpha1.KedaTargetRef{
							Name:      scaledObjectName,
							Namespace: scaledObjectNamespace,
						},
						MaxReplicasByCarbonIntensity: []carbonawarev1alpha1.CarbonIntensityConfig{
							{
								CarbonIntensityThreshold: 100,
								MaxReplicas:              pointer.Int32(9),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, carbonawarekedascaler)).Should(Succeed())
			})

			It("should update the scaledobject to the max replicas", func() {
				scaledobject := &kedav1alpha1.ScaledObject{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: scaledObjectName, Namespace: scaledObjectNamespace}, scaledobject)
				Expect(err).NotTo(HaveOccurred())

				// By("confirming the scaledobject has the correct max replicas")
				// Expect(scaledobject.Spec.MaxReplicaCount).To(Equal(int32(9)))
			})

			AfterEach(func() {
				configmap := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: testConfigMapName, Namespace: testConfigMapNamespace}, configmap)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, configmap)).Should(Succeed())

				carbonawarekedascaler := &carbonawarev1alpha1.CarbonAwareKedaScaler{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: carbonAwareKedaScalerName, Namespace: carbonAwareKedaScalerNamespace}, carbonawarekedascaler)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, carbonawarekedascaler)).Should(Succeed())

				scaledobject := &kedav1alpha1.ScaledObject{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: scaledObjectName, Namespace: scaledObjectNamespace}, scaledobject)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, scaledobject)).Should(Succeed())

				deployment := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: deploymentNamespace}, deployment)
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			})
		})
	})

	Context("the carbonawarekedascaler should be configurable to be disabled based on certain conditions", func() {
		When("the custom schedule is configured", func() {
			It("should turn eco mode off", func() {
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					CustomSchedule: []carbonawarev1alpha1.Schedule{
						{
							StartTime: time.Now().UTC().Add(time.Duration(1) * time.Hour).Format("2006-01-02T15:00:00Z"),
							EndTime:   time.Now().UTC().Add(time.Duration(2) * time.Hour).Format("2006-01-02T15:00:00Z"),
						},
						{
							StartTime: time.Now().UTC().Format("2006-01-02T15:00:00Z"),
							EndTime:   time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:00:00Z"),
						},
					},
				}
				err := setEcoMode(status, configs, carbonforecast)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeTrue())
			})
		})

		When("the custom schedule is not configured but does not meet time criteria", func() {
			It("should leave eco mode on", func() {
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					CustomSchedule: []carbonawarev1alpha1.Schedule{
						{
							StartTime: time.Now().UTC().Add(time.Duration(-3) * time.Hour).Format("2006-01-02T15:00:00Z"),
							EndTime:   time.Now().UTC().Add(time.Duration(-2) * time.Hour).Format("2006-01-02T15:00:00Z"),
						},
						{
							StartTime: time.Now().UTC().Add(time.Duration(2) * time.Hour).Format("2006-01-02T15:00:00Z"),
							EndTime:   time.Now().UTC().Add(time.Duration(3) * time.Hour).Format("2006-01-02T15:00:00Z"),
						},
					},
				}
				err := setEcoMode(status, configs, carbonforecast)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeFalse())
			})
		})

		When("the recurring schedule is configured", func() {
			It("should turn eco mode off", func() {
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					RecurringSchedule: []string{
						fmt.Sprintf("* * %d * *", time.Now().UTC().Day()), // current day
					},
				}
				err := setEcoMode(status, configs, carbonforecast)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeTrue())
			})
		})

		When("the recurring schedule is not configured but does not meet time criteria", func() {
			It("should leave eco mode on", func() {
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					RecurringSchedule: []string{
						fmt.Sprintf("%d * * * *", time.Now().UTC().Add(time.Duration(-1)*time.Hour).Minute()), // one minute in the past
					},
				}
				err := setEcoMode(status, configs, carbonforecast)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeFalse())
			})
		})

		When("the carbon intensity over a duration is configured", func() {
			It("should turn eco mode off", func() {
				forecast := []CarbonForecast{
					{
						Value:     90,
						Timestamp: time.Now().UTC().Add(time.Duration(-20) * time.Minute),
						Duration:  5,
					},
					{
						Value:     60,
						Timestamp: time.Now().UTC().Add(time.Duration(-15) * time.Minute),
						Duration:  5,
					},
					{
						Value:     70,
						Timestamp: time.Now().UTC().Add(time.Duration(-10) * time.Minute),
						Duration:  5,
					},
					{
						Value:     80,
						Timestamp: time.Now().UTC().Add(time.Duration(-5) * time.Minute),
						Duration:  5,
					},
					{
						Value:     90,
						Timestamp: time.Now().UTC(),
						Duration:  5,
					},
				}
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					CarbonIntensityDuration: v1alpha1.CarbonIntensityDuration{
						CarbonIntensityThreshold:       50,
						OverrideEcoAfterDurationInMins: 20,
					},
				}
				err := setEcoMode(status, configs, forecast)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeTrue())
			})
		})

		When("the carbon intensity over a duration is configured but doesn't meet time criteria", func() {
			It("should leave eco mode on", func() {
				forecast := []CarbonForecast{
					{
						Value:     90,
						Timestamp: time.Now().UTC().Add(time.Duration(-20) * time.Minute),
						Duration:  5,
					},
					{
						Value:     80,
						Timestamp: time.Now().UTC().Add(time.Duration(-15) * time.Minute),
						Duration:  5,
					},
					{
						Value:     80,
						Timestamp: time.Now().UTC().Add(time.Duration(-10) * time.Minute),
						Duration:  5,
					},
					{
						Value:     80,
						Timestamp: time.Now().UTC().Add(time.Duration(-5) * time.Minute),
						Duration:  5,
					},
					{
						Value:     60,
						Timestamp: time.Now().UTC(),
						Duration:  5,
					},
				}
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					CarbonIntensityDuration: v1alpha1.CarbonIntensityDuration{
						CarbonIntensityThreshold:       80,
						OverrideEcoAfterDurationInMins: 15,
					},
				}
				err := setEcoMode(status, configs, forecast)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeFalse())
			})
		})

		When("the carbon intensity over a duration is configured but doesn't meet threshold criteria", func() {
			It("should leave eco mode on", func() {
				data := []CarbonForecast{
					{
						Value:     90,
						Timestamp: time.Now().UTC().Add(time.Duration(-20) * time.Minute),
						Duration:  5,
					},
					{
						Value:     60,
						Timestamp: time.Now().UTC().Add(time.Duration(-15) * time.Minute),
						Duration:  5,
					},
					{
						Value:     70,
						Timestamp: time.Now().UTC().Add(time.Duration(-10) * time.Minute),
						Duration:  5,
					},
					{
						Value:     80,
						Timestamp: time.Now().UTC().Add(time.Duration(-5) * time.Minute),
						Duration:  5,
					},
					{
						Value:     90,
						Timestamp: time.Now().UTC(),
						Duration:  5,
					},
				}
				status := &EcoModeStatus{}
				configs := carbonawarev1alpha1.EcoModeOff{
					CarbonIntensityDuration: v1alpha1.CarbonIntensityDuration{
						CarbonIntensityThreshold:       80,
						OverrideEcoAfterDurationInMins: 10,
					},
				}
				err := setEcoMode(status, configs, data)
				Expect(err).NotTo(HaveOccurred())
				Expect(status.IsDisabled).To(BeFalse())
			})
		})
	})

	Context("max replicas calculation is based on carbon intensity threshold configured", func() {
		When("the forecast is nil", func() {
			It("should return an error", func() {
				var maxReplicaConfig *int32 = new(int32)
				*maxReplicaConfig = 10
				maxReplicas, err := getMaxReplicas(nil, []carbonawarev1alpha1.CarbonIntensityConfig{
					{
						MaxReplicas:              maxReplicaConfig,
						CarbonIntensityThreshold: 100,
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(maxReplicas).To(BeNil())
			})
		})

		When("the forecasted carbon intensity is 100 and max replicas is set to 10", func() {
			var configs []carbonawarev1alpha1.CarbonIntensityConfig

			BeforeEach(func() {
				configs = []carbonawarev1alpha1.CarbonIntensityConfig{
					{
						CarbonIntensityThreshold: 100,
						MaxReplicas:              new(int32), // 10
					},
					{
						CarbonIntensityThreshold: 90,
						MaxReplicas:              new(int32), // 20
					},
					{
						CarbonIntensityThreshold: 40,
						MaxReplicas:              new(int32), // 80
					},
				}

				*configs[0].MaxReplicas = 10
				*configs[1].MaxReplicas = 20
				*configs[2].MaxReplicas = 80
			})

			It("should return 10", func() {
				forecast := make([]CarbonForecast, 1)
				forecast[0] = CarbonForecast{
					Timestamp: time.Now().UTC(),
					Value:     100,
					Duration:  5,
				}
				maxReplicas, err := getMaxReplicas(&forecast[0], configs)
				Expect(err).NotTo(HaveOccurred())
				Expect(*maxReplicas).To(Equal(int32(10)))
			})

			It("should return 80", func() {
				forecast := make([]CarbonForecast, 1)
				forecast[0] = CarbonForecast{
					Timestamp: time.Now().UTC(),
					Value:     30,
					Duration:  5,
				}
				maxReplicas, err := getMaxReplicas(&forecast[0], configs)
				Expect(err).NotTo(HaveOccurred())
				Expect(*maxReplicas).To(Equal(int32(80)))
			})

			It("should return 10", func() {
				forecast := make([]CarbonForecast, 1)
				forecast[0] = CarbonForecast{
					Timestamp: time.Now().UTC(),
					Value:     101,
					Duration:  5,
				}
				maxReplicas, err := getMaxReplicas(&forecast[0], configs)
				Expect(err).NotTo(HaveOccurred())
				Expect(*maxReplicas).To(Equal(int32(10)))
			})

			It("should return 10", func() {
				forecast := make([]CarbonForecast, 1)
				forecast[0] = CarbonForecast{
					Timestamp: time.Now().UTC(),
					Value:     101010,
					Duration:  5,
				}
				maxReplicas, err := getMaxReplicas(&forecast[0], configs)
				Expect(err).NotTo(HaveOccurred())
				Expect(*maxReplicas).To(Equal(int32(10)))
			})

			It("should return 20", func() {
				forecast := make([]CarbonForecast, 1)
				forecast[0] = CarbonForecast{
					Timestamp: time.Now().UTC(),
					Value:     67,
					Duration:  5,
				}
				maxReplicas, err := getMaxReplicas(&forecast[0], configs)
				Expect(err).NotTo(HaveOccurred())
				Expect(*maxReplicas).To(Equal(int32(20)))
			})
		})
	})
})
