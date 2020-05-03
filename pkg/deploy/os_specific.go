// +build !windows

package deploy

func convertOsFileName(fileName string) string {
	return fileName
}

func osSpecificSkipFiles() map[string]bool {
	return map[string]bool{"coredns_windows.yaml": true}
}
