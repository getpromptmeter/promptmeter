package datagen

import "time"

// HourlyWeight returns a multiplier for the number of events in a given hour (UTC).
// Models a business-hours traffic pattern.
func HourlyWeight(hour int) float64 {
	switch {
	case hour >= 0 && hour <= 6:
		return 0.2
	case hour >= 7 && hour <= 8:
		return 0.6
	case hour >= 9 && hour <= 12:
		return 1.0
	case hour >= 13 && hour <= 14:
		return 0.8
	case hour >= 15 && hour <= 17:
		return 1.0
	case hour >= 18 && hour <= 20:
		return 0.5
	default: // 21-23
		return 0.3
	}
}

// DayWeight returns a multiplier for weekend/weekday traffic patterns.
func DayWeight(weekday time.Weekday) float64 {
	if weekday == time.Saturday || weekday == time.Sunday {
		return 0.3
	}
	return 1.0
}

// totalHourlyWeightSum returns the sum of all hourly weights for a 24-hour day.
// Used to distribute events-per-day across hours proportionally.
func totalHourlyWeightSum() float64 {
	var sum float64
	for h := 0; h < 24; h++ {
		sum += HourlyWeight(h)
	}
	return sum
}
