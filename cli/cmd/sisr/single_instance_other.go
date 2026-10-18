//go:build !windows

package sisr

func acquireSingleInstance() (func(), error) {
	return func() {}, nil
}
