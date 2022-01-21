package main

import (
	"fmt"
	"github.com/pulumi/pulumi-linode/sdk/v3/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"strconv"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create a linode resource (Linode Instance)
		nodes, err := nodes(ctx)
		firewall, err := firewall(ctx, nodes)
		if err != nil {
			return err
		}
		ctx.Export("firewall", firewall)

		return nil
	})
}

func nodes(ctx *pulumi.Context) ([]*linode.Instance, error) {
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
		name := fmt.Sprintf("node-%d", i)
		instance, err := linode.NewInstance(ctx, name, &linode.InstanceArgs{
			Type:      pulumi.String("g6-nanode-1"),
			Region:    pulumi.String("ca-central"),
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
		})
		if err != nil {
			return instances, err
		}

		instances[i] = instance
		ctx.Export(name, instance)
	}
	return instances, nil
}

func firewall(ctx *pulumi.Context, instances []*linode.Instance) (*linode.Firewall, error) {
	var linodes pulumi.IntArray
	for _, instance := range instances {
		instanceId := instance.ID().ToStringOutput().ApplyT(func(id string) int {
			// a goofy workaround for a type mismatch https://github.com/pulumi/pulumi-terraform-bridge/issues/352
			var idInt int
			idInt, err := strconv.Atoi(id)
			if err != nil {
				fmt.Println(err)
				return idInt
			}
			return idInt
		}).(pulumi.IntInput)
		linodes = append(linodes, instanceId)
	}
	wall, err := linode.NewFirewall(ctx, "myFirewall", &linode.FirewallArgs{
		Label: pulumi.String("my_firewall"),
		Tags: pulumi.StringArray{
			pulumi.String("test"),
		},
		Inbounds: linode.FirewallInboundArray{
			&linode.FirewallInboundArgs{
				Label:    pulumi.String("allow-http"),
				Action:   pulumi.String("ACCEPT"),
				Protocol: pulumi.String("TCP"),
				Ports:    pulumi.String("80"),
				Ipv4s: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
				Ipv6s: pulumi.StringArray{
					pulumi.String("::/0"),
				},
			},
			&linode.FirewallInboundArgs{
				Label:    pulumi.String("allow-https"),
				Action:   pulumi.String("ACCEPT"),
				Protocol: pulumi.String("TCP"),
				Ports:    pulumi.String("443"),
				Ipv4s: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
				Ipv6s: pulumi.StringArray{
					pulumi.String("::/0"),
				},
			},
		},
		InboundPolicy: pulumi.String("DROP"),
		Outbounds: linode.FirewallOutboundArray{
			&linode.FirewallOutboundArgs{
				Label:    pulumi.String("reject-http"),
				Action:   pulumi.String("DROP"),
				Protocol: pulumi.String("TCP"),
				Ports:    pulumi.String("80"),
				Ipv4s: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
				Ipv6s: pulumi.StringArray{
					pulumi.String("::/0"),
				},
			},
			&linode.FirewallOutboundArgs{
				Label:    pulumi.String("reject-https"),
				Action:   pulumi.String("DROP"),
				Protocol: pulumi.String("TCP"),
				Ports:    pulumi.String("443"),
				Ipv4s: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
				Ipv6s: pulumi.StringArray{
					pulumi.String("::/0"),
				},
			},
		},
		OutboundPolicy: pulumi.String("ACCEPT"),
		Linodes:        linodes,
	})
	return wall, err

}
