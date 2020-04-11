    $ErrorActionPreference = "Stop";

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
    
    $networkJson = wins cli net get | convertfrom-json
    $cniJson.delegate.policies[0].Value.ExceptionList = $serviceSubnet, $podSubnet, $networkJson.SubnetCIDR
    $cniJson.delegate.policies[1].Value.DestinationPrefix = $serviceSubnet
    $cniJson.delegate.policies[2].Value.DestinationPrefix = $networkJson.AddressCIDR
    Set-Content -Path /host/etc/cni/net.d/10-flannel.conf ($cniJson | ConvertTo-Json -depth 100)
    cp -force /etc/kube-flannel/net-conf.json /host/etc/kube-flannel
    cp -force -recurse /cni/* /host/opt/cni/bin
    cp -force /k/flannel/* /host/k/flannel/
    cp -force /kube-proxy/kubeconfig.conf /host/k/flannel/kubeconfig.yml
    cp -force /var/run/secrets/kubernetes.io/serviceaccount/* /host/k/flannel/var/run/secrets/kubernetes.io/serviceaccount/
    wins cli process run --path /k/flannel/setup.exe --args "--mode=l2bridge --interface=Ethernet"
    wins cli route add --addresses 169.254.169.254
 

    ipmo ..\flannel\hns.psm1; 
    New-HNSNetwork -Type l2bridge -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -Name "External" -AdapterName "Ethernet"

    mkdir -force C:\tmp\k3s\agent\etc\cni\net.d\
    mkdir -force C:\tmp\k3s\agent\etc\flannel\
    
    cp cni-conf.json C:\tmp\k3s\agent\etc\cni\net.d\10-flannel.conf
    cp .\net-conf.json C:\tmp\k3s\agent\etc\flannel\net-conf.json

