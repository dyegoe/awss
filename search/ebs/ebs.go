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

// Package ebs contains the search for EBS volumes.
//
// It implements the common.Results interface.
package ebs

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/dyegoe/awss/common"
	searchEC2 "github.com/dyegoe/awss/search/ec2"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Results describes results of the EBS volumes search.
type Results struct {
	common.BaseResults

	// Data contains the volumes found.
	Data []dataRow `json:"data"`

	// Filters is a map of strings used to search.
	Filters map[string][]string `json:"-"`

	// NoInstanceName skips the instance name lookup when true.
	NoInstanceName bool `json:"-"`
}

// dataRow represents a row of the EBS volumes search results.
type dataRow struct {
	// VolumeID is the ID of the volume.
	VolumeID string `json:"id,omitempty" header:"ID" sort:"id"`

	// Size is the size of the volume in GiB.
	Size int32 `json:"size,omitempty" header:"Size (GiB)" sort:"size"`

	// VolumeType is the type of the volume.
	VolumeType string `json:"type,omitempty" header:"Type" sort:"type"`

	// State is the state of the volume.
	State string `json:"state,omitempty" header:"State" sort:"state"`

	// AvailabilityZone is the AZ of the volume.
	AvailabilityZone string `json:"az,omitempty" header:"AZ" sort:"az"`

	// Encrypted indicates whether the volume is encrypted.
	Encrypted string `json:"encrypted,omitempty" header:"Encrypted" sort:"encrypted"`

	// InstanceID is the ID of the instance the volume is attached to.
	InstanceID string `json:"instance_id,omitempty" header:"Instance ID" sort:"instance-id"`

	// InstanceName is the name of the instance the volume is attached to.
	InstanceName string `json:"instance_name,omitempty" header:"Instance Name" sort:"instance-name"`

	// Device is the device name for the attachment.
	Device string `json:"device,omitempty" header:"Device" sort:"device"`

	// Tags are the tags assigned to the volume.
	Tags map[string]string `json:"tags,omitempty" header:"Tags"`
}

// New initiates and returns a new instance of EBS results.
func New(profile, region string, filters map[string][]string, sortField string, noInstanceName bool) *Results {
	return &Results{
		BaseResults: common.BaseResults{
			Profile:   profile,
			Region:    region,
			Errors:    []string{},
			SortField: sortField,
		},
		Data:           []dataRow{},
		Filters:        filters,
		NoInstanceName: noInstanceName,
	}
}

// Search performs the EBS volumes search.
//
// Results are stored in the Data field.
func (r *Results) Search(ctx context.Context) {
	input, err := r.getFilters()
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error building filters: %v", err))
		return
	}

	cfg, err := common.AwsConfig(r.Profile, r.Region)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("error getting aws config: %s", err))
		return
	}

	client := ec2.NewFromConfig(cfg)
	paginator := ec2.NewDescribeVolumesPaginator(client, input)

	instanceIDSet := make(map[string]struct{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("error describing volumes: %v", err))
			return
		}
		for _, vol := range page.Volumes { //nolint:gocritic
			rows := parseVolume(&vol)
			for i := range rows {
				r.Data = append(r.Data, rows[i])
				if rows[i].InstanceID != "" {
					instanceIDSet[rows[i].InstanceID] = struct{}{}
				}
			}
		}
	}

	if len(instanceIDSet) > 0 && !r.NoInstanceName {
		instanceIDs := make([]string, 0, len(instanceIDSet))
		for id := range instanceIDSet {
			instanceIDs = append(instanceIDs, id)
		}
		names, err := searchEC2.SearchInstanceNames(r.Profile, r.Region, instanceIDs)
		if err != nil {
			r.Errors = append(r.Errors, err.Error())
		} else {
			for i := range r.Data {
				if id := r.Data[i].InstanceID; id != "" {
					r.Data[i].InstanceName = names[id]
				}
			}
		}
	}

	if r.SortField != "" {
		if err := r.sortResults(r.SortField); err != nil {
			r.Errors = append(r.Errors, err.Error())
		}
	}
}

