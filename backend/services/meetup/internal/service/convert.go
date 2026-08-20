package service

import (
	"time"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

var intentToProto = map[repository.Intent]meetupv1.Intent{
	repository.IntentCoffee:     meetupv1.Intent_INTENT_COFFEE,
	repository.IntentLunch:      meetupv1.Intent_INTENT_LUNCH,
	repository.IntentNetworking: meetupv1.Intent_INTENT_NETWORKING,
	repository.IntentMentorship: meetupv1.Intent_INTENT_MENTORSHIP,
	repository.IntentRideShare:  meetupv1.Intent_INTENT_RIDE_SHARE,
	repository.IntentDating:     meetupv1.Intent_INTENT_DATING,
}

var intentFromProto = map[meetupv1.Intent]repository.Intent{
	meetupv1.Intent_INTENT_COFFEE:     repository.IntentCoffee,
	meetupv1.Intent_INTENT_LUNCH:      repository.IntentLunch,
	meetupv1.Intent_INTENT_NETWORKING: repository.IntentNetworking,
	meetupv1.Intent_INTENT_MENTORSHIP: repository.IntentMentorship,
	meetupv1.Intent_INTENT_RIDE_SHARE: repository.IntentRideShare,
	meetupv1.Intent_INTENT_DATING:     repository.IntentDating,
}

var meetupStatusToProto = map[repository.MeetupStatus]meetupv1.MeetupStatus{
	repository.MeetupStatusOpen:      meetupv1.MeetupStatus_MEETUP_STATUS_OPEN,
	repository.MeetupStatusFull:      meetupv1.MeetupStatus_MEETUP_STATUS_FULL,
	repository.MeetupStatusCancelled: meetupv1.MeetupStatus_MEETUP_STATUS_CANCELLED,
	repository.MeetupStatusCompleted: meetupv1.MeetupStatus_MEETUP_STATUS_COMPLETED,
}

var requestStatusToProto = map[repository.MeetupRequestStatus]meetupv1.MeetupRequestStatus{
	repository.RequestStatusPending:   meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_PENDING,
	repository.RequestStatusAccepted:  meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_ACCEPTED,
	repository.RequestStatusRejected:  meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_REJECTED,
	repository.RequestStatusWithdrawn: meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_WITHDRAWN,
}

// meetupToProto converts a repository.Meetup to the wire response,
// computing IsHostedByMe relative to viewerID — never trusted from the
// client, always derived server-side from the caller's own verified id.
func meetupToProto(m repository.Meetup, viewerID string) *meetupv1.MeetupResponse {
	resp := &meetupv1.MeetupResponse{
		Id:                     m.ID,
		HostUserId:             m.HostUserID,
		HostFullName:           m.HostFullName,
		HostProfilePhotoUrl:    m.HostProfilePhotoURL,
		HostTrustLevel:         int32(m.HostTrustLevel),
		HostRatingAverage:      m.HostRatingAverage,
		HostRatingCount:        int32(m.HostRatingCount),
		Intent:                 intentToProto[m.Intent],
		LocationLat:            m.LocationLat,
		LocationLng:            m.LocationLng,
		LocationLabel:          m.LocationLabel,
		Capacity:               int32(m.Capacity),
		AcceptedCount:          int32(m.AcceptedCount),
		Status:                 meetupStatusToProto[m.Status],
		CreatedAtUnixSeconds:   m.CreatedAt.Unix(),
		IsHostedByMe:           m.HostUserID == viewerID,
		WindowStartUnixSeconds: m.WindowStart.Unix(),
		WindowEndUnixSeconds:   m.WindowEnd.Unix(),
	}
	if m.CancelledAt != nil {
		seconds := m.CancelledAt.Unix()
		resp.CancelledAtUnixSeconds = &seconds
	}
	if m.ClosedAt != nil {
		seconds := m.ClosedAt.Unix()
		resp.ClosedAtUnixSeconds = &seconds
	}
	if m.MyRequestStatus != nil {
		status := requestStatusToProto[*m.MyRequestStatus]
		resp.MyRequestStatus = &status
		resp.MyRequestAutoRejected = m.MyRequestAutoRejected
	}
	return resp
}

func meetupsToProto(meetups []repository.Meetup, viewerID string) []*meetupv1.MeetupResponse {
	out := make([]*meetupv1.MeetupResponse, 0, len(meetups))
	for _, m := range meetups {
		out = append(out, meetupToProto(m, viewerID))
	}
	return out
}

func requestToProto(r repository.MeetupRequest) *meetupv1.MeetupRequestResponse {
	resp := &meetupv1.MeetupRequestResponse{
		Id:                       r.ID,
		MeetupId:                 r.MeetupID,
		RequesterId:              r.RequesterID,
		RequesterFullName:        r.RequesterFullName,
		RequesterProfilePhotoUrl: r.RequesterProfilePhotoURL,
		RequesterTrustLevel:      int32(r.RequesterTrustLevel),
		RequesterRatingAverage:   r.RequesterRatingAverage,
		RequesterRatingCount:     int32(r.RequesterRatingCount),
		Status:                   requestStatusToProto[r.Status],
		AutoRejected:             r.AutoRejected,
		CreatedAtUnixSeconds:     r.CreatedAt.Unix(),
	}
	if r.ResolvedAt != nil {
		seconds := r.ResolvedAt.Unix()
		resp.ResolvedAtUnixSeconds = &seconds
	}
	return resp
}

func requestsToProto(requests []repository.MeetupRequest) []*meetupv1.MeetupRequestResponse {
	out := make([]*meetupv1.MeetupRequestResponse, 0, len(requests))
	for _, r := range requests {
		out = append(out, requestToProto(r))
	}
	return out
}

func safetyStateToProto(s repository.SafetyState) *meetupv1.SafetyStateResponse {
	resp := &meetupv1.SafetyStateResponse{
		MeetupId:          s.MeetupID,
		LiveLocationOptIn: s.LiveLocationOptIn,
	}
	if s.ChecklistAckAt != nil {
		seconds := s.ChecklistAckAt.Unix()
		resp.ChecklistAckAtUnixSeconds = &seconds
	}
	if s.CheckedInAt != nil {
		seconds := s.CheckedInAt.Unix()
		resp.CheckedInAtUnixSeconds = &seconds
	}
	return resp
}

// timeFromUnixSeconds converts a wire-format Unix timestamp to time.Time.
// window_start/window_end are required now (ADR-016), unlike the old
// optional scheduled_for_unix_seconds this replaces — no nil case needed.
func timeFromUnixSeconds(unixSeconds int64) time.Time {
	return time.Unix(unixSeconds, 0).UTC()
}
