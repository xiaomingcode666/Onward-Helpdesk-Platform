package migration

func init() {
	register(76, "simplify platform AI-only workflow to lightweight knowledge Q&A", func() error {
		return nil
	})
}
