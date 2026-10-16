//go:build !windows

package main

import "fmt"

func loadRunConfig(path string) (runConfig, error) {
	return runConfig{}, fmt.Errorf("-config is only supported by the Windows client")
}
