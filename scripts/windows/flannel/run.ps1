# https://github.com/kubernetes-sigs/sig-windows-tools/tree/master/kubeadm/flannel

$ErrorActionPreference = "Stop";
    mkdir -force /etc/cni/net.d
    # mkdir -force /etc/kube-flannel
    # mkdir -force /opt/cni/bin
    $cniJson = get-content .\cni-conf.json | ConvertFrom-Json
    # $serviceSubnet = yq r /etc/kubeadm-config/ClusterConfiguration networking.serviceSubnet
    # $podSubnet = yq r /etc/kubeadm-config/ClusterConfiguration networking.podSubnet
    $serviceSubnet = "10.43.0.0/16"
    $podSubnet = "10.42.0.0/24"
    $interfaceName = "Ethernet"

    # $networkJson = wins cli net get | convertfrom-json
    $cniJson.delegate.policies[0].Value.ExceptionList = $serviceSubnet, $podSubnet
    $cniJson.delegate.policies[1].Value.DestinationPrefix = $serviceSubnet
    Set-Content -Path /etc/cni/net.d/10-flannel.conf ($cniJson | ConvertTo-Json -depth 100)

    type /etc/cni/net.d/10-flannel.conf
    cp /etc/cni/net.d/10-flannel.conf C:\tmp\k3s\agent\etc\cni\net.d\10-flannel.conf

    ipmo .\hns.psm1; 
    New-HNSNetwork -Type Overlay -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -Name "External" -AdapterName $interfaceName -SubnetPolicies @(@{Type = "VSID"; VSID = 9999; })




    # this is the flannel config, need to figure out where to get it from
    # cp -force /etc/kube-flannel/net-conf.json /host/etc/kube-flannel
    cp -force -recurse /cni/* /host/opt/cni/bin
    cp -force /k/flannel/* /host/k/flannel/
    cp -force /kube-proxy/kubeconfig.conf /host/k/flannel/kubeconfig.yml
    cp -force /var/run/secrets/kubernetes.io/serviceaccount/* /host/k/flannel/var/run/secrets/kubernetes.io/serviceaccount/
    wins cli process run --path /k/flannel/setup.exe --args "--mode=overlay --interface=Ethernet"
    wins cli route add --addresses 169.254.169.254
    wins cli process run --path /k/flannel/flanneld.exe --args "--kube-subnet-mgr --kubeconfig-file /k/flannel/kubeconfig.yml" --envs "POD_NAME=$env:POD_NAME POD_NAMESPACE=$env:POD_NAMESPACE"
  