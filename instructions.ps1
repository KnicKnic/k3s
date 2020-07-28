#cleanup 
del C:\tmp\k3s\ -Recurse -Force

#setup environment
echo """
nameserver 8.8.8.8
""" > /etc/resolv.conf

#add paths, #note k3s is where my host-local files are
# $env:KUBECONFIG="C:\tmp\k3s\server\cred\admin.kubeconfig"
$env:KUBECONFIG="C:\Users\Administrator\.kube\k3s.yaml"
$env:Path +=";C:\Users\Administrator\go\src\github.com\rancher\k3s"
$env:Path +=";C:\tmp"

#optional resolve-conf --resolv-conf C:\etc\resolv.conf 

$hostNetwork = get-NetIPAddress -InterfaceAlias "vEthernet (Ethernet)"| ?{$_.AddressFamily -eq "IPv4"}
$env:hostIp = $hostNetwork.IpAddress
$env:hostCidr = "{0}/{1}" -f $hostNetwork.IpAddress, $hostNetwork.PrefixLength

#for host-gw
#eventually need to get rid of KUBE_NETWORK
$env:KUBE_NETWORK="cbr0"
.\k3s.exe server -d c:\tmp\k3s  --flannel-backend host-gw --docker --disable-network-policy --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0 --disable servicelb,traefik,local-storage,metrics-server 

# Known issues, currently it does not seem that kube-proxy works for containers contacting services that are on the host
# this is an issue as kubernetes (api-server) routes to an IP on a host for a one node k3s
# it should work just fine if Linux (off-box) is the api server
# 21397931


# for vxlan0
# Not tested... I think I have to setup a special vip and record that
#eventually need to get rid of KUBE_NETWORK
$env:KUBE_NETWORK="vxlan0"
.\k3s.exe server -d c:\tmp\k3s  --flannel-backend vxlan --docker --disable-network-policy --pause-image mcr.microsoft.com/k8s/core/pause:1.0.0 --disable servicelb,traefik,local-storage,metrics-server 



#ensure you have an uptodate HNS should be 9.2
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

#cleanup
ipmo C:\etc\rancher\node\hns.psm1
Get-HNSEndpoint | Remove-HNSEndpoint
Get-HNSNetwork | ? Name -Like "cbr0" | Remove-HNSNetwork
Get-HNSNetwork | ? Name -Like "vxlan0" | Remove-HNSNetwork
# Get-HnsNetwork |?{$_.name -eq "External"} |Remove-HnsNetwork
Get-HnsPolicyList | Remove-HnsPolicyList
