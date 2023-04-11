package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CarbonForecast struct {
	Location  string    `json:"location,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Duration  int32     `json:"duration,omitempty"`
	Value     float64   `json:"value,omitempty"`
}

func findCarbonForecast(cfs []CarbonForecast, t time.Time) *CarbonForecast {
	for _, cf := range cfs {
		if (t.Equal(cf.Timestamp) || t.After(cf.Timestamp)) && t.Before(cf.Timestamp.Add(time.Duration(cf.Duration)*time.Minute)) {
			return &cf
		}
	}
	return nil
}

type CarbonForecastFetcher interface {
	Fetch(ctx context.Context) ([]CarbonForecast, error)
}

// CarbonForecastConfigMapFetcher is an implementation of CarbonForecastFetcher that fetches the carbon forecast from a configmap
type CarbonForecastConfigMapFetcher struct {
	Client             client.Client
	ConfigMapName      string
	ConfigMapNamespace string
	ConfigMapKey       string
}

func (c *CarbonForecastConfigMapFetcher) Fetch(ctx context.Context) ([]CarbonForecast, error) {
	// load the carbonForecastConfigMap
	cm := &corev1.ConfigMap{}
	err := c.Client.Get(ctx, types.NamespacedName{Name: c.ConfigMapName, Namespace: c.ConfigMapNamespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		return nil, err
	}

	// unmarshal the configmap data into a map
	var cf []CarbonForecast
	err = json.Unmarshal([]byte(cm.BinaryData[c.ConfigMapKey]), &cf)
	if err != nil {
		fmt.Println("got carbon forecast err yo")
		return nil, err
	}

	return cf, nil
}

// CarbonForecastMockConfigMapFetcher is an implementation of CarbonForecastFetcher that creates and fetches a mock configmap
type CarbonForecastMockConfigMapFetcher struct {
	Client         client.Client
	CarbonForecast []CarbonForecast
}

func (c *CarbonForecastMockConfigMapFetcher) Fetch(ctx context.Context) ([]CarbonForecast, error) {
	// if the carbon forecast is already set, return it
	if len(c.CarbonForecast) > 0 {
		return c.CarbonForecast, nil
	}

	// create a new dynamically sized array of CarbonForecast
	c.CarbonForecast = make([]CarbonForecast, 0)

	// for 3 hours ago and 7 days in the future loop at each 5 min increment and add a carbon intensity value
	for i := -3; i < 7*24*12; i++ {
		// generate a random number between 529 and 580
		rand.Seed(time.Now().UnixNano())
		c.CarbonForecast = append(c.CarbonForecast, CarbonForecast{
			Timestamp: time.Now().UTC().Add(time.Duration(i*5) * time.Minute),
			Value:     rand.Float64()*51 + 529,
			Duration:  5,
		})
	}

	// marshal the carbon forecast into byte array
	forecast, err := json.Marshal(c.CarbonForecast)
	if err != nil {
		return nil, err
	}

	// create a configmap and pass carbon forecast as binary data
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "carbon-intensity",
			Namespace: "kube-system",
		},
		BinaryData: map[string][]byte{
			"data": forecast,
		},
	}

	// create or update the configmap
	if err = c.Client.Create(ctx, cm); err != nil {
		if err = c.Client.Update(ctx, cm); err != nil {
			return nil, err
		}
	}

	return c.CarbonForecast, nil
}
