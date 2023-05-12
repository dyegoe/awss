/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmd enables the CLI commands and flags for the EC2 engine.
package cmd

import (
	"net"

	"github.com/dyegoe/awss/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// EngineName is the name of the engine.
const EngineName string = "ec2"

// // filters is the struct that holds the filter flags content.
// type filters struct {
// 	Ids               []string `filter:"instance-id"`
// 	Names             []string `filter:"tag:Name"`
// 	Tags              []string `filter:"tag"`
// 	TagsKey           []string `filter:"tag-key"`
// 	InstanceTypes     []string `filter:"instance-type"`
// 	InstanceStates    []string `filter:"instance-state-name"`
// 	AvailabilityZones []string `filter:"availability-zone"`
// 	PrivateIPs        []net.IP `filter:"network-interface.addresses.private-ip-address"`
// 	PublicIPs         []net.IP `filter:"network-interface.addresses.association.public-ip"`
// }

// Command returns the initialized ec2 command.
func Command() (*cobra.Command, error) {
	log := logger.NewLogger(logger.DefaultOutput, map[string]string{"pkg": "cmd", "cmd": "ec2"})

	c := &cobra.Command{
		Use:   EngineName,
		Short: "Search for EC2 instances.",
		Long: `You can search EC2 instances using the following filters:
  ids, names, tags, instance-types, availability-zones, instance-states, private-ips and public-ips.
You can use multiple values for each filter, separated by comma. Example: --names 'Name1,Name2'
You can use multiple filters at same time, for example:
	awss ec2 -n '*' -t 'Key=Value1:Value2,Environment=Production' -T t2.micro,t2.small -z a,b -s running,stopped
(You can use the wildcard '*' to search for all values in a filter)
`,
		RunE: func(c *cobra.Command, args []string) error {
			if err := execute(c, log); err != nil {
				return err
			}
			return nil
		},
	}

	initFlags(c)

	return c, nil
}

// initFlags initializes the flags for the ec2 command.
//
//nolint:lll
func initFlags(c *cobra.Command) {
	flags := c.Flags()
	flags.StringSliceP("ids", "i", []string{}, "Filter EC2 instances by ids. `i-1230456078901,i-1230456078902`")
	flags.StringSliceP("names", "n", []string{}, "Filter EC2 instances by names. It searches using the 'tag:Name'. `instance-1,instance-2`")
	flags.StringSliceP("tags", "t", []string{}, "Filter EC2 instances by tags. `'Key=Value1:Value2,Environment=Production'`")
	flags.StringSliceP("tags-key", "k", []string{}, "Filter EC2 instances by tags key. `Key,Environment`")
	flags.StringSliceP("instance-types", "T", []string{}, "Filter EC2 instances by instance type. `t2.micro,t2.small`")
	flags.StringSliceP("availability-zones", "z", []string{}, "Filter EC2 instances by availability zones. It will append to current region. `a,b`")
	flags.StringSliceP("instance-states", "s", []string{}, "Filter EC2 instances by instance state. `running,stopped`")
	flags.IPSliceP("private-ips", "p", []net.IP{}, "Filter EC2 instances by private IPs. `172.16.0.1,172.17.1.254`")
	flags.IPSliceP("public-ips", "P", []net.IP{}, "Filter EC2 instances by public IPs. `52.28.19.20,52.30.31.32`")
	flags.String("sort", "name", "Sort EC2 instances by id, name, type, az, state, private-ip or public-ip. `name`")
}

// execute is the function that runs the ec2 command.
func execute(c *cobra.Command, log *logger.Logger) error {
	log.AddFields(map[string]string{"func": "execute"})

	_ = c

	c.Flags().VisitAll(func(f *pflag.Flag) {
		log.Debugf("Filter: %s -> %v", f.Name, f.Value.String())
	})

	return nil
}