// parseVolume converts a Volume into one dataRow per attachment.
//
// Volumes with no attachments produce a single row with empty InstanceID and Device.
// Multi-Attach volumes (io1/io2) produce one row per attachment so that each row
// contains a single, sortable InstanceID and Device value.
func parseVolume(vol *types.Volume) []dataRow {
	base := dataRow{
		VolumeID:         common.StringValue(vol.VolumeId),
		VolumeType:       string(vol.VolumeType),
		State:            string(vol.State),
		AvailabilityZone: common.StringValue(vol.AvailabilityZone),
		Tags:             common.TagsToMap(vol.Tags),
	}
	if vol.Size != nil {
		base.Size = *vol.Size
	}
	if vol.Encrypted != nil {
		base.Encrypted = strconv.FormatBool(*vol.Encrypted)
	}
	if len(vol.Attachments) == 0 {
		return []dataRow{base}
	}
	rows := make([]dataRow, 0, len(vol.Attachments))
	for _, att := range vol.Attachments {
		row := base
		row.InstanceID = common.StringValue(att.InstanceId)
		row.Device = common.StringValue(att.Device)
		rows = append(rows, row)
	}
	return rows
}

// Len returns the length of the results.
func (r *Results) Len() int { return len(r.Data) }

// GetHeaders returns the tag `header` of the struct fields.
func (r *Results) GetHeaders() []interface{} {
	headers := []interface{}{}

	v := reflect.ValueOf(dataRow{})
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		if header, ok := field.Tag.Lookup("header"); ok {
			headers = append(headers, header)
		}
	}

	return headers
}

// GetRows iterates results.Data and returns the results as a slice of interface{}.
func (r *Results) GetRows() []interface{} {
	rows := []interface{}{}

	for _, row := range r.Data { //nolint:gocritic
		rows = append(rows, row)
	}
	return rows
}

// getFilters returns the filters used to search.
//
// The filters are defined in the results.Filters field.
// Except for "volume-id", "tag" and "availability-zone", all other filters are passed as-is.
func (r *Results) getFilters() (*ec2.DescribeVolumesInput, error) {
	input := ec2.DescribeVolumesInput{}

	for key, values := range r.Filters {
		switch key {
		case "volume-id":
			input.VolumeIds = values
		case "tag":
			tagFilters, err := common.FilterTags(values)
			if err != nil {
				return nil, fmt.Errorf("building tag filters: %w", err)
			}
			input.Filters = append(input.Filters, tagFilters...)
		case "availability-zone":
			input.Filters = append(input.Filters, common.FilterAvailabilityZones(values, r.Region)...)
		default:
			input.Filters = append(input.Filters, common.FilterDefault(key, values)...)
		}
	}
	return &input, nil
}

// sortResults sorts the results by the given field.
func (r *Results) sortResults(field string) error {
	sortFields, err := GetSortFields(field)
	if err != nil {
		return err
	}

	fieldName := sortFields[field]
	sort.Slice(r.Data, func(p, q int) bool {
		pField := reflect.ValueOf(r.Data[p]).FieldByName(fieldName)
		qField := reflect.ValueOf(r.Data[q]).FieldByName(fieldName)
		if pField.Kind() == reflect.Int32 {
			return pField.Int() < qField.Int()
		}
		return pField.String() < qField.String()
	})
	return nil
}

// GetSortFields returns a map of the sort fields and their corresponding struct field.
//
// The sort fields are defined in the struct tag `sort` on dataRow.
// The function returns an error if the given field is not a valid sort field.
func GetSortFields(f string) (map[string]string, error) {
	sortFields := map[string]string{}

	v := reflect.ValueOf(dataRow{})
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)

		if s, ok := field.Tag.Lookup("sort"); ok {
			sortFields[s] = field.Name
		}
	}

	if _, ok := sortFields[f]; !ok {
		options := make([]string, 0, len(sortFields))
		for k := range sortFields {
			options = append(options, k)
		}
		sort.Strings(options)
		return nil, fmt.Errorf("invalid sort field: %s. The options are: %s", f, common.StringSliceToString(options, ", "))
	}
	return sortFields, nil
}
