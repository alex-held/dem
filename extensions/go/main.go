package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mholt/archiver"
	"github.com/samuelngs/dem/pkg/ext"
	"github.com/samuelngs/dem/pkg/util/envcomposer"
	"github.com/samuelngs/dem/pkg/util/fs"
	"github.com/samuelngs/dem/pkg/workspaceconfig"
	"gopkg.in/yaml.v2"
)

// Example of .workspace.yaml:
//
// workspace:
//   shell:
//     program: /bin/zsh
//   with:
//     go:
//       version: 1.11.2
//       go_path: false
//       go_111_module: auto

var goBinaryHost = "https://dl.google.com/go"

type plugin struct {
	wsconf *workspaceconfig.Config
	goconf *goConfig
}

type config struct {
	Workspace *workspaceConfig `yaml:"workspace"`
}

type workspaceConfig struct {
	With *withConfig `yaml:"with"`
}

type withConfig struct {
	Go *goConfig `yaml:"go"`
}

type goConfig struct {
	Version     string `yaml:"version"`
	GoPath      string `yaml:"go_path"`
	Go111Module string `yaml:"go_111_module"`
}

func (v *plugin) Init(wsconf *workspaceconfig.Config) (bool, error) {
	var goconf *config
	if err := yaml.Unmarshal(wsconf.Src, &goconf); err != nil {
		return false, err
	}
	if goconf == nil || goconf.Workspace.With.Go == nil || len(goconf.Workspace.With.Go.Version) == 0 {
		return false, nil
	}
	v.wsconf = wsconf
	v.goconf = goconf.Workspace.With.Go
	return true, nil
}

func (v *plugin) SetupTasks() ext.SetupTasks {
	var (
		path = filepath.Join(v.wsconf.WorkingDir, ".go", v.goconf.Version)
		bin  = filepath.Join(path, "go", "bin", "go")
		tar  = fmt.Sprintf("go%s.%s-%s.tar.gz", v.goconf.Version, runtime.GOOS, runtime.GOARCH)
		url  = fmt.Sprintf("%s/%s", goBinaryHost, tar)
		tmp  = filepath.Join(v.wsconf.WorkingDir, ".go", "release")
		file = filepath.Join(tmp, tar)
	)
	if fs.Exists(bin) {
		return nil
	}
	return ext.SetupTasks{
		ext.Procedure("initializing", func(bar ext.ProgressBar) error {
			fs.Mkdir(path)
			fs.Mkdir(tmp)
			return nil
		}),
		ext.Procedure("downloading", func(bar ext.ProgressBar) error {
			out, err := os.Create(file)
			if err != nil {
				return err
			}
			defer out.Close()
			rsp, err := http.Get(url)
			if err != nil {
				return err
			}
			defer rsp.Body.Close()
			_, err = io.Copy(out, rsp.Body)
			if err != nil {
				return err
			}
			return nil
		}),
		ext.Procedure("unpacking", func(bar ext.ProgressBar) error {
			if err := archiver.NewTarGz().Unarchive(file, path); err != nil {
				return err
			}
			return nil
		}),
	}
}

func (v *plugin) Environment() map[string]string {
	composer := envcomposer.New()
	if len(v.goconf.GoPath) > 0 && v.goconf.GoPath != "false" {
		composer.Set("GOPATH", v.goconf.GoPath)
	}
	if len(v.goconf.Go111Module) > 0 {
		composer.Set("GO111MODULE", v.goconf.Go111Module)
	}
	return composer.AsMap()
}

func (v *plugin) Aliases() map[string]string {
	return nil
}

func (v *plugin) Paths() []string {
	return []string{filepath.Join(v.wsconf.WorkingDir, ".go", v.goconf.Version, "go", "bin")}
}

func (v *plugin) String() string {
	return "Go"
}

// Export is a plugin instance used for workspace
var Export = ext.Extension(new(plugin))
