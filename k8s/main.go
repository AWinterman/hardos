package main

import (
	"fmt"
	"github.com/pulumi/pulumi-linode/sdk/v3/go/linode"
	tls "github.com/pulumi/pulumi-tls/sdk/v4/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"strconv"
)

const Region = "ca-central"

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		// Create a linode resource (Linode Instance)
		controllers, err := Nodes(ctx, Region, "controller")
		if err != nil {
			return err
		}

		workers, err := Nodes(ctx, Region, "worker")
		if err != nil {
			return err
		}
		// create a Firewall (restrict access to specific ports/ips)
		firewall, err := Firewall(ctx, append(controllers, workers...), Region)
		if err != nil {
			return err
		}

		balancer, err := LoadBalancer(ctx, Region, append(controllers, workers...))
		if err != nil {
			return err
		}
		ipv4 := balancer.Ipv4

		ctx.Export("Firewall", firewall)
		ctx.Export("LoadBalancer", balancer)
		ctx.Export("publicIp", ipv4)
		return nil
	})
}

func Nodes(ctx *pulumi.Context, region string, label string) ([]*linode.Instance, error) {
	conf := config.New(ctx, "")

	instances := make([]*linode.Instance, 3)
	me, err := linode.GetProfile(ctx, nil, nil)
	if err != nil {
		return instances, err
	}
	for i, _ := range instances {
		if err != nil {
			return instances, err
		}
		name := fmt.Sprintf("%s-node-%d", label, i)
		instance, err := linode.NewInstance(ctx, name, &linode.InstanceArgs{
			Type:      pulumi.String("g6-nanode-1"),
			Region:    pulumi.String(region),
			Image:     pulumi.String("linode/alpine3.15"),
			PrivateIp: pulumi.Bool(true),
			RootPass:  conf.RequireSecret("node_password"),
			AuthorizedUsers: pulumi.StringArray{
				pulumi.String(me.Username),
			},
			Interfaces: linode.InstanceInterfaceArray{
				linode.InstanceInterfaceArgs{Purpose: pulumi.String("public")},
				linode.InstanceInterfaceArgs{Purpose: pulumi.String("vlan"), Label: pulumi.StringPtr("internal")},
			},
			Label: pulumi.String(name),
			Tags:  pulumi.StringArray{pulumi.String(name)},
		})
		if err != nil {
			return instances, err
		}

		instances[i] = instance
		ctx.Export(name, instance)
	}
	return instances, nil
}

func LoadBalancer(ctx *pulumi.Context, region string, instances []*linode.Instance) (*linode.NodeBalancer, error) {

	var nodeId pulumi.IntArray
	for _, instance := range instances {
		instanceId := instance.ID().ToStringOutput().ApplyT(parseString()).(pulumi.IntInput)
		nodeId = append(nodeId, instanceId)

	}

	balancer, err := linode.NewNodeBalancer(ctx, "LoadBalancer", &linode.NodeBalancerArgs{
		ClientConnThrottle: pulumi.Int(20),
		Label:              pulumi.String("LoadBalancer"),
		Region:             pulumi.String(region),
	})
	if err != nil {
		return nil, err
	}

	balancerId := balancer.ID().ToStringOutput().ApplyT(parseString()).(pulumi.IntInput)

	args := linode.NodeBalancerConfigArgs{
		Algorithm:      pulumi.String("leastconn"),
		Check:          pulumi.String("connection"),
		CheckAttempts:  pulumi.Int(3),
		CheckInterval:  pulumi.Int(10),
		CheckPath:      pulumi.String("/healthz"),
		CheckTimeout:   pulumi.Int(5),
		NodebalancerId: balancerId,
		Port:           nil,
		Protocol:       nil,
		ProxyProtocol:  nil,
		SslCert:        nil,
		SslKey:         nil,
		Stickiness:     nil,
	}
	balancerConfig, err := linode.NewNodeBalancerConfig(ctx, "LoadBalancer-config", &args)
	ctx.Export("LoadBalancer-conf-id", balancerConfig.ID())
	return balancer, nil
}

