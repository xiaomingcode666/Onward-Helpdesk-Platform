package services

import (
	"fmt"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func withDispatchScanLock(lockName string, fn func() (int, error)) (int, error) {
	db := sqls.DB()
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return fn()
	}

	var (
		ret     int
		skipped bool
	)
	err := db.Transaction(func(tx *gorm.DB) error {
		var acquired bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", lockName).Scan(&acquired).Error; err != nil {
			return fmt.Errorf("acquire dispatch scan lock %s: %w", lockName, err)
		}
		if !acquired {
			skipped = true
			return nil
		}
		count, err := fn()
		ret = count
		return err
	})
	if skipped {
		return 0, nil
	}
	return ret, err
}
