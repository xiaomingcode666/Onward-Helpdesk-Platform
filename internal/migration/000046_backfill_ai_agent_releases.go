package migration

func init() {
	register(46, "retire automatic legacy release backfill in favor of manual deployment", func() error {
		return nil
	})
}
