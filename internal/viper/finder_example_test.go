package viper_test

import (
	"fmt"

	"github.com/sagikazarmark/locafero"
	"github.com/spf13/afero"

	"github.com/inovacc/config/internal/viper"
)

func ExampleFinder() {
	fs := afero.NewMemMapFs()

	_ = fs.Mkdir("/home/user", 0o777)

	f, _ := fs.Create("/home/user/myapp.yaml")
	_, _ = f.WriteString("foo: bar")
	_ = f.Close()

	// HCL will have a "lower" priority in the search order
	_, _ = fs.Create("/home/user/myapp.hcl")

	finder := locafero.Finder{
		Paths: []string{"/home/user"},
		Names: locafero.NameWithExtensions("myapp", viper.SupportedExts...),
		Type:  locafero.FileTypeFile, // This is important!
	}

	v := viper.NewWithOptions(viper.WithFinder(finder))
	v.SetFs(fs)
	_ = v.ReadInConfig()

	fmt.Println(v.GetString("foo"))

	// Output:
	// bar
}

func ExampleFinders() {
	fs := afero.NewMemMapFs()

	_ = fs.Mkdir("/home/user", 0o777)
	f, _ := fs.Create("/home/user/myapp.yaml")
	_, _ = f.WriteString("foo: bar")
	_ = f.Close()

	_ = fs.Mkdir("/etc/myapp", 0o777)
	_, _ = fs.Create("/etc/myapp/config.yaml")

	// Combine multiple finders to search for files in multiple locations with different criteria
	finder := viper.Finders(
		locafero.Finder{
			Paths: []string{"/home/user"},
			Names: locafero.NameWithExtensions("myapp", viper.SupportedExts...),
			Type:  locafero.FileTypeFile, // This is important!
		},
		locafero.Finder{
			Paths: []string{"/etc/myapp"},
			Names: []string{"config.yaml"}, // Only accept YAML files in the system config directory
			Type:  locafero.FileTypeFile,   // This is important!
		},
	)

	v := viper.NewWithOptions(viper.WithFinder(finder))
	v.SetFs(fs)
	_ = v.ReadInConfig()

	fmt.Println(v.GetString("foo"))

	// Output:
	// bar
}
