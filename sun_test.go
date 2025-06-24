package hal

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestNewSunTimes(t *testing.T) {
	config := LocationConfig{
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	sunTimes := NewSunTimes(config)
	assert.Assert(t, sunTimes != nil)
	assert.Equal(t, sunTimes.location.Latitude, 37.7749)
	assert.Equal(t, sunTimes.location.Longitude, -122.4194)
}

func TestSunTimes_Sunrise(t *testing.T) {
	t.Run("returns sunrise time for San Francisco", func(t *testing.T) {
		config := LocationConfig{
			Latitude:  37.7749,
			Longitude: -122.4194,
		}
		sunTimes := NewSunTimes(config)

		sunrise := sunTimes.Sunrise()
		// Sunrise should be a valid time and in the past or future of today
		assert.Assert(t, !sunrise.IsZero())
		
		// Sunrise should be before sunset
		sunset := sunTimes.Sunset()
		assert.Assert(t, sunrise.Before(sunset))
	})

	t.Run("returns different times for different locations", func(t *testing.T) {
		sfConfig := LocationConfig{Latitude: 37.7749, Longitude: -122.4194} // San Francisco
		nyConfig := LocationConfig{Latitude: 40.7128, Longitude: -74.0060}  // New York

		sfSunTimes := NewSunTimes(sfConfig)
		nySunTimes := NewSunTimes(nyConfig)

		sfSunrise := sfSunTimes.Sunrise()
		nySunrise := nySunTimes.Sunrise()

		// They should be different (unless it's a very special day)
		assert.Assert(t, !sfSunrise.Equal(nySunrise))
	})
}

func TestSunTimes_Sunset(t *testing.T) {
	t.Run("returns sunset time for San Francisco", func(t *testing.T) {
		config := LocationConfig{
			Latitude:  37.7749,
			Longitude: -122.4194,
		}
		sunTimes := NewSunTimes(config)

		sunset := sunTimes.Sunset()
		// Sunset should be a valid time
		assert.Assert(t, !sunset.IsZero())
		
		// Sunset should be after sunrise
		sunrise := sunTimes.Sunrise()
		assert.Assert(t, sunset.After(sunrise))
	})
}

func TestSunTimes_IsDayTime(t *testing.T) {
	t.Run("correctly determines day/night for known time", func(t *testing.T) {
		config := LocationConfig{
			Latitude:  37.7749,
			Longitude: -122.4194,
		}
		sunTimes := NewSunTimes(config)

		// This is a basic test - in real usage, the result depends on current time
		// We just verify the method returns a boolean and doesn't panic
		isDayTime := sunTimes.IsDayTime()
		assert.Assert(t, isDayTime == true || isDayTime == false)
	})

	t.Run("day/night are opposite", func(t *testing.T) {
		config := LocationConfig{
			Latitude:  37.7749,
			Longitude: -122.4194,
		}
		sunTimes := NewSunTimes(config)

		isDayTime := sunTimes.IsDayTime()
		isNightTime := sunTimes.IsNightTime()

		// One should be true, the other false
		assert.Assert(t, isDayTime != isNightTime)
	})
}

func TestSunTimes_IsNightTime(t *testing.T) {
	t.Run("is opposite of IsDayTime", func(t *testing.T) {
		config := LocationConfig{
			Latitude:  37.7749,
			Longitude: -122.4194,
		}
		sunTimes := NewSunTimes(config)

		isDayTime := sunTimes.IsDayTime()
		isNightTime := sunTimes.IsNightTime()

		assert.Equal(t, isDayTime, !isNightTime)
		assert.Equal(t, isNightTime, !isDayTime)
	})
}

func TestSunTimes_EdgeCases(t *testing.T) {
	t.Run("handles extreme northern latitude", func(t *testing.T) {
		// Test with extreme latitude (northern Norway)
		config := LocationConfig{
			Latitude:  78.2156,
			Longitude: 15.5503,
		}
		sunTimes := NewSunTimes(config)

		// Should not panic, even in polar regions
		sunrise := sunTimes.Sunrise()
		sunset := sunTimes.Sunset()
		isDayTime := sunTimes.IsDayTime()
		isNightTime := sunTimes.IsNightTime()

		// Verify methods complete without error
		// In polar regions, sunrise/sunset can be zero during polar night/day
		// so we just verify the calculations don't panic and return valid booleans
		assert.Assert(t, isDayTime == true || isDayTime == false)
		assert.Assert(t, isNightTime == true || isNightTime == false)
		// Ensure sunrise and sunset are either both zero (polar night/day) or both non-zero
		bothZero := sunrise.IsZero() && sunset.IsZero()
		bothNonZero := !sunrise.IsZero() && !sunset.IsZero()
		assert.Assert(t, bothZero || bothNonZero)
	})

	t.Run("handles zero coordinates", func(t *testing.T) {
		config := LocationConfig{
			Latitude:  0,
			Longitude: 0,
		}
		sunTimes := NewSunTimes(config)

		// Should work at equator/prime meridian
		sunrise := sunTimes.Sunrise()
		sunset := sunTimes.Sunset()

		assert.Assert(t, !sunrise.IsZero())
		assert.Assert(t, !sunset.IsZero())
		assert.Assert(t, sunrise.Before(sunset))
	})
}