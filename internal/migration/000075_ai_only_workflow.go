package migration

func init() {
	register(75, "materialize platform AI-only customer service workflow", func() error {
		return nil
	})
}
