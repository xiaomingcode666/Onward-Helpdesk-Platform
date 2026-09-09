package dbtime

import (
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var databaseLocationCache sync.Map

// WallClockUnixMilli interprets a timestamp-without-time-zone value in the
// database session location before converting it to epoch milliseconds.
func WallClockUnixMilli(db *gorm.DB, value time.Time) int64 {
	if value.IsZero() || db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return value.UnixMilli()
	}
	location := databaseLocation(db)
	if location == nil {
		return value.UnixMilli()
	}
	return WallClockUnixMilliInLocation(value, location)
}

func WallClockUnixMilliInLocation(value time.Time, location *time.Location) int64 {
	if value.IsZero() || location == nil {
		return value.UnixMilli()
	}
	return time.Date(
		value.Year(), value.Month(), value.Day(),
		value.Hour(), value.Minute(), value.Second(), value.Nanosecond(),
		location,
	).UnixMilli()
}

func databaseLocation(db *gorm.DB) *time.Location {
	sqlDB, err := db.DB()
	if err != nil {
		return nil
	}
	if cached, ok := databaseLocationCache.Load(sqlDB); ok {
		location, _ := cached.(*time.Location)
		return location
	}
	location := loadDatabaseLocation(db)
	if location != nil {
		databaseLocationCache.Store(sqlDB, location)
	}
	return location
}

func loadDatabaseLocation(db *gorm.DB) *time.Location {
	var zone string
	if err := db.Raw("SHOW TIME ZONE").Scan(&zone).Error; err != nil {
		return nil
	}
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return nil
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil
	}
	return location
}
