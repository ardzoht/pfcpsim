// SPDX-License-Identifier: Apache-2.0
// Copyright 2022-present Open Networking Foundation

package pfcpsim

import (
	"context"
	"fmt"
	"net"

	pb "github.com/ardzoht/pfcpsim/api"
	"github.com/ardzoht/pfcpsim/pkg/pfcpsim"
	"github.com/ardzoht/pfcpsim/pkg/pfcpsim/session"
	"github.com/c-robinson/iplib"
	log "github.com/sirupsen/logrus"
	ieLib "github.com/wmnsk/go-pfcp/ie"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pfcpSimService implements the Protobuf interface and keeps a connection to a remote PFCP Agent peer.
// Its state is handled in internal/pfcpsim/state.go
type pfcpSimService struct {
	pb.UnimplementedPFCPSimServer
}

// SessionStep identifies the step in loops, used while creating/modifying/deleting sessions and rules IDs.
// It should be high enough to avoid IDs overlap when creating sessions. 5 Applications should be enough.
// In theory with ROC limitations, we should expect max 8 applications (5 explicit applications + 3 filters
// to deny traffic to the RFC1918 IPs, in case we have a ALLOW-PUBLIC)
const SessionStep = 10

func NewPFCPSimService(iface string) *pfcpSimService {
	interfaceName = iface
	return &pfcpSimService{}
}

func checkServerStatus() error {
	if !isConfigured() {
		return status.Error(codes.Aborted, "Server is not configured")
	}

	if !isRemotePeerConnected() {
		return status.Error(codes.Aborted, "Server is not associated")
	}

	return nil
}

func (P pfcpSimService) Configure(ctx context.Context, request *pb.ConfigureRequest) (*pb.Response, error) {
	if net.ParseIP(request.UpfN3Address) == nil {
		errMsg := fmt.Sprintf("Error while parsing UPF N3 address: %v", request.UpfN3Address)
		log.Error(errMsg)
		return &pb.Response{}, status.Error(codes.Aborted, errMsg)
	}
	// remotePeerAddress is validated in pfcpsim
	remotePeerAddress = request.RemotePeerAddress
	upfN3Address = request.UpfN3Address

	configurationMsg := fmt.Sprintf("Server is configured. Remote peer address: %v, N3 interface address: %v ", remotePeerAddress, upfN3Address)
	log.Info(configurationMsg)

	return &pb.Response{
		StatusCode: int32(codes.OK),
		Message:    configurationMsg,
	}, nil
}

func (P pfcpSimService) Associate(ctx context.Context, empty *pb.EmptyRequest) (*pb.Response, error) {
	if !isConfigured() {
		log.Error("Server is not configured")
		return &pb.Response{}, status.Error(codes.Aborted, "Server is not configured")
	}

	if !isRemotePeerConnected() {
		if err := connectPFCPSim(); err != nil {
			errMsg := fmt.Sprintf("Could not connect to remote peer :%v", err)
			log.Error(errMsg)
			return &pb.Response{}, status.Error(codes.Aborted, errMsg)
		}
	}

	if err := sim.SetupAssociation(); err != nil {
		log.Error(err.Error())
		return &pb.Response{}, status.Error(codes.Aborted, err.Error())
	}

	infoMsg := "Association established"
	log.Info(infoMsg)

	return &pb.Response{
		StatusCode: int32(codes.OK),
		Message:    infoMsg,
	}, nil
}

func (P pfcpSimService) Disassociate(ctx context.Context, empty *pb.EmptyRequest) (*pb.Response, error) {
	if err := checkServerStatus(); err != nil {
		return &pb.Response{}, err
	}

	if err := sim.TeardownAssociation(); err != nil {
		log.Error(err.Error())
		return &pb.Response{}, status.Error(codes.Aborted, err.Error())
	}

	sim.DisconnectN4()

	remotePeerConnected = false

	infoMsg := "Association teardown completed and connection to remote peer closed"
	log.Info(infoMsg)

	return &pb.Response{
		StatusCode: int32(codes.OK),
		Message:    infoMsg,
	}, nil
}

func (P pfcpSimService) CreateSession(ctx context.Context, request *pb.CreateSessionRequest) (*pb.Response, error) {
	if err := checkServerStatus(); err != nil {
		return &pb.Response{}, err
	}

	baseID := int(request.BaseID)
	count := int(request.Count)

	uplinkDstIp := request.UlTunnelDstIP
	if uplinkDstIp == "" {
		uplinkDstIp = "0.0.0.0"
	}

	downlinkDstIp := request.DlTunnelDstIP
	if downlinkDstIp == "" {
		downlinkDstIp = request.NodeBAddress
	}

	teidAlloc := request.TeidAllocFlag

	lastUEAddr, _, err := net.ParseCIDR(request.UeAddressPool)
	if err != nil {
		errMsg := fmt.Sprintf(" Could not parse Address Pool: %v", err)
		log.Error(errMsg)
		return &pb.Response{}, status.Error(codes.Aborted, errMsg)
	}

	var qfi uint8 = 0

	if request.Qfi != 0 {
		qfi = uint8(request.Qfi)
	}

	if err = isNumOfAppFiltersCorrect(request.AppFilters); err != nil {
		return &pb.Response{}, err
	}

	for i := baseID; i < (count*SessionStep + baseID); i = i + SessionStep {
		// using variables to ease comprehension on how rules are linked together
		uplinkTEID := uint32(i)

		ueAddress := iplib.NextIP(lastUEAddr)
		lastUEAddr = ueAddress

		sessQerID := uint32(0)

		var pdrs, fars, urrs []*ieLib.IE

		qers := []*ieLib.IE{
			// session QER
			session.NewQERBuilder().
				WithID(sessQerID).
				WithMethod(session.Create).
				WithUplinkMBR(60000).
				WithDownlinkMBR(60000).
				Build(),
		}

		// create as many PDRs, FARs, App QERs and URRs as the number of app filters provided through pfcpctl
		ID := uint16(i)

		for _, appFilter := range request.AppFilters {
			SDFFilter, gateStatus, precedence, err := parseAppFilter(appFilter)
			if err != nil {
				return &pb.Response{}, status.Error(codes.Aborted, err.Error())
			}

			log.Infof("Successfully parsed application filter. SDF Filter: %v", SDFFilter)

			uplinkPdrID := ID
			downlinkPdrID := ID + 1

			uplinkFarID := uint32(ID)
			downlinkFarID := uint32(ID + 1)

			uplinkAppQerID := uint32(ID)
			downlinkAppQerID := uint32(ID + 1)

			uplinkUrrID := uint32(ID)
			downlinkUrrID := uint32(ID + 1)

			uplinkPDR := session.NewPDRBuilder().
				WithID(uplinkPdrID).
				WithMethod(session.Create).
				WithTEID(uplinkTEID).
				WithFARID(uplinkFarID).
				AddQERID(sessQerID).
				AddQERID(uplinkAppQerID).
				WithN3Address(upfN3Address).
				WithSDFFilter(SDFFilter).
				WithPrecedence(precedence).
				WithTeidAlloc(teidAlloc).
				MarkAsUplink().
				BuildPDR()

			downlinkPDR := session.NewPDRBuilder().
				WithID(downlinkPdrID).
				WithMethod(session.Create).
				WithPrecedence(precedence).
				WithUEAddress(ueAddress.String()).
				WithSDFFilter(SDFFilter).
				AddQERID(sessQerID).
				AddQERID(downlinkAppQerID).
				WithFARID(downlinkFarID).
				WithTeidAlloc(teidAlloc).
				MarkAsDownlink().
				BuildPDR()

			pdrs = append(pdrs, uplinkPDR)
			pdrs = append(pdrs, downlinkPDR)

			uplinkFAR := session.NewFARBuilder().
				WithID(uplinkFarID).
				WithAction(session.ActionForward).
				WithDstInterface(ieLib.DstInterfaceCore).
				WithMethod(session.Create).
				WithUplinkIP(uplinkDstIp).
				BuildFAR()

			downlinkFAR := session.NewFARBuilder().
				WithID(downlinkFarID).
				WithAction(session.ActionForward).
				WithMethod(session.Create).
				WithDstInterface(ieLib.DstInterfaceAccess).
				WithTEID(uplinkTEID).
				WithDownlinkIP(downlinkDstIp).
				BuildFAR()

			fars = append(fars, uplinkFAR)
			fars = append(fars, downlinkFAR)

			uplinkAppQER := session.NewQERBuilder().
				WithID(uplinkAppQerID).
				WithMethod(session.Create).
				WithQFI(qfi).
				WithUplinkMBR(50000).
				WithDownlinkMBR(30000).
				WithGateStatus(gateStatus).
				Build()

			downlinkAppQER := session.NewQERBuilder().
				WithID(downlinkAppQerID).
				WithMethod(session.Create).
				WithQFI(qfi).
				WithUplinkMBR(50000).
				WithDownlinkMBR(30000).
				WithGateStatus(gateStatus).
				Build()

			qers = append(qers, uplinkAppQER)
			qers = append(qers, downlinkAppQER)

			// TODO - for now hardcode some values
			uplinkURR := session.NewURRBuilder().
				WithID(uplinkUrrID).
				WithMethod(session.Create).
				WithMeasurementMethodEvent(0).
				WithMeasurementMethodVolume(1).
				WithMeasurementMethodDuration(1).
				WithTriggers(0x01).
				WithVolThresholdFlags(0x07).
				WithVolThresholdTotalVol(10_000_000).
				WithVolThresholdUplinkVol(5_000_000).
				WithVolThresholdDownlinkVol(5_000_000).
				WithVolQuotaFlags(0x07).
				WithVolQuotaTotalVol(50_000_000).
				WithVolQuotaUplinkVol(10_000_000).
				WithVolQuotaDownlinkVol(40_000_000).
				Build()

			downlinkURR := session.NewURRBuilder().
				WithID(downlinkUrrID).
				WithMethod(session.Create).
				WithMeasurementMethodEvent(0).
				WithMeasurementMethodVolume(1).
				WithMeasurementMethodDuration(1).
				WithTriggers(0x01).
				WithVolThresholdFlags(0x07).
				WithVolThresholdTotalVol(10_000_000).
				WithVolThresholdUplinkVol(5_000_000).
				WithVolThresholdDownlinkVol(5_000_000).
				WithVolQuotaFlags(0x07).
				WithVolQuotaTotalVol(50_000_000).
				WithVolQuotaUplinkVol(10_000_000).
				WithVolQuotaDownlinkVol(40_000_000).
				Build()

			urrs = append(urrs, uplinkURR)
			urrs = append(urrs, downlinkURR)

			ID += 2
		}

		sess, err := sim.EstablishSession(pdrs, fars, qers, urrs)
		if err != nil {
			return &pb.Response{}, status.Error(codes.Internal, err.Error())
		}
		insertSession(i, sess)
	}

	infoMsg := fmt.Sprintf("%v sessions were established using %v as baseID ", count, baseID)
	log.Info(infoMsg)

	return &pb.Response{
		StatusCode: int32(codes.OK),
		Message:    infoMsg,
	}, nil
}

func (P pfcpSimService) ModifySession(ctx context.Context, request *pb.ModifySessionRequest) (*pb.Response, error) {
	if err := checkServerStatus(); err != nil {
		return &pb.Response{}, err
	}

	// TODO add 5G mode
	baseID := int(request.BaseID)
	count := int(request.Count)
	nodeBaddress := request.NodeBAddress

	if len(activeSessions) < count {
		err := pfcpsim.NewNotEnoughSessionsError()
		log.Error(err)
		return &pb.Response{}, status.Error(codes.Aborted, err.Error())
	}

	var actions uint8 = 0

	if request.BufferFlag || request.NotifyCPFlag {
		// We currently support only both flags set
		actions |= session.ActionNotify
		actions |= session.ActionBuffer
	} else {
		// If no flag was passed, default action is Forward
		actions |= session.ActionForward
	}

	if err := isNumOfAppFiltersCorrect(request.AppFilters); err != nil {
		return &pb.Response{}, err
	}

	for i := baseID; i < (count*SessionStep + baseID); i = i + SessionStep {
		var newFARs []*ieLib.IE

		ID := uint32(i + 1)
		teid := uint32(i + 1)

		if request.BufferFlag || request.NotifyCPFlag {
			teid = 0 // When buffering, TEID = 0.
		}

		for range request.AppFilters {
			downlinkFAR := session.NewFARBuilder().
				WithID(ID). // Same FARID that was generated in create sessions
				WithMethod(session.Update).
				WithAction(actions).
				WithDstInterface(ieLib.DstInterfaceAccess).
				WithTEID(teid).
				WithDownlinkIP(nodeBaddress).
				WithEndMarker(request.EndMarkerFlag).
				BuildFAR()

			newFARs = append(newFARs, downlinkFAR)

			ID += 2
		}

		sess, ok := getSession(i)
		if !ok {
			errMsg := fmt.Sprintf("Could not retrieve session with index %v", i)
			log.Error(errMsg)
			return &pb.Response{}, status.Error(codes.Internal, errMsg)
		}

		err := sim.ModifySession(sess, nil, newFARs, nil, nil)
		if err != nil {
			return &pb.Response{}, status.Error(codes.Internal, err.Error())
		}
	}

	infoMsg := fmt.Sprintf("%v sessions were modified", count)
	log.Info(infoMsg)

	return &pb.Response{
		StatusCode: int32(codes.OK),
		Message:    infoMsg,
	}, nil
}

func (P pfcpSimService) DeleteSession(ctx context.Context, request *pb.DeleteSessionRequest) (*pb.Response, error) {
	if err := checkServerStatus(); err != nil {
		return &pb.Response{}, err
	}

	baseID := int(request.BaseID)
	count := int(request.Count)

	if len(activeSessions) < count {
		err := pfcpsim.NewNotEnoughSessionsError()
		log.Error(err)
		return &pb.Response{}, status.Error(codes.Aborted, err.Error())
	}

	for i := baseID; i < (count*SessionStep + baseID); i = i + SessionStep {
		sess, ok := getSession(i)
		if !ok {
			errMsg := "Session was nil. Check baseID"
			log.Error(errMsg)
			return &pb.Response{}, status.Error(codes.Aborted, errMsg)
		}

		err := sim.DeleteSession(sess)
		if err != nil {
			log.Error(err.Error())
			return &pb.Response{}, status.Error(codes.Aborted, err.Error())
		}
		// remove from activeSessions
		deleteSession(i)
	}

	infoMsg := fmt.Sprintf("%v sessions deleted; activeSessions: %v", count, len(activeSessions))
	log.Info(infoMsg)

	return &pb.Response{
		StatusCode: int32(codes.OK),
		Message:    infoMsg,
	}, nil
}
