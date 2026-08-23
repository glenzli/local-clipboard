//go:build !darwin

package client

func ReadFileRefs() ([]string, error) {
	return nil, nil
}
