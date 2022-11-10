package commands

import (
	"fmt"
	"net"

	"github.com/dyegoe/awss/search"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ec2Ids, ec2Names, ec2Tags, ec2InstanceTypes, ec2InstanceStates, ec2AvailabilityZones []string
var ec2PrivateIps, ec2PublicIps []net.IP

var ec2Cmd = &cobra.Command{
	Use:   "ec2",
	Short: "Search for EC2 instances.",
	Long: `Search for EC2 instances.
You can search EC2 instances using the following filters: ids, names, tags, instance-types, availability-zones, instance-states, private-ips and public-ips.
You can use multiple values for each filter, separated by comma and multiple filters at same time.


For example, if you want to search for EC2 instances with:

ids i-1230456078901 and i-1230456078902, you can use:
	awss ec2 -i i-1230456078901,i-1230456078902
names instance-1 and instance-2, you can use:
	awss ec2 -n instance-1,instance-2
tag Key and the values Value1 and Value2, you can use:
	awss ec2 -t 'Key=Value1:Value2'
tag Environment and the value Production, you can use:
	awss ec2 -t 'Environment=Production'
tags Key=Value1:Value2 and Environment=Production, you can use:
	awss ec2 -t 'Key=Value1:Value2,Environment=Production'
instance type t2.micro and t2.small, you can use:
	awss ec2 -T t2.micro,t2.small
availability zones us-east-1a and us-east-1b, you can use:
	awss --region us-east-1 ec2 -z a,b
instance state running and stopped, you can use:
	awss ec2 -s running,stopped
private IPs 172.16.0.1 and 172.17.1.254, you can use:
	awss ec2 -p 172.16.0.1,172.17.1.254
public IPs 52.28.19.20 and 52.30.31.32, you can use:
	awss ec2 -P 52.28.19.20,52.30.31.32

You can use multiple filters at same time, for example:
	awss ec2 -n '*' -t 'Key=Value1:Value2,Environment=Production' -T t2.micro,t2.small -z a,b -s running,stopped
	(You can use the wildcard '*' to search for all values in a filter)
`,
	Args: cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if _, err := search.ParseTags(ec2Tags); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var searchBy = make(map[string][]string)

		if len(ec2Ids) > 0 {
			searchBy["ids"] = ec2Ids
		}
		if len(ec2Names) > 0 {
			searchBy["names"] = ec2Names
		}
		if len(ec2Tags) > 0 {
			searchBy["tags"] = ec2Tags
		}
		if len(ec2InstanceTypes) > 0 {
			searchBy["instance-types"] = ec2InstanceTypes
		}
		if len(ec2AvailabilityZones) > 0 {
			searchBy["availability-zones"] = ec2AvailabilityZones
		}
		if len(ec2InstanceStates) > 0 {
			searchBy["instance-states"] = ec2InstanceStates
		}
		if len(ec2PrivateIps) > 0 {
			searchBy["private-ips"] = ipToString(ec2PrivateIps)
		}
		if len(ec2PublicIps) > 0 {
			searchBy["public-ips"] = ipToString(ec2PublicIps)
		}

		if len(searchBy) == 0 {
			return fmt.Errorf("you must specify at least one filter")
		}

		err := search.Run(viper.GetStringSlice("profiles"), viper.GetStringSlice("regions"), viper.GetString("output"), viper.GetBool("show-empty"), cmd.Name(), searchBy)
		if err != nil {
			return fmt.Errorf("something went wrong while running %s. error: %v", cmd.Name(), err)
		}
		return nil
	},
}

func init() {
	// Set flags for ec2Cmd
	ec2Cmd.Flags().StringSliceVarP(&ec2Ids, "ids", "i", []string{}, "Filter EC2 instances by ids. `i-1230456078901,i-1230456078902`")
	ec2Cmd.Flags().StringSliceVarP(&ec2Names, "names", "n", []string{}, "Filter EC2 instances by names. It searchs using the 'tag:Name'. `instance-1,instance-2`")
	ec2Cmd.Flags().StringSliceVarP(&ec2Tags, "tags", "t", []string{}, "Filter EC2 instances by tags. `'Key=Value1:Value2,Environment=Production'`")
	ec2Cmd.Flags().StringSliceVarP(&ec2InstanceTypes, "instance-types", "T", []string{}, "Filter EC2 instances by instance type. `t2.micro,t2.small`")
	ec2Cmd.Flags().StringSliceVarP(&ec2AvailabilityZones, "availability-zones", "z", []string{}, "Filter EC2 instances by availability zones. It will append to current region. `a,b`")
	ec2Cmd.Flags().StringSliceVarP(&ec2InstanceStates, "instance-states", "s", []string{}, "Filter EC2 instances by instance state. `running,stopped`")
	ec2Cmd.Flags().IPSliceVarP(&ec2PrivateIps, "private-ips", "p", []net.IP{}, "Filter EC2 instances by private IPs. `172.16.0.1,172.17.1.254`")
	ec2Cmd.Flags().IPSliceVarP(&ec2PublicIps, "public-ips", "P", []net.IP{}, "Filter EC2 instances by public IPs. `52.28.19.20,52.30.31.32`")
}

// ipToString converts a slice of net.IP to a slice of string
func ipToString(ip []net.IP) []string {
	var ips []string
	for _, i := range ip {
		ips = append(ips, i.String())
	}
	return ips
}
