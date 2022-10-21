// SPDX-License-Identifier: Apache-2.0
// Copyright 2022-present Open Networking Foundation

package session

import (
	"github.com/wmnsk/go-pfcp/ie"
)

type urrBuilder struct {
	urrID  uint32
	method IEMethod

	// Triggers are defined at https://fburl.com/code/hk3htg8o
	triggers uint16

	// Defined at https://fburl.com/code/s1mkubht
	// values are either 0 or 1
	measurementMethodEvent    int
	measurementMethodVolume   int
	measurementMethodDuration int

	// Volume threshold flags definition can be found https://fburl.com/code/8dexz1f3
	// first bit marks the existence of total volume threshold
	// second bit marks the existence of uplink volume threshold
	// first bit marks the existence of downlink volume threshold
	volThresholdFlags       uint8
	volThresholdTotalVol    uint64
	volThresholdUplinkVol   uint64
	volThresholdDownlinkVol uint64

	// Volume quota flags definition can be found https://fburl.com/code/mgevzvb6
	// first bit marks the existence of total volume quota
	// second bit marks the existence of uplink volume quota
	// first bit marks the existence of downlink volume quota
	volQuotaFlags       uint8
	volQuotaTotalVol    uint64
	volQuotaUplinkVol   uint64
	volQuotaDownlinkVol uint64
}

// NewURRBuilder returns the pointer to a new urrBuilder instance
func NewURRBuilder() *urrBuilder {
	return &urrBuilder{}
}

func (b *urrBuilder) WithID(id uint32) *urrBuilder {
	b.urrID = id
	return b
}

func (b *urrBuilder) WithMethod(method IEMethod) *urrBuilder {
	b.method = method
	return b
}

func (b *urrBuilder) WithMeasurementMethodEvent(measurementMethodEvent int) *urrBuilder {
	b.measurementMethodEvent = measurementMethodEvent
	return b
}

func (b *urrBuilder) WithMeasurementMethodVolume(measurementMethodVolume int) *urrBuilder {
	b.measurementMethodVolume = measurementMethodVolume
	return b
}

func (b *urrBuilder) WithMeasurementMethodDuration(measurementMethodDuration int) *urrBuilder {
	b.measurementMethodDuration = measurementMethodDuration
	return b
}

func (b *urrBuilder) WithTriggers(triggers uint16) *urrBuilder {
	b.triggers = triggers
	return b
}

func (b *urrBuilder) WithVolThresholdFlags(volThresholdFlags uint8) *urrBuilder {
	b.volThresholdFlags = volThresholdFlags
	return b
}

func (b *urrBuilder) WithVolThresholdTotalVol(volThresholdTotalVol uint64) *urrBuilder {
	b.volThresholdTotalVol = volThresholdTotalVol
	return b
}

func (b *urrBuilder) WithVolThresholdUplinkVol(volThresholdUplinkVol uint64) *urrBuilder {
	b.volThresholdUplinkVol = volThresholdUplinkVol
	return b
}

func (b *urrBuilder) WithVolThresholdDownlinkVol(volThresholdDownlinkVol uint64) *urrBuilder {
	b.volThresholdDownlinkVol = volThresholdDownlinkVol
	return b
}

func (b *urrBuilder) WithVolQuotaFlags(volQuotaFlags uint8) *urrBuilder {
	b.volQuotaFlags = volQuotaFlags
	return b
}

func (b *urrBuilder) WithVolQuotaTotalVol(volQuotaTotalVol uint64) *urrBuilder {
	b.volQuotaTotalVol = volQuotaTotalVol
	return b
}
func (b *urrBuilder) WithVolQuotaUplinkVol(volQuotaUplinkVol uint64) *urrBuilder {
	b.volQuotaUplinkVol = volQuotaUplinkVol
	return b
}
func (b *urrBuilder) WithVolQuotaDownlinkVol(volQuotaDownlinkVol uint64) *urrBuilder {
	b.volQuotaDownlinkVol = volQuotaDownlinkVol
	return b
}

func (b *urrBuilder) validate() {
	if b.urrID == 0 {
		panic("Tried building URR without setting URR ID")
	}
}

func (b *urrBuilder) Build() *ie.IE {
	b.validate()

	createFunc := ie.NewCreateURR
	if b.method == Update {
		createFunc = ie.NewUpdateURR
	}

	urr := createFunc(
		ie.NewURRID(b.urrID),
	)

	urr.Add(ie.NewMeasurementMethod(b.measurementMethodEvent, b.measurementMethodVolume, b.measurementMethodDuration))

	urr.Add(ie.NewReportingTriggers(b.triggers))

	urr.Add(ie.NewVolumeThreshold(
		b.volThresholdFlags, b.volThresholdTotalVol, b.volThresholdUplinkVol, b.volThresholdDownlinkVol))

	urr.Add(ie.NewVolumeQuota(
		b.volQuotaFlags, b.volQuotaTotalVol, b.volQuotaUplinkVol, b.volQuotaDownlinkVol))

	if b.method == Delete {
		return ie.NewRemoveURR(urr)
	}

	return urr
}
