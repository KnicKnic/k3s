// +build !windows

package agent

import (
	"bufio"
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/component-base/logs"
	proxy "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubelet "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"

	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)



func startKubeProxy(cfg *config.Agent) {
	argsMap := map[string]string{
		"proxy-mode":           "iptables",
		"healthz-bind-address": "127.0.0.1",
		"kubeconfig":           cfg.KubeConfigKubeProxy,
		"cluster-cidr":         cfg.ClusterCIDR.String(),
	}
	if cfg.NodeName != "" {
		argsMap["hostname-override"] = cfg.NodeName
	}

	args := config.GetArgsList(argsMap, cfg.ExtraKubeProxyArgs)
	command := proxy.NewProxyCommand()
	command.SetArgs(args)

	go func() {
		logrus.Infof("Running kube-proxy %s", config.ArgString(args))
		logrus.Fatalf("kube-proxy exited: %v", command.Execute())
	}()
}