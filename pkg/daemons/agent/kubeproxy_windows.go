package agent

import (
	// "path/filepath"
	"os"
	// "github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/sirupsen/logrus"

	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

// // TODO: need to grab sourceVip and update cidr & overwrite with .0
// // example assumes 10.42.0.0/16
// const (
// 	kubeProxyOverlayConfig = `
// winkernel:
//   sourceVip: 10.42.0.0
//   networkName: vxlan0
// apiVersion: componentconfig/v1alpha1
// kind: KubeProxyConfiguration
// `
// )

// func saveKubeProxyConfig(scriptDirectory string) string {
// 	p := filepath.Join(scriptDirectory, "kubeproxy.config")
// 	util.WriteFile(p, kubeProxyOverlayConfig)
// 	return p
// }

func startKubeProxy(cfg *config.Agent) error {

	kubeNetwork := os.Getenv("KUBE_NETWORK")

	// configPath := saveKubeProxyConfig(cfg.NodeConfigPath)

	// TODO: need to grab sourceVip and update cidr & overswrite with .0
	// example assumes 10.42.0.0/16
	argsMap := map[string]string{
		"proxy-mode":           "kernelspace",
		"healthz-bind-address": "127.0.0.1",
		"kubeconfig":           cfg.KubeConfigKubeProxy,
		"cluster-cidr":         cfg.ClusterCIDR.String(),
		// "cluster-cidr": "10.43.0.0/16",
		// "config":               configPath,
	}

	if kubeNetwork == "" || kubeNetwork == "vxlan0" {
		argsMap["network-name"] = "vxlan0"
		argsMap["feature-gates"] = "WinOverlay=true"
		// argsMap["source-vip"] = "10.42.0.8"
	}

	// if cfg.
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}

	args := config.GetArgsList(argsMap, cfg.ExtraKubeProxyArgs)
	logrus.Infof("Running kube-proxy %s", config.ArgString(args))
	return executor.KubeProxy(args)
}
