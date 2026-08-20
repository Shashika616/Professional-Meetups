import 'package:flutter/foundation.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';

/// Mirrors the backend's meetup_status Postgres enum exactly
/// (db/migrations/0003_meetups.up.sql) — no client-side states that don't
/// exist server-side (frontend/meetup-scheduling-PLAN.md Step 3).
enum MeetupStatus {
  open,
  full,
  cancelled,
  completed;

  static MeetupStatus fromWire(String value) => switch (value) {
    'open' => MeetupStatus.open,
    'full' => MeetupStatus.full,
    'cancelled' => MeetupStatus.cancelled,
    'completed' => MeetupStatus.completed,
    _ => throw FormatException('Unknown meetup status: $value'),
  };
}

/// Mirrors the backend's meetup_request_status Postgres enum exactly.
enum MeetupRequestStatus {
  pending,
  accepted,
  rejected,
  withdrawn;

  static MeetupRequestStatus fromWire(String value) => switch (value) {
    'pending' => MeetupRequestStatus.pending,
    'accepted' => MeetupRequestStatus.accepted,
    'rejected' => MeetupRequestStatus.rejected,
    'withdrawn' => MeetupRequestStatus.withdrawn,
    _ => throw FormatException('Unknown meetup request status: $value'),
  };
}

/// A scheduled meetup — host-initiated, with a hard participant cap
/// (ADR-013). `windowStart`/`windowEnd` replace the old nullable
/// `scheduledFor` (ADR-016): every meetup, "today" included, now requires a
/// real time range — no more silent no-time-entered case.
@immutable
class Meetup {
  const Meetup({
    required this.id,
    required this.hostUserId,
    required this.hostFullName,
    this.hostProfilePhotoUrl = '',
    required this.hostTrustLevel,
    this.hostRatingAverage = 0,
    this.hostRatingCount = 0,
    required this.intent,
    required this.windowStart,
    required this.windowEnd,
    required this.locationLat,
    required this.locationLng,
    required this.locationLabel,
    required this.capacity,
    required this.acceptedCount,
    required this.status,
    required this.createdAt,
    this.cancelledAt,
    this.closedAt,
    this.isHostedByMe = false,
    this.myRequestStatus,
    this.myRequestAutoRejected = false,
  });

  final String id;
  final String hostUserId;
  final String hostFullName;
  final String hostProfilePhotoUrl;
  final int hostTrustLevel;

  /// Post-meetup star rating aggregate (ADR-015,
  /// docs/02-domain/domain-model.md § Rating) — 0/0 for a host who's never
  /// been rated, same "no data yet" convention as an unrated
  /// [MeetupRequestModel.requesterRatingCount].
  final double hostRatingAverage;
  final int hostRatingCount;

  final IntentType intent;
  final DateTime windowStart;
  final DateTime windowEnd;
  final double locationLat;
  final double locationLng;
  final String locationLabel;
  final int capacity;
  final int acceptedCount;
  final MeetupStatus status;
  final DateTime createdAt;
  final DateTime? cancelledAt;

  /// Set once the host calls [MeetupService.closeMeetup] (ADR-016) —
  /// independent of rating eligibility, which stays gated on each
  /// participant's own confirmed-attendance feedback (ADR-015, unchanged).
  final DateTime? closedAt;

  final bool isHostedByMe;
  final MeetupRequestStatus? myRequestStatus;

  /// Only meaningful when [myRequestStatus] is [MeetupRequestStatus.rejected]
  /// — distinguishes the host's explicit rejection from a system auto-reject
  /// (capacity filled before the host acted). Only populated on the "My
  /// Meetups" requested list, per the backend's own doc comment on this
  /// field.
  final bool myRequestAutoRejected;

  /// "Today, 3:00–5:00 PM" / "Aug 22, 6:00–8:00 PM" style display (ADR-016)
  /// — shown on every meetup card, not just stored. See
  /// [formatMeetupWindow] for the shared formatting logic (also used by the
  /// schedule flow's Review step, which has a draft window but not yet a
  /// full [Meetup]).
  String get formattedWindow => formatMeetupWindow(windowStart, windowEnd);

