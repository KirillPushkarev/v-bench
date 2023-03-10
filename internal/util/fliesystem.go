package util

import (
	"os/user"
	"path/filepath"
	"strings"
)

type PathExpander interface {
	ExpandPath(path string) string
}

type NoOpPathExpander struct {
}

func (*NoOpPathExpander) ExpandPath(path string) string {
	return path
}

type StandardPathExpander struct {
}

func (*StandardPathExpander) ExpandPath(path string) string {
	usr, _ := user.Current()
	homeDir := usr.HomeDir

	if path == "~" {
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}

	return path
}
