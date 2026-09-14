package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectAdministratorPasswordFileIsScoped(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "27", "staging", "bootstrap-admin-password")
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0700))
	require.NoError(t, os.WriteFile(file, []byte("Password-file-fixture-2026!\n"), 0600))
	admin, err := readProjectDeploymentAdministrator(0, "fixture.admin", file, root, 27, "staging")
	require.NoError(t, err)
	require.Equal(t, "Password-file-fixture-2026!", admin.Password)
	_, err = readProjectDeploymentAdministrator(0, "fixture.admin", file, root, 28, "staging")
	require.Error(t, err)
	_, err = readProjectDeploymentAdministrator(0, "fixture.admin", file, root, 27, "production")
	require.Error(t, err)
	_, err = readProjectDeploymentAdministrator(7, "fixture.admin", file, root, 27, "staging")
	require.Error(t, err)
	_, err = readProjectDeploymentAdministrator(0, "fixture.admin", "", root, 27, "staging")
	require.Error(t, err)
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(file, 0644))
		_, err = readProjectDeploymentAdministrator(0, "fixture.admin", file, root, 27, "staging")
		require.ErrorContains(t, err, "仅文件所有者")
	}
}
