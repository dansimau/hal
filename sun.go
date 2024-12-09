package hal

import (
	"log/slog"
	"time"

	"github.com/nathan-osman/go-sunrise"
)

type SunTimes struct {
	location LocationConfig
}

func NewSunTimes(cfg LocationConfig) *SunTimes {
	sunTimes := &SunTimes{
		location: cfg,
	}

	slog.Info("Sun times for location",
		"lat", cfg.Latitude,
		"lng", cfg.Longitude,
		"sunrise", sunTimes.Sunrise().Local().Format(time.Kitchen),
		"sunset", sunTimes.Sunset().Local().Format(time.Kitchen),
	)

	return sunTimes
}

func (s *SunTimes) IsDayTime() bool {
	now := time.Now()

	rise, set := sunrise.SunriseSunset(s.location.Latitude, s.location.Longitude, now.Year(), now.Month(), now.Day())

	return now.After(rise) && now.Before(set)
}

func (s *SunTimes) IsNightTime() bool {
	return !s.IsDayTime()
}

func (s *SunTimes) Sunrise() time.Time {
	now := time.Now()

	rise, _ := sunrise.SunriseSunset(s.location.Latitude, s.location.Longitude, now.Year(), now.Month(), now.Day())

	return rise
}

func (s *SunTimes) Sunset() time.Time {
	now := time.Now()

	_, set := sunrise.SunriseSunset(s.location.Latitude, s.location.Longitude, now.Year(), now.Month(), now.Day())

	return set
}
