/*
Copyright 2019 Cortex Labs, Inc.

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

package userconfig

import (
	"github.com/cortexlabs/cortex/pkg/lib/aws"
	cr "github.com/cortexlabs/cortex/pkg/lib/configreader"
	"github.com/cortexlabs/cortex/pkg/lib/errors"
	"github.com/cortexlabs/cortex/pkg/operator/api/resource"
)

type Constants []*Constant

type Constant struct {
	ResourceFields
	Type     OutputSchema      `json:"type" yaml:"type"`
	Value    interface{}       `json:"value" yaml:"value"`
	Tags     Tags              `json:"tags" yaml:"tags"`
	External *ExternalConstant `json:"external" yaml:"external"`
}

var constantValidation = &cr.StructValidation{
	StructFieldValidations: []*cr.StructFieldValidation{
		{
			StructField: "Name",
			StringValidation: &cr.StringValidation{
				Required:                   true,
				AlphaNumericDashUnderscore: true,
			},
		},
		{
			StructField: "Type",
			InterfaceValidation: &cr.InterfaceValidation{
				Required:  false,
				Validator: outputSchemaValidator,
			},
		},
		{
			StructField: "Value",
			InterfaceValidation: &cr.InterfaceValidation{
				Required: false,
			},
		},
		{
			StructField:      "External",
			StructValidation: externalModelFieldValidation,
		},
		tagsFieldValidation,
		typeFieldValidation,
	},
}

type ExternalConstant struct {
	Path   string `json:"path" yaml:"path"`
	Region string `json:"region" yaml:"region"`
}

var externalConstantFieldValidation = &cr.StructValidation{
	DefaultNil: true,
	StructFieldValidations: []*cr.StructFieldValidation{
		{
			StructField: "Path",
			StringValidation: &cr.StringValidation{
				Validator: cr.GetS3PathValidator(),
				Required:  true,
			},
		},
		{
			StructField: "Region",
			StringValidation: &cr.StringValidation{
				Default:       aws.DefaultS3Region,
				AllowedValues: aws.S3Regions.Slice(),
			},
		},
	},
}

func (constants Constants) Validate() error {
	for _, constant := range constants {
		if err := constant.Validate(); err != nil {
			return err
		}
	}

	resources := make([]Resource, len(constants))
	for i, res := range constants {
		resources[i] = res
	}

	dups := FindDuplicateResourceName(resources...)
	if len(dups) > 0 {
		return ErrorDuplicateResourceName(dups...)
	}

	return nil
}

func (constant *Constant) Validate() error {
	if constant.External == nil && constant.Value == nil {
		return errors.Wrap(ErrorSpecifyOnlyOneMissing(ValueKey, ExternalKey), Identify(constant))
	}

	if constant.External != nil && constant.Value != nil {
		return errors.Wrap(ErrorSpecifyOnlyOne(ValueKey, ExternalKey), Identify(constant))
	}

	if constant.External != nil {
		bucket, key, err := aws.SplitS3Path(constant.External.Path)
		if err != nil {
			return errors.Wrap(err, Identify(constant), ExternalKey, PathKey)
		}

		if ok, err := aws.IsS3FileExternal(bucket, key, constant.External.Region); err != nil || !ok {
			return errors.Wrap(ErrorExternalNotFound(constant.External.Path), Identify(constant), ExternalKey, PathKey)
		}
	}

	if constant.Type != nil {
		castedValue, err := CastOutputValue(constant.Value, constant.Type)
		if err != nil {
			return errors.Wrap(err, Identify(constant), ValueKey)
		}
		constant.Value = castedValue
	}

	return nil
}

func (constant *Constant) GetResourceType() resource.Type {
	return resource.ConstantType
}

func (constants Constants) Names() []string {
	names := make([]string, len(constants))
	for i, constant := range constants {
		names[i] = constant.Name
	}
	return names
}
