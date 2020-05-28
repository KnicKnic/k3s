package flannel

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/rancher/k3s/pkg/agent/util"
)

const (
	// this code comes from https://github.com/kubernetes-sigs/sig-windows-tools/blob/master/kubeadm/flannel/hns.psm1
	// also contains GetSourceVip (modified) from https://github.com/microsoft/SDN/blob/d56fe83dfa167bfd5cdff1666bb5d2275662dec4/Kubernetes/windows/helper.psm1
	hns string = `
#########################################################################
# Global Initialize
function Get-VmComputeNativeMethods()
{
        $signature = @'
                     [DllImport("vmcompute.dll")]
                     public static extern void HNSCall([MarshalAs(UnmanagedType.LPWStr)] string method, [MarshalAs(UnmanagedType.LPWStr)] string path, [MarshalAs(UnmanagedType.LPWStr)] string request, [MarshalAs(UnmanagedType.LPWStr)] out string response);
'@

    # Compile into runtime type
    Add-Type -MemberDefinition $signature -Namespace VmCompute.PrivatePInvoke -Name NativeMethods -PassThru
}

#########################################################################
# Configuration
#########################################################################
function Get-HnsSwitchExtensions
{
    param
    (
        [parameter(Mandatory=$true)] [string] $NetworkId
    )

    return (Get-HNSNetwork $NetworkId).Extensions
}

function Set-HnsSwitchExtension
{
    param
    (
        [parameter(Mandatory=$true)] [string] $NetworkId,
        [parameter(Mandatory=$true)] [string] $ExtensionId,
        [parameter(Mandatory=$true)] [bool]   $state
    )

    # { "Extensions": [ { "Id": "...", "IsEnabled": true|false } ] }
    $req = @{
        "Extensions"=@(@{
            "Id"=$ExtensionId;
            "IsEnabled"=$state;
        };)
    }
    Invoke-HNSRequest -Method POST -Type networks -Id $NetworkId -Data (ConvertTo-Json $req)
}

#########################################################################
# Activities
#########################################################################
function Get-HNSActivities
{
    [cmdletbinding()]Param()
    return Invoke-HNSRequest -Type activities -Method GET
}

#########################################################################
# PolicyLists
#########################################################################
function Get-HNSPolicyList {
    [cmdletbinding()]Param()
    return Invoke-HNSRequest -Type policylists -Method GET
}

function Remove-HnsPolicyList
{
    [CmdletBinding()]
    param
    (
        [parameter(Mandatory=$true,ValueFromPipeline=$True,ValueFromPipelinebyPropertyName=$True)]
        [Object[]] $InputObjects
    )
    begin {$Objects = @()}
    process {$Objects += $InputObjects; }
    end {
        $Objects | foreach {  Invoke-HNSRequest -Method DELETE -Type  policylists -Id $_.Id }
    }
}

function New-HnsRoute {
    param
    (
        [parameter(Mandatory = $false)] [Guid[]] $Endpoints = $null,
        [parameter(Mandatory = $true)] [string] $DestinationPrefix,
        [parameter(Mandatory = $false)] [switch] $EncapEnabled
    )

    $policyLists = @{
        References = @(
            get-endpointReferences $Endpoints;
        );
        Policies   = @(
            @{
                Type = "ROUTE";
                DestinationPrefix = $DestinationPrefix;
                NeedEncap = $EncapEnabled.IsPresent;
            }
        );
    }

    Invoke-HNSRequest -Method POST -Type policylists -Data (ConvertTo-Json  $policyLists -Depth 10)
}

function New-HnsLoadBalancer {
    param
    (
        [parameter(Mandatory = $false)] [Guid[]] $Endpoints = $null,
        [parameter(Mandatory = $true)] [int] $InternalPort,
        [parameter(Mandatory = $true)] [int] $ExternalPort,
        [parameter(Mandatory = $false)] [string] $Vip
    )

    $policyLists = @{
        References = @(
            get-endpointReferences $Endpoints;
        );
        Policies   = @(
            @{
                Type = "ELB";
                InternalPort = $InternalPort;
                ExternalPort = $ExternalPort;
                VIPs = @($Vip);
            }
        );
    }

    Invoke-HNSRequest -Method POST -Type policylists -Data ( ConvertTo-Json  $policyLists -Depth 10)
}


function get-endpointReferences {
    param
    (
        [parameter(Mandatory = $true)] [Guid[]] $Endpoints = $null
    )
    if ($Endpoints ) {
        $endpointReference = @()
        foreach ($endpoint in $Endpoints)
        {
            $endpointReference += "/endpoints/$endpoint"
        }
        return $endpointReference
    }
    return @()
}

#########################################################################
# Networks
#########################################################################
function New-HnsNetwork
{
    param
    (
        [parameter(Mandatory=$false, Position=0)]
        [string] $JsonString,
        [ValidateSet('ICS', 'Internal', 'Transparent', 'NAT', 'Overlay', 'L2Bridge', 'L2Tunnel', 'Layered', 'Private')]
        [parameter(Mandatory = $false, Position = 0)]
        [string] $Type,
        [parameter(Mandatory = $false)] [string] $Name,
        [parameter(Mandatory = $false)] $AddressPrefix,
        [parameter(Mandatory = $false)] $Gateway,
        [HashTable[]][parameter(Mandatory=$false)] $SubnetPolicies, #  @(@{VSID = 4096; })

        [parameter(Mandatory = $false)] [switch] $IPv6,
        [parameter(Mandatory = $false)] [string] $DNSServer,
        [parameter(Mandatory = $false)] [string] $AdapterName,
        [HashTable][parameter(Mandatory=$false)] $AdditionalParams, #  @ {"ICSFlags" = 0; }
        [HashTable][parameter(Mandatory=$false)] $NetworkSpecificParams #  @ {"InterfaceConstraint" = ""; }
    )

    Begin {
        if (!$JsonString) {
            $netobj = @{
                Type          = $Type;
            };

            if ($Name) {
                $netobj += @{
                    Name = $Name;
                }
            }

            # Coalesce prefix/gateway into subnet objects.
            if ($AddressPrefix) {
                $subnets += @()
                $prefixes = @($AddressPrefix)
                $gateways = @($Gateway)

                $len = $prefixes.length
                for ($i = 0; $i -lt $len; $i++) {
                    $subnet = @{ AddressPrefix = $prefixes[$i]; }
                    if ($i -lt $gateways.length -and $gateways[$i]) {
                        $subnet += @{ GatewayAddress = $gateways[$i]; }

                        if ($SubnetPolicies) {
                            $subnet.Policies += $SubnetPolicies
                        }
                    }

                    $subnets += $subnet
                }

                $netobj += @{ Subnets = $subnets }
            }

            if ($IPv6.IsPresent) {
                $netobj += @{ IPv6 = $true }
            }

            if ($AdapterName) {
                $netobj += @{ NetworkAdapterName = $AdapterName; }
            }

            if ($AdditionalParams) {
                $netobj += @{
                    AdditionalParams = @{}
                }

                foreach ($param in $AdditionalParams.Keys) {
                    $netobj.AdditionalParams += @{
                        $param = $AdditionalParams[$param];
                    }
                }
            }

            if ($NetworkSpecificParams) {
                $netobj += $NetworkSpecificParams
            }

            $JsonString = ConvertTo-Json $netobj -Depth 10
        }

    }
    Process{
        return Invoke-HnsRequest -Method POST -Type networks -Data $JsonString
    }
}


#########################################################################
# Endpoints
#########################################################################
function New-HnsEndpoint
{
    param
    (
        [parameter(Mandatory=$false, Position = 0)] [string] $JsonString = $null,
        [parameter(Mandatory = $false, Position = 0)] [Guid] $NetworkId,
        [parameter(Mandatory = $false)] [string] $Name,
        [parameter(Mandatory = $false)] [string] $IPAddress,
        [parameter(Mandatory = $false)] [string] $Gateway,
        [parameter(Mandatory = $false)] [string] $MacAddress,
        [parameter(Mandatory = $false)] [switch] $EnableOutboundNat
    )

    begin
    {
        if ($JsonString)
        {
            $EndpointData = $JsonString | ConvertTo-Json | ConvertFrom-Json
        }
        else
        {
            $endpoint = @{
                VirtualNetwork = $NetworkId;
                Policies       = @();
            }

            if ($Name) {
                $endpoint += @{
                    Name = $Name;
                }
            }

            if ($MacAddress) {
                $endpoint += @{
                    MacAddress     = $MacAddress;
                }
            }

            if ($IPAddress) {
                $endpoint += @{
                    IPAddress      = $IPAddress;
                }
            }

            if ($Gateway) {
                $endpoint += @{
                    GatewayAddress = $Gateway;
                }
            }

            if ($EnableOutboundNat) {
                $endpoint.Policies += @{
                    Type = "OutBoundNAT";
                }

            }
            # Try to Generate the data
            $EndpointData = convertto-json $endpoint
        }
    }

    Process
    {
        return Invoke-HNSRequest -Method POST -Type endpoints -Data $EndpointData
    }
}


function New-HnsRemoteEndpoint
{
    param
    (
        [parameter(Mandatory = $true)] [Guid] $NetworkId,
        [parameter(Mandatory = $false)] [string] $IPAddress,
        [parameter(Mandatory = $false)] [string] $MacAddress
    )

    $remoteEndpoint = @{
        ID = [Guid]::NewGuid();
        VirtualNetwork = $NetworkId;
        IPAddress = $IPAddress;
        MacAddress = $MacAddress;
        IsRemoteEndpoint = $true;
    }

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $remoteEndpoint  -Depth 10)

}


function Attach-HnsHostEndpoint
{
    param
    (
     [parameter(Mandatory=$true)] [Guid] $EndpointID,
     [parameter(Mandatory=$true)] [int] $CompartmentID
     )
    $request = @{
        SystemType    = "Host";
        CompartmentId = $CompartmentID;
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request) -Action "attach" -Id $EndpointID
}

function Attach-HNSVMEndpoint
{
    param
    (
     [parameter(Mandatory=$true)] [Guid] $EndpointID,
     [parameter(Mandatory=$true)] [string] $VMNetworkAdapterName
     )

    $request = @{
        VirtualNicName   = $VMNetworkAdapterName;
        SystemType    = "VirtualMachine";
    };
    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "attach" -Id $EndpointID

}

function Attach-HNSEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID,
        [parameter(Mandatory=$true)] [int] $CompartmentID,
        [parameter(Mandatory=$true)] [string] $ContainerID
    )
     $request = @{
        ContainerId = $ContainerID;
        SystemType="Container";
        CompartmentId = $CompartmentID;
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request) -Action "attach" -Id $EndpointID
}

function Detach-HNSVMEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID
    )
    $request = @{
        SystemType  = "VirtualMachine";
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "detach" -Id $EndpointID
}

function Detach-HNSHostEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID
    )
    $request = @{
        SystemType  = "Host";
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "detach" -Id $EndpointID
}

function Detach-HNSEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID,
        [parameter(Mandatory=$true)] [string] $ContainerID
    )

    $request = @{
        ContainerId = $ContainerID;
        SystemType="Container";
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "detach" -Id $EndpointID
}
#########################################################################

function Invoke-HNSRequest
{
    param
    (
        [ValidateSet('GET', 'POST', 'DELETE')]
        [parameter(Mandatory=$true)] [string] $Method,
        [ValidateSet('networks', 'endpoints', 'activities', 'policylists', 'endpointstats', 'plugins')]
        [parameter(Mandatory=$true)] [string] $Type,
        [parameter(Mandatory=$false)] [string] $Action = $null,
        [parameter(Mandatory=$false)] [string] $Data = $null,
        [parameter(Mandatory=$false)] [Guid] $Id = [Guid]::Empty
    )

    $hnsPath = "/$Type"

    if ($id -ne [Guid]::Empty)
    {
        $hnsPath += "/$id";
    }

    if ($Action)
    {
        $hnsPath += "/$Action";
    }

    $request = "";
    if ($Data)
    {
        $request = $Data
    }

    $output = "";
    $response = "";
    Write-Verbose "Invoke-HNSRequest Method[$Method] Path[$hnsPath] Data[$request]"

    $hnsApi = Get-VmComputeNativeMethods
    $hnsApi::HNSCall($Method, $hnsPath, "$request", [ref] $response);

    Write-Verbose "Result : $response"
    if ($response)
    {
        try {
            $output = ($response | ConvertFrom-Json);
        } catch {
            Write-Error $_.Exception.Message
            return ""
        }
        if ($output.Error)
        {
             Write-Error $output;
        }
        $output = $output.Output;
    }

    return $output;
}


function Get-HNSSourceVip($nodeDir, $hostLocalDir, $NetworkName)
{
    $hnsNetwork = Get-HnsNetwork | ? Name -EQ $NetworkName.ToLower()

    # example subnet 10.42.0.0/24
    $subnet = $hnsNetwork.Subnets[0].AddressPrefix

    $ipamConfig = @"
        {"cniVersion": "0.2.0", "name": "$NetworkName", "ipam":{"type":"host-local","ranges":[[{"subnet":"$subnet"}]],"dataDir":"/var/lib/cni/networks"}}
"@

    $ipamConfig | Out-File "$nodeDir\sourceVipRequest.json"

    $env:CNI_COMMAND="ADD"
    $env:CNI_CONTAINERID="dummy"
    $env:CNI_NETNS="dummy"
    $env:CNI_IFNAME="dummy"
    $env:CNI_PATH=$hostLocalDir #path to host-local.exe

    If(!(Test-Path "$nodeDir\sourceVip.json")){
        Get-Content "$nodeDir\sourceVipRequest.json" | &"$hostLocalDir\host-local.exe" | Out-File "$nodeDir\sourceVip.json"
    }
    $sourceVipJSON = Get-Content sourceVip.json | ConvertFrom-Json 
    $sourceVip = $sourceVipJSON.ip4.ip.Split("/")[0]
    $sourceVip

    Remove-Item env:CNI_COMMAND
    Remove-Item env:CNI_CONTAINERID
    Remove-Item env:CNI_NETNS
    Remove-Item env:CNI_IFNAME
    Remove-Item env:CNI_PATH
}


function Reset-HNSNetwork()
{
    Get-HNSEndpoint | Remove-HNSEndpoint
    Get-HNSNetwork | ? Name -Like "cbr0" | Remove-HNSNetwork
    Get-HNSNetwork | ? Name -Like "vxlan0" | Remove-HNSNetwork
    Get-HnsPolicyList | Remove-HnsPolicyList
}



#########################################################################
Export-ModuleMember -Function Reset-HNSNetwork
Export-ModuleMember -Function Get-HNSSourceVip
Export-ModuleMember -Function Get-HNSActivities
Export-ModuleMember -Function Get-HnsSwitchExtensions
Export-ModuleMember -Function Set-HnsSwitchExtension

Export-ModuleMember -Function New-HNSNetwork

Export-ModuleMember -Function New-HNSEndpoint
Export-ModuleMember -Function New-HnsRemoteEndpoint

Export-ModuleMember -Function Attach-HNSHostEndpoint
Export-ModuleMember -Function Attach-HNSVMEndpoint
Export-ModuleMember -Function Attach-HNSEndpoint
Export-ModuleMember -Function Detach-HNSHostEndpoint
Export-ModuleMember -Function Detach-HNSVMEndpoint
Export-ModuleMember -Function Detach-HNSEndpoint

Export-ModuleMember -Function Get-HNSPolicyList
Export-ModuleMember -Function Remove-HnsPolicyList
Export-ModuleMember -Function New-HnsRoute
Export-ModuleMember -Function New-HnsLoadBalancer

Export-ModuleMember -Function Invoke-HNSRequest
`
)

