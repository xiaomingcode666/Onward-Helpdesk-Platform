# Platform workflow manifests

These JSON files are the source of truth for platform-built-in workflows. They
are embedded in the backend binary and materialized into the database after
schema migrations on every application startup.

Change an existing manifest only through a reviewed code change. A definition
hash change creates a new immutable stable Workflow Version, but it never
reviews or deploys a product Agent Release automatically.
