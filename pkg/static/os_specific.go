// +build !windows

package static

func skipFileForOs(_ string) bool {
	return false
}