func Firewall(ctx *pulumi.Context, instances []*linode.Instance, region string) (*linode.Firewall, error) {
	var linodes pulumi.IntArray
	for _, instance := range instances {
		instanceId := instance.ID().ToStringOutput().ApplyT(parseString()).(pulumi.IntInput)
		linodes = append(linodes, instanceId)
	}

	allIps := pulumi.String("0.0.0.0/0")
	acceptAction := pulumi.String("ACCEPT")

	tcp := pulumi.String("TCP")
	udp := pulumi.String("UDP")
	icmp := pulumi.String("ICMP")

	allPorts := pulumi.String("1-65535")
	var wall, err = linode.NewFirewall(ctx, "Firewall", &linode.FirewallArgs{
		Label: pulumi.String("Firewall"),
		Tags: pulumi.StringArray{
			pulumi.String("test"),
		},
		InboundPolicy:  pulumi.String("DROP"),
		OutboundPolicy: pulumi.String("DROP"),
		Inbounds: linode.FirewallInboundArray{
			linode.FirewallInboundArgs{
				Action:   acceptAction,
				Ipv4s:    pulumi.StringArray{allIps},
				Label:    pulumi.String("accept-inbound-SSH"),
				Ports:    pulumi.String("22"),
				Protocol: tcp,
			},
			linode.FirewallInboundArgs{
				Action:   acceptAction,
				Ipv4s:    pulumi.StringArray{allIps},
				Label:    pulumi.String("control-ingress"),
				Ports:    pulumi.String("6443"),
				Protocol: tcp,
			},
			linode.FirewallInboundArgs{
				Action: acceptAction,
				Ipv4s: pulumi.StringArray{
					pulumi.String("192.168.0.0/16"),
				},
				Label:    pulumi.String("TCP-inter-node"),
				Ports:    allPorts,
				Protocol: tcp,
			},
			linode.FirewallInboundArgs{
				Action: acceptAction,
				Ipv4s: pulumi.StringArray{
					pulumi.String("192.168.0.0/16"),
				},
				Label:    pulumi.String("UDP-inter-node"),
				Ports:    allPorts,
				Protocol: udp,
			},
			linode.FirewallInboundArgs{
				Action: acceptAction,
				Ipv4s: pulumi.StringArray{
					pulumi.String("10.200.0.0/16"),
				},
				Label:    pulumi.String("TCP-pods-traffic"),
				Ports:    allPorts,
				Protocol: tcp,
			},
			linode.FirewallInboundArgs{
				Action: acceptAction,
				Ipv4s: pulumi.StringArray{
					pulumi.String("10.200.0.0/16"),
				},
				Label:    pulumi.String("UDP-pods-traffic"),
				Ports:    allPorts,
				Protocol: udp,
			},
			linode.FirewallInboundArgs{
				Action: acceptAction,
				Ipv4s: pulumi.StringArray{
					allIps,
				},
				Label:    pulumi.String("ICMP-ingress-for-pings"),
				Protocol: icmp,
			},
		},
		Outbounds: linode.FirewallOutboundArray{
			linode.FirewallOutboundArgs{
				Action:   acceptAction,
				Ipv4s:    pulumi.StringArray{allIps},
				Label:    pulumi.String("ICMP-egress-for-pings"),
				Protocol: icmp,
			},
			linode.FirewallOutboundArgs{
				Action:   acceptAction,
				Ipv4s:    pulumi.StringArray{allIps},
				Label:    pulumi.String("TCP-egress"),
				Protocol: tcp,
				Ports:    allPorts,
			},
			linode.FirewallOutboundArgs{
				Action:   acceptAction,
				Ipv4s:    pulumi.StringArray{allIps},
				Label:    pulumi.String("UDP-egress"),
				Protocol: udp,
				Ports:    allPorts,
			},
		},
		Linodes: linodes,
	})

	return wall, err
}

// https://github.com/equinix-labs/kubernetes-cloud-init/blob/main/src/kubernetes/control-plane/certificates.ts
func CertificateAuthority(ctx *pulumi.Context, clusterName string) (interface{}, error) {
	key, err := tls.NewPrivateKey(ctx, "private-key", &tls.PrivateKeyArgs{
		Algorithm: pulumi.String("rsa"),
		RsaBits:   pulumi.Int(2048),
	})
	if err != nil {
		return nil, err
	}
	uses := stringsToStringArray("signing", "key encipherment", "server auth", "client auth")

	cert, err := tls.NewSelfSignedCert(ctx, "certificate-authority", &tls.SelfSignedCertArgs{
		AllowedUses:     uses,
		IsCaCertificate: pulumi.Bool(true),
		KeyAlgorithm:    key.Algorithm,
		PrivateKeyPem:   key.PrivateKeyPem,
		Subjects: tls.SelfSignedCertSubjectArray{
			tls.SelfSignedCertSubjectArgs{
				CommonName: pulumi.String(clusterName),
			},
		},
		ValidityPeriodHours: pulumi.Int(8760),
		EarlyRenewalHours:   pulumi.Int(168),
	})
	
	if err != nil {
		return nil, err
	}

	request, err := tls.NewCertRequest(ctx, "admin cert request", &tls.CertRequestArgs{
		KeyAlgorithm:  key.Algorithm,
		PrivateKeyPem: key.PrivateKeyPem,
		Subjects: tls.CertRequestSubjectArray{
			tls.CertRequestSubjectArgs{
				CommonName:         pulumi.String("admin"),
				Country:            pulumi.String("US"),
				Locality:           pulumi.String("Portland"),
				Organization:       pulumi.String("hardos"),
				OrganizationalUnit: pulumi.String("system:masters"),
				Province:           pulumi.String("Oregon"),
			},
		},
		Uris: nil,
	},
	)
	if err != nil {
		return nil, err
	}

	signedCert, err := tls.NewLocallySignedCert(ctx, "admin cert", &tls.LocallySignedCertArgs{
		AllowedUses:     uses,
		CaCertPem:       cert.CertPem,
		CaKeyAlgorithm:  cert.KeyAlgorithm,
		CaPrivateKeyPem: cert.PrivateKeyPem,
		CertRequestPem:  request.CertRequestPem,
	})
	if err != nil {
		return nil, err
	}

}

func parseString() func(id string) int {
	return func(id string) int {
		// a goofy workaround for a type mismatch https://github.com/pulumi/pulumi-terraform-bridge/issues/352
		var idInt int
		idInt, err := strconv.Atoi(id)
		if err != nil {
			fmt.Println(err)
			return idInt
		}
		return idInt
	}
}

func stringsToStringArray(ss ...string) pulumi.StringArray {
	array := pulumi.StringArray{}
	for _, s := range ss {
		array = append(array, pulumi.String(s))
	}
	return array
}
