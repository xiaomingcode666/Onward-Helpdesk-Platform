package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
)

func TestProjectConfigVersionOriginIdentifiesSystemVersions(t *testing.T) {
	cases := []struct {
		name    string
		version models.ProjectConfigurationVersion
		want    string
	}{
		{"migration baseline", models.ProjectConfigurationVersion{RequestKey: "system:baseline"}, ProjectConfigOriginMigrationBaseline},
		{"deployment initializer", models.ProjectConfigurationVersion{RequestKey: "initial:abc"}, ProjectConfigOriginInitialDeployment},
		{"user draft", models.ProjectConfigurationVersion{RequestKey: "draft-123456", CreatedBy: 7}, ""},
		{"user draft claiming the initial prefix", models.ProjectConfigurationVersion{RequestKey: "initial:abc", CreatedBy: 7}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, projectConfigVersionOrigin(tc.version))
		})
	}
}
