// SPDX-License-Identifier: Apache-2.0
// Copyright 2022-present Open Networking Foundation

package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wmnsk/go-pfcp/ie"
)

func TestURRBuilderShouldPanic(t *testing.T) {
	type testCase struct {
		input       *urrBuilder
		expected    *urrBuilder
		description string
	}

	for _, scenario := range []testCase{
		{
			input: NewURRBuilder().
				WithMethod(Create),
			expected: &urrBuilder{
				method: Create,
			},
			description: "Invalid URR: No ID provided",
		},
	} {
		t.Run(scenario.description, func(t *testing.T) {
			assert.Panics(t, func() { scenario.input.Build() })
			assert.Equal(t, scenario.input, scenario.expected)
		})
	}
}

func TestURRBuilder(t *testing.T) {
	type testCase struct {
		input       *urrBuilder
		expected    *ie.IE
		description string
	}

	for _, scenario := range []testCase{
		{
			input: NewURRBuilder().
				WithID(1).
				WithMethod(Create).
				WithMeasurementMethodVolume(1).
				WithVolThresholdFlags(7).
				WithVolThresholdTotalVol(1000).
				WithVolThresholdUplinkVol(200).
				WithVolThresholdDownlinkVol(800).
				WithVolQuotaFlags(3).
				WithVolQuotaTotalVol(700).
				WithVolQuotaUplinkVol(300).
				WithVolQuotaDownlinkVol(400).
				WithTriggers(2),
			expected: ie.NewCreateURR(
				ie.NewURRID(1),
				ie.NewMeasurementMethod(0, 1, 0),
				ie.NewReportingTriggers(2),
				ie.NewVolumeThreshold(7, 1000, 200, 800),
				ie.NewVolumeQuota(3, 700, 300, 400),
			),
			description: "Valid Create URR",
		},
		{
			input: NewURRBuilder().
				WithID(1).
				WithMethod(Update).
				WithMeasurementMethodDuration(1).
				WithVolThresholdFlags(7).
				WithVolThresholdTotalVol(1000).
				WithVolThresholdUplinkVol(200).
				WithVolThresholdDownlinkVol(800).
				WithVolQuotaFlags(3).
				WithVolQuotaTotalVol(700).
				WithVolQuotaUplinkVol(300).
				WithVolQuotaDownlinkVol(400).
				WithTriggers(2),
			expected: ie.NewUpdateURR(
				ie.NewURRID(1),
				ie.NewMeasurementMethod(0, 0, 1),
				ie.NewReportingTriggers(2),
				ie.NewVolumeThreshold(7, 1000, 200, 800),
				ie.NewVolumeQuota(3, 700, 300, 400),
			),
			description: "Valid Update URR",
		},
		{
			input: NewURRBuilder().
				WithID(1).
				WithMethod(Delete),
			expected: ie.NewRemoveURR(
				ie.NewCreateURR(
					ie.NewURRID(1),
					ie.NewMeasurementMethod(0, 0, 0),
					ie.NewReportingTriggers(0),
					ie.NewVolumeThreshold(0, 0, 0, 0),
					ie.NewVolumeQuota(0, 0, 0, 0),
				),
			),
			description: "Valid Delete URR",
		},
	} {
		t.Run(scenario.description, func(t *testing.T) {
			assert.NotPanics(t, func() { _ = scenario.input.Build() })
			assert.Equal(t, scenario.input.Build(), scenario.expected)
		})
	}
}
