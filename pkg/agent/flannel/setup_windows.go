package flannel

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	cniConf_vxlan = `
{
	"name": "vxlan0",
	"cniVersion": "0.3.0",
	"type": "flannel",
	"capabilities": {
	  "dns": true
	},
	"delegate": {
	  "type": "win-overlay",
	  "policies": [
		{
		  "Name": "EndpointPolicy",
		  "Value": {
			"Type": "OutBoundNAT",
			"ExceptionList": ["%SERVICECIDR%","%CLUSTERCIDR%"]
		  }
		},
		{
		  "Name": "EndpointPolicy",
		  "Value": {
			"Type": "ROUTE",
			"DestinationPrefix": "%SERVICECIDR%",
			"NeedEncap": true
		  }
		}
	  ]
	}
  }
`

	cniConf_hostgw = `
	{
		"name": "cbr0",
		"cniVersion": "0.3.0",
		"type": "flannel",
		"capabilities": {
		  "dns": true
		},
		"delegate": {
		  "type": "win-bridge",
		  "hairpinMode": true,
		  "isDefaultGateway": true,
		  "policies": [
			{
			  "Name": "EndpointPolicy",
			  "Value": {
				"Type": "OutBoundNAT",
				"ExceptionList": ["%SERVICECIDR%","%CLUSTERCIDR%","%HOSTCIDR%"]
			  }
			},
			{
			  "Name": "EndpointPolicy",
			  "Value": {
				"Type": "ROUTE",
				"DestinationPrefix": "%SERVICECIDR%",
				"NeedEncap": true
			  }
			},
			{
			  "Name": "EndpointPolicy",
			  "Value": {
				"Type": "ROUTE",
				"DestinationPrefix": "%HOSTIPCIDR%",
				"NeedEncap": true
			  }
			}
		  ]
		}
	  }
	`

	flannelConf = `{
	"Network": "%CIDR%",
	"Backend": %backend%
}
`

	vxlanBackend = `{
	"Name": "vxlan0",
	"Type": "vxlan"
}`

	hostGWBackend = `{
	"Name": "cbr0",
	"Type": "host-gw"
}`
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {

	flannelIface := "Ethernet"
	if nodeConfig.FlannelIface != nil {
		if nodeConfig.FlannelIface.Name != "" {
			flannelIface = nodeConfig.FlannelIface.Name
		}
	}

	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendVXLAN:
		setupOverlay(nodeConfig.AgentConfig.NodeConfigPath, flannelIface)
	case config.FlannelBackendHostGW:
		setupL2bridge(nodeConfig.AgentConfig.NodeConfigPath, flannelIface)
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}

	// Do not setup cni if flannel conf specified
	if !nodeConfig.FlannelConfOverride && nodeConfig.FlannelConf != "" {
		if err := createCNIConf(nodeConfig); err != nil {
			return err
		}
	}

	return createFlannelConf(nodeConfig)
}

func Run(ctx context.Context, nodeConfig *config.Node, nodes v1.NodeInterface) error {
	nodeName := nodeConfig.AgentConfig.NodeName

	for {
		node, err := nodes.Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil && node.Spec.PodCIDR != "" {
			break
		}
		if err == nil {
			logrus.Infof("waiting for node %s CIDR not assigned yet", nodeName)
		} else {
			logrus.Infof("waiting for node %s: %v", nodeName, err)
		}
		time.Sleep(2 * time.Second)
	}

	go func() {
		err := flannel(ctx, nodeConfig.FlannelIface, nodeConfig.FlannelConf, nodeConfig.AgentConfig.KubeConfigKubelet)
		logrus.Fatalf("flannel exited: %v", err)
	}()

	return nil
}

func createCNIConf(nodeConfig *config.Node) error {
	if nodeConfig.AgentConfig.CNIConfDir == "" {
		return nil
	}
	p := filepath.Join(nodeConfig.AgentConfig.CNIConfDir, "10-flannel.conf")
	cniConf := ""
	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendVXLAN:
		cniConf = cniConf_vxlan
	case config.FlannelBackendHostGW:
		cniConf = cniConf_hostgw
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON := strings.Replace(cniConf, "%CLUSTERCIDR%", nodeConfig.AgentConfig.ClusterCIDR.String(), -1)
	// TODO: figure out how to fetch service cidr
	confJSON = strings.Replace(confJSON, "%SERVICECIDR%", "10.43.0.0/16", -1)
	confJSON = strings.Replace(confJSON, "%HOSTCIDR%", "10.231.120.177/24", -1)
	confJSON = strings.Replace(confJSON, "%HOSTIPCIDR%", "10.231.120.177/32", -1)

	return util.WriteFile(p, confJSON)
}

func createFlannelConf(nodeConfig *config.Node) error {
	if nodeConfig.FlannelConf == "" {
		return nil
	}
	if nodeConfig.FlannelConfOverride {
		logrus.Infof("Using custom flannel conf defined at %s", nodeConfig.FlannelConf)
		return nil
	}
	confJSON := strings.Replace(flannelConf, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String(), -1)

	var backendConf string

	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendVXLAN:
		backendConf = vxlanBackend
	case config.FlannelBackendHostGW:
		backendConf = hostGWBackend
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON = strings.Replace(confJSON, "%backend%", backendConf, -1)

	return util.WriteFile(nodeConfig.FlannelConf, confJSON)
}
