package migration

func init() {
	register(84, "add deterministic fallback to lightweight AI workflow", func() error {
		return nil
	})
}
