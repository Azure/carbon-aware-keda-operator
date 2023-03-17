package controllers

import "time"

// controller should requeue at the next 5 minute interval
func getRequeueDuration(now time.Time, interval int32) time.Duration {
	// get the difference betwen the current minute and the next minute interval
	// e.g., if the interval is 5 minutes and the current minute is 37, the difference is 3 minutes
	diff := interval - (int32(now.Minute()) % interval)
	// add the difference to the current time to get the next minute interval
	// e.g., if the interval is 5 minutes and the current time is 12:37, the next minute interval is 12:40
	nextMin := now.Add(time.Duration(diff) * time.Minute)
	// get the difference between the next 5 minute interval and the current time
	// e.g., if the interval is 5 minutes and the current time is 12:37, the difference is 3 minutes
	return nextMin.Sub(now)
}
