package deploy

import (
	"strings"
)

func convertOsFileName(fileName string) string {
	return strings.ReplaceAll(fileName, "_windows", "")
}

func osSpecificSkipFiles() map[string]bool {
	return map[string]bool{
		"ccm.yaml":             false, // i should figure out what this is
		"coredns.yaml":         true,
		"coredns_windows.yaml": false,
		"local-storage.yaml":   true,
		"metrics-server":       true,
		"rolebindings.yaml":    false,
		"traefik.yaml":         true,
	}
}
