//go:build !windows

package town

import "os"

func replaceSystemEgressFile(source, destination string) error {
	return os.Rename(source, destination)
}
