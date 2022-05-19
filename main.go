package main

import (
	"github.com/dyegoe/awss/ec2"
	"github.com/dyegoe/awss/elb"
	"github.com/dyegoe/awss/eni"
)

func main() {
	ec2.Names()
	ec2.PrivateIps()
	ec2.PublicIps()
	elb.Arns()
	elb.DnsNames()
	elb.Names()
	eni.PrivateIps()
	eni.PublicIps()
}
