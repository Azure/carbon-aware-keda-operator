package controllers

import (
	"fmt"
	"time"

	"github.com/aptible/supercronic/cronexpr"

	carbonawarev1alpha1 "github.com/azure/carbon-aware-keda-operator/api/v1alpha1"
)

/*
EcoMode is enabled by default. There are a few instances where it should be disabled:
1. If the carbonawarekedascaler is scheduled to be disabled based on a custom schedule
2. If the carbonawarekedascaler is scheduled to be disabled based on a recurring schedule
3. If the carbonawarekedascaler is scheduled to be disabled based on a carbon intensity threshold
4. If the maximum number of replicas is less than what the horizontal pod autoscaler desires (though this is handled in the reconcile loop in carbonawarekedascaler_controller.go)
*/

type EcoModeStatus struct {
	IsDisabled    bool
	DisableReason string
	RequeueAfter  time.Duration
}

// goal of this function is to determine if the carbonawarekedascaler should be disabled based on the configuration
func setEcoMode(ecoModeStatus *EcoModeStatus, configs carbonawarev1alpha1.EcoModeOff, forecast []CarbonForecast) error {
	// check if the carbonawarekedascaler should be disabled based on a custom schedule
	if len(configs.CustomSchedule) > 0 {
		now := time.Now().UTC()
		// each entry in the custom schedule has a start and end time
		for _, entry := range configs.CustomSchedule {
			// parse the start time
			start, err := time.Parse(time.RFC3339, entry.StartTime)
			if err != nil {
				return err
			}
			// parse the end time
			end, err := time.Parse(time.RFC3339, entry.EndTime)
			if err != nil {
				return err
			}

			// if the current time is between the start and end time, then disable the carbonawarekedascaler
			if now.After(start) && now.Before(end) {
				// find the number of minutes until the end time and requeue the carbonawarekedascaler after that time
				duration := time.Until(end.Add(time.Microsecond * 1))
				ecoModeStatus.IsDisabled = true
				ecoModeStatus.DisableReason = fmt.Sprintf("custom schedule from %s to %s", start, end)
				ecoModeStatus.RequeueAfter = duration
				return nil
			}
		}
	}

	// check if the carbonawarekedascaler should be disabled based on a recurring schedule
	if len(configs.RecurringSchedule) > 0 {
		now := time.Now().UTC()
		// each entry in the recurring schedule is a cron expression
		for _, entry := range configs.RecurringSchedule {
			// parse the start time which is set using cron syntax
			expr := cronexpr.MustParse(entry)

			// get the next time the cron expression will run
			next := expr.Next(now)

			// if the next time the cron expression is within one minute away, then disable the carbonawarekedascaler
			if next.Sub(now) <= time.Minute {
				// must find the next earliest time outside of the cron expression so that the carbonawarekedascaler can be requeued after that time

				// find the number of minutes until the end of the day
				eod := time.Until(time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC).Add(time.Second * 1))

				// get the next n minutes where n is the number of minutes until the end of the day
				nextn := expr.NextN(now, uint(eod.Minutes()))

				// pull out indices where the date is the same as current date as we only want the last time the cron expression will run today
				for i, n := range nextn {
					if n.Day() != now.Day() {
						nextn = nextn[:i]
						break
					}
				}

				// get the last time the cron expression will run today
				last := nextn[len(nextn)-1]

				// find the number of minutes until the last time the cron expression will run today and requeue the carbonawarekedascaler after that time
				duration := time.Until(last.Add(time.Minute + 1))
				ecoModeStatus.IsDisabled = true
				ecoModeStatus.DisableReason = fmt.Sprintf("recurring schedule \"%s\"", entry)
				ecoModeStatus.RequeueAfter = duration
				return nil
			}
		}
	}

	// check if the carbonawarekedascaler should be disabled based on a carbon intensity threshold over a duration
	if configs.CarbonIntensityDuration.OverrideEcoAfterDurationInMins > 0 {
		carbonIntensityThreshold := configs.CarbonIntensityDuration.CarbonIntensityThreshold
		overrideEcoAfterDuration := time.Duration(configs.CarbonIntensityDuration.OverrideEcoAfterDurationInMins) * time.Minute

		// get the number of minutes in the duration
		durationMins := overrideEcoAfterDuration.Minutes()

		// count the number of times the carbon intensity threshold is met
		meetsThresholdCount := 0

		// for each minute in the duration, check if the carbon intensity is >= carbonIntensityThreshold
		for i := 0; i < int(overrideEcoAfterDuration.Minutes()); i++ {
			// get the current time and go back i minutes
			lookback := time.Now().UTC().Add(time.Duration(-i) * time.Minute)

			// get the forecast for time we are looking back to
			currentForecast := findCarbonForecast(forecast, lookback)

			// if the forecast is nil
			if currentForecast == nil {
				continue
			}

			// if the carbon intensity is >= carbonIntensityThreshold, then increment the count
			if currentForecast.Value >= float64(carbonIntensityThreshold) {
				meetsThresholdCount++
			}
		}

		// if the number of hours that meet the carbon intensity threshold is equal to the number of minutes in the duration, then disable the carbonawarekedascaler
		if meetsThresholdCount == int(durationMins) {
			ecoModeStatus.IsDisabled = true
			ecoModeStatus.DisableReason = fmt.Sprintf("carbon intensity >= threshold of %d for the last %s", carbonIntensityThreshold, overrideEcoAfterDuration)
			return nil
		}
	}

	return nil
}
