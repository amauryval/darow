package modules

import (
	"testing"

	"github.com/MyAppLibs/go-pkg/apidoc"
	"github.com/MyAppLibs/go-pkg/config"
	"github.com/MyAppLibs/go-pkg/paths"
	"github.com/MyAppLibs/go-pkg/settings"
	"github.com/MyAppLibs/go-pkg/starter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amauryval/darow/docs"
)

func Test_APIDescription(t *testing.T) {
	workDir := t.TempDir()
	modules, err := starter.BuildModules(workDir, RegisterFlags()...)
	require.NoError(t, err)
	app := starter.NewAppStarter("127.0.0.1", 0).WithDocs(docs.Docs())
	require.NoError(t, app.Setup(config.Config{WorkDir: workDir, Backup: settings.DefaultBackupConfig()}, modules...))
	t.Cleanup(func() { _ = app.AppShutdown() })
	description := docs.Docs().API

	cases := []struct {
		name  string
		check func() ([]string, error)
	}{
		{name: "Every served route is described", check: func() ([]string, error) {
			return apidoc.Missing(app.Routes(), paths.APIV1Root(), description)
		}},
		{name: "Every described operation names its access", check: func() ([]string, error) {
			return apidoc.Unscoped(description)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.check()
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	}
}
