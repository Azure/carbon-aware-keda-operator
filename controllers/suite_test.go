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
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	carbonawarev1alpha1 "github.com/azure/carbon-aware-keda-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg             *rest.Config
	k8sClient       client.Client
	testEnv         *envtest.Environment
	ctx             context.Context
	cancel          context.CancelFunc
	k8sManager      manager.Manager
	carbonforecast  []CarbonForecast
	carbonintensity []float64
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases"), filepath.Join("..", "hack", "keda")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = carbonawarev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// add keda to scheme
	err = kedav1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	// create a mock carbon forecast configmap
	carbonforecast = make([]CarbonForecast, 0)
	// create an array of carbon intensity values for now and 3 hours in the future and 3 hours in the past
	carbonintensity = []float64{20, 50, 80, 16, 73, 93, 36, 68, 49, 54, 81, 30, 67, 22, 38, 19, 92, 84, 100, 46, 68, 29, 80, 10, 71, 92, 34, 67, 42, 58, 84, 10, 79, 22, 35, 18, 72}
	for i := -18; i <= 18; i++ {
		carbonforecast = append(carbonforecast, CarbonForecast{
			Timestamp: time.Now().UTC().Add(time.Duration(i*5) * time.Minute),
			Value:     carbonintensity[i+18],
			Duration:  5,
		})
	}

	err = (&CarbonAwareKedaScalerReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("carbon-aware-keda-scaler-controller"),
		CarbonForecastFetcher: &CarbonForecastMockConfigMapFetcher{
			Client:         k8sClient,
			CarbonForecast: carbonforecast,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	// start the k8sManager
	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