  Meetup copyWith({
    MeetupStatus? status,
    DateTime? closedAt,
    DateTime? cancelledAt,
  }) {
    return Meetup(
      id: id,
      hostUserId: hostUserId,
      hostFullName: hostFullName,
      hostProfilePhotoUrl: hostProfilePhotoUrl,
      hostTrustLevel: hostTrustLevel,
      hostRatingAverage: hostRatingAverage,
      hostRatingCount: hostRatingCount,
      intent: intent,
      windowStart: windowStart,
      windowEnd: windowEnd,
      locationLat: locationLat,
      locationLng: locationLng,
      locationLabel: locationLabel,
      capacity: capacity,
      acceptedCount: acceptedCount,
      status: status ?? this.status,
      createdAt: createdAt,
      cancelledAt: cancelledAt ?? this.cancelledAt,
      closedAt: closedAt ?? this.closedAt,
      isHostedByMe: isHostedByMe,
      myRequestStatus: myRequestStatus,
      myRequestAutoRejected: myRequestAutoRejected,
    );
  }

  factory Meetup.fromJson(Map<String, dynamic> json) {
    return Meetup(
      id: json['id'] as String,
      hostUserId: json['host_user_id'] as String,
      hostFullName: json['host_full_name'] as String? ?? '',
      hostProfilePhotoUrl: json['host_profile_photo_url'] as String? ?? '',
      hostTrustLevel: json['host_trust_level'] as int? ?? 0,
      hostRatingAverage: (json['host_rating_average'] as num?)?.toDouble() ?? 0,
      hostRatingCount: json['host_rating_count'] as int? ?? 0,
      intent: IntentType.fromWire(json['intent'] as String),
      windowStart:
          _secondsToDateTime(json['window_start_unix_seconds']) ??
          DateTime.now(),
      windowEnd:
          _secondsToDateTime(json['window_end_unix_seconds']) ?? DateTime.now(),
      locationLat: (json['location_lat'] as num).toDouble(),
      locationLng: (json['location_lng'] as num).toDouble(),
      locationLabel: json['location_label'] as String? ?? '',
      capacity: json['capacity'] as int,
      acceptedCount: json['accepted_count'] as int? ?? 0,
      status: MeetupStatus.fromWire(json['status'] as String),
      createdAt:
          _secondsToDateTime(json['created_at_unix_seconds']) ?? DateTime.now(),
      cancelledAt: _secondsToDateTime(json['cancelled_at_unix_seconds']),
      closedAt: _secondsToDateTime(json['closed_at_unix_seconds']),
      isHostedByMe: json['is_hosted_by_me'] as bool? ?? false,
      myRequestStatus: json['my_request_status'] != null
          ? MeetupRequestStatus.fromWire(json['my_request_status'] as String)
          : null,
      myRequestAutoRejected: json['my_request_auto_rejected'] as bool? ?? false,
    );
  }
}

const _monthAbbr = [
  'Jan',
  'Feb',
  'Mar',
  'Apr',
  'May',
  'Jun',
  'Jul',
  'Aug',
  'Sep',
  'Oct',
  'Nov',
  'Dec',
];

/// "Today, 3:00–5:00 PM" / "Aug 22, 6:00–8:00 PM" style display (ADR-016) —
/// a top-level function (not just [Meetup.formattedWindow]) so the
/// schedule flow's Review step can format a draft window before a full
/// [Meetup] exists yet. The date shown is [start]'s; a window is allowed to
/// cross midnight (e.g. "10:00 PM–1:00 AM" is valid, nothing in ADR-016
/// requires same-day), in which case both times carry their own AM/PM
/// suffix — otherwise the suffix is shown once, at the end.
String formatMeetupWindow(DateTime start, DateTime end) {
  final now = DateTime.now();
  final isToday =
      start.year == now.year &&
      start.month == now.month &&
      start.day == now.day;
  final datePart = isToday
      ? 'Today'
      : '${_monthAbbr[start.month - 1]} ${start.day}';
  final startIsPM = start.hour >= 12;
  final endIsPM = end.hour >= 12;
  final rangePart = startIsPM == endIsPM
      ? '${_formatTime(start, showPeriod: false)}–${_formatTime(end)}'
      : '${_formatTime(start)}–${_formatTime(end)}';
  return '$datePart, $rangePart';
}

/// Formats [time] as "3:00 PM" (or "3:00" with [showPeriod] false, for the
/// start of a same-AM/PM-period range where the end already carries the
/// suffix).
String _formatTime(DateTime time, {bool showPeriod = true}) {
  final hour24 = time.hour;
  final hour12 = hour24 % 12 == 0 ? 12 : hour24 % 12;
  final minute = time.minute.toString().padLeft(2, '0');
  if (!showPeriod) return '$hour12:$minute';
  final period = hour24 >= 12 ? 'PM' : 'AM';
  return '$hour12:$minute $period';
}

