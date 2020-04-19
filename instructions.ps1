 $env:Path +=";C:\Users\Administrator\go\src\github.com\rancher\k3s"
.\k3s.exe server -d c:\tmp\k3s --flannel-backend none --docker  --kube-proxy-arg "proxy-mode=userspace" --disable-network-policy

Get-HnsNetwork  |?{$_.Name -eq "External"} |Remove-HnsNetwork
del C:\tmp\k3s\ -Recurse -Force






install files setting up network
$NetworkMode = "overlay"
$NetworkName = "vxlan0"
$clusterCIDR="10.244.0.0/16"
$KubeDnsServiceIP="10.96.0.10"
$serviceCIDR="10.96.0.0/12"
$InterfaceName="Ethernet"
$LogDir = "C:\k"
$BaseDir = "c:\k"




echo """
nameserver 8.8.8.8
""" > /etc/resolv.conf

del C:\tmp\k3s\ -Recurse -Force
$env:KUBECONFIG="C:\tmp\k3s\server\cred\admin.kubeconfig"
$env:Path +=";C:\Users\Administrator\go\src\github.com\rancher\k3s"
$env:Path +=";C:\tmp"
.\k3s.exe server -d c:\tmp\k3s --flannel-backend vxlan --docker  --kube-proxy-arg "proxy-mode=userspace" --disable-network-policy

.\k3s.exe server -d c:\tmp\k3s --flannel-backend host-gw --docker  --kube-proxy-arg "proxy-mode=userspace" --disable-network-policy  --flannel-conf C:\tmp\k3s\agent\etc\flannel\net-conf.json --resolv-conf C:\Users\Administrator\go\src\github.com\rancher\k3s\k3s-resolv.conf --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0

$env:KUBE_NETWORK="cbr0"
.\k3s.exe server -d c:\tmp\k3s --flannel-backend host-gw --docker  --kube-proxy-arg "proxy-mode=kernelspace" --kube-proxy-arg "cluster-cidr=10.42.0.0/16" --kube-proxy-arg "hostname-override=$($env:COMPUTERNAME)" --disable-network-policy  --flannel-conf C:\tmp\k3s\agent\etc\flannel\net-conf.json --resolv-conf C:\Users\Administrator\go\src\github.com\rancher\k3s\k3s-resolv.conf --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0



$env:KUBE_NETWORK="vxlan0"
.\k3s.exe server -d c:\tmp\k3s --docker  --kube-proxy-arg "proxy-mode=kernelspace" --kube-proxy-arg "cluster-cidr=10.42.0.0/16" --kube-proxy-arg "hostname-override=$($env:COMPUTERNAME)" --disable-network-policy  --resolv-conf C:\Users\Administrator\go\src\github.com\rancher\k3s\k3s-resolv.conf --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0

.\k3s.exe server -d c:\tmp\k3s --docker  --kube-proxy-arg "proxy-mode=kernelspace" --kube-proxy-arg "feature-gates=WinOverlay=true" --kube-proxy-arg "hostname-override=$($env:COMPUTERNAME)" --disable-network-policy  --flannel-conf C:\tmp\k3s\agent\etc\flannel\net-conf.json --resolv-conf C:\Users\Administrator\go\src\github.com\rancher\k3s\k3s-resolv.conf --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0



.\k3s.exe server -d c:\tmp\k3s --docker --disable-network-policy  --flannel-conf C:\tmp\k3s\agent\etc\flannel\net-conf.json --resolv-conf C:\Users\Administrator\go\src\github.com\rancher\k3s\k3s-resolv.conf --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0

--resolv-conf C:\Users\Administrator\go\src\github.com\rancher\k3s\k3s-resolv.conf 

$env:KUBE_NETWORK="cbr0"
.\k3s.exe server -d c:\tmp\k3s  --flannel-backend host-gw --docker --disable-network-policy --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0 --disable servicelb,traefik,local-storage,metrics-server 

--disable coredns 

--kube-proxy-arg "source-vip=10.42.0.0"


del C:\tmp\k3s\ -Recurse -Force
$env:KUBE_NETWORK="cbr0"
.\k3s.exe server -d c:\tmp\k3s  --flannel-backend host-gw --docker --disable-network-policy --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0 


function Get-VmComputeNativeMethods()
{
        $signature = @'
                     [DllImport("vmcompute.dll")]
                     public static extern void HNSCall([MarshalAs(UnmanagedType.LPWStr)] string method, [MarshalAs(UnmanagedType.LPWStr)] string path, [MarshalAs(UnmanagedType.LPWStr)] string request, [MarshalAs(UnmanagedType.LPWStr)] out string response);
'@

    # Compile into runtime type
    Add-Type -MemberDefinition $signature -Namespace VmCompute.PrivatePInvoke -Name NativeMethods -PassThru
}
$response = "";

$hnsApi = Get-VmComputeNativeMethods
$hnsApi::HNSCall("GET", "/globals/version", "", [ref] $response);
$response