func saveHnsScript(scriptDirectory string) string {
	p := filepath.Join(scriptDirectory, "hns.psm1")
	util.WriteFile(p, hns)
	return p
}

func setupOverlay(scriptDirectory string, interfaceName string) {
	hnsLocation := saveHnsScript(scriptDirectory)
	_ = run("ipmo  " + hnsLocation + fmt.Sprintf(`; New-HNSNetwork -Type Overlay -AddressPrefix "192.168.255.0/30"`+
		` -Gateway "192.168.255.1" -Name "External" -AdapterName "%s" -SubnetPolicies @(@{Type = "VSID"; VSID = 9999; })`,
		interfaceName),
	)
}


func setupOverlayVip(scriptDirectory string, hostLocalDir string, networkName string) string{
	hnsLocation := saveHnsScript(scriptDirectory)
	return = run("ipmo  " + hnsLocation + fmt.Sprintf(`; Get-HNSSourceVip -nodeDir "%s" -hostLocalDir "%s" -NetworkName "%s"`,
        scriptDirectory,
        hostLocalDir,
        networkName),
	)
}


func setupL2bridge(scriptDirectory string, interfaceName string) {
	hnsLocation := saveHnsScript(scriptDirectory)
	_ = run("ipmo  " + hnsLocation + fmt.Sprintf(`; New-HNSNetwork -Type l2bridge -AddressPrefix "192.168.255.0/30"`+
		` -Gateway "192.168.255.1" -Name "External" -AdapterName "%s"`,
		interfaceName),
	)
}

func resetHnsNetwork(scriptDirectory string) {
	hnsLocation := saveHnsScript(scriptDirectory)
	_ = run("ipmo  " + hnsLocation + `; Reset-HNSNetwork`)
}

func resetHnsNetwork(scriptDirectory string) {
	hnsLocation := saveHnsScript(scriptDirectory)
	_ = run("ipmo  " + hnsLocation + `; Reset-HNSNetwork`)
}

func run(command string) string {
	logrus.Info(command)
	cmd := exec.Command("powershell", "-Command", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Fatalf("Error running command: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, cmd.Stdout)
	return buf.String()
}
