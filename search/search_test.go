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

// Package search provides the entry point for the search command.
//
// It implements a search command that searches for resources in AWS.
// The searchs are done in parallel and the results are printed in the
// specified format.
package search

import "testing"

func TestCheckSortField(t *testing.T) {
	type args struct {
		cmd string
		f   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "InstanceID", args: args{cmd: "ec2", f: "id"}, wantErr: false},
		{name: "InstanceName", args: args{cmd: "ec2", f: "name"}, wantErr: false},
		{name: "InstanceType", args: args{cmd: "ec2", f: "type"}, wantErr: false},
		{name: "AvailabilityZone", args: args{cmd: "ec2", f: "az"}, wantErr: false},
		{name: "InstanceState", args: args{cmd: "ec2", f: "state"}, wantErr: false},
		{name: "PrivateIPAddress", args: args{cmd: "ec2", f: "private-ip"}, wantErr: false},
		{name: "PublicIPAddress", args: args{cmd: "ec2", f: "public-ip"}, wantErr: false},
		{name: "NetworkInterfaces", args: args{cmd: "ec2", f: "enis"}, wantErr: false},
		{name: "Tags", args: args{cmd: "ec2", f: "tags"}, wantErr: true},
		{name: "InvalidField", args: args{cmd: "ec2", f: "invalid"}, wantErr: true},
		{name: "InvalidCommand", args: args{cmd: "invalid", f: "id"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckSortField(tt.args.cmd, tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("CheckSortField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
