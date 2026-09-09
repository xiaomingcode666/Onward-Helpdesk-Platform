package migration

func init() {
	register(77, "materialize lightweight AI-only workflow without activating releases", func() error {
		return nil
	})
}