/// Another user's request to join a [Meetup].
@immutable
class MeetupRequestModel {
  const MeetupRequestModel({
    required this.id,
    required this.meetupId,
    required this.requesterId,
    required this.requesterFullName,
    this.requesterProfilePhotoUrl = '',
    required this.requesterTrustLevel,
    this.requesterRatingAverage = 0,
    this.requesterRatingCount = 0,
    required this.status,
    this.autoRejected = false,
    required this.createdAt,
    this.resolvedAt,
  });

  final String id;
  final String meetupId;
  final String requesterId;
  final String requesterFullName;
  final String requesterProfilePhotoUrl;
  final int requesterTrustLevel;

  /// Post-meetup star rating aggregate (ADR-015) — 0/0 for a requester
  /// who's never been rated.
  final double requesterRatingAverage;
  final int requesterRatingCount;

  final MeetupRequestStatus status;
  final bool autoRejected;
  final DateTime createdAt;
  final DateTime? resolvedAt;

  factory MeetupRequestModel.fromJson(Map<String, dynamic> json) {
    return MeetupRequestModel(
      id: json['id'] as String,
      meetupId: json['meetup_id'] as String,
      requesterId: json['requester_id'] as String,
      requesterFullName: json['requester_full_name'] as String? ?? '',
      requesterProfilePhotoUrl:
          json['requester_profile_photo_url'] as String? ?? '',
      requesterTrustLevel: json['requester_trust_level'] as int? ?? 0,
      requesterRatingAverage:
          (json['requester_rating_average'] as num?)?.toDouble() ?? 0,
      requesterRatingCount: json['requester_rating_count'] as int? ?? 0,
      status: MeetupRequestStatus.fromWire(json['status'] as String),
      autoRejected: json['auto_rejected'] as bool? ?? false,
      createdAt:
          _secondsToDateTime(json['created_at_unix_seconds']) ?? DateTime.now(),
      resolvedAt: _secondsToDateTime(json['resolved_at_unix_seconds']),
    );
  }
}

/// A meetup's Safety Gate progress (ADR-013 § 3, Safety UX Flows.md).
@immutable
class SafetyState {
  const SafetyState({
    required this.meetupId,
    this.checklistAckAt,
    this.liveLocationOptIn = false,
    this.checkedInAt,
  });

  final String meetupId;
  final DateTime? checklistAckAt;
  final bool liveLocationOptIn;
  final DateTime? checkedInAt;

  bool get checklistAcknowledged => checklistAckAt != null;
  bool get checkedIn => checkedInAt != null;

  factory SafetyState.fromJson(Map<String, dynamic> json) {
    return SafetyState(
      meetupId: json['meetup_id'] as String,
      checklistAckAt: _secondsToDateTime(json['checklist_ack_at_unix_seconds']),
      liveLocationOptIn: json['live_location_opt_in'] as bool? ?? false,
      checkedInAt: _secondsToDateTime(json['checked_in_at_unix_seconds']),
    );
  }
}

/// Another participant of a meetup the viewer can (or already did) rate
/// (ADR-015, docs/02-domain/domain-model.md § Rating) — returned by
/// [MeetupService.listRatableParticipants].
@immutable
class RatableParticipant {
  const RatableParticipant({
    required this.userId,
    required this.fullName,
    this.profilePhotoUrl = '',
    required this.trustLevel,
    this.alreadyRated = false,
  });

  final String userId;
  final String fullName;
  final String profilePhotoUrl;
  final int trustLevel;
  final bool alreadyRated;

  factory RatableParticipant.fromJson(Map<String, dynamic> json) {
    return RatableParticipant(
      userId: json['user_id'] as String,
      fullName: json['full_name'] as String? ?? '',
      profilePhotoUrl: json['profile_photo_url'] as String? ?? '',
      trustLevel: json['trust_level'] as int? ?? 0,
      alreadyRated: json['already_rated'] as bool? ?? false,
    );
  }
}

DateTime? _secondsToDateTime(Object? value) {
  if (value == null) return null;
  return DateTime.fromMillisecondsSinceEpoch((value as int) * 1000);
}
