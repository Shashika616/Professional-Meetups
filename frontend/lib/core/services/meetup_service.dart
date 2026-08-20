import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/models/paged_result.dart';

/// Contract for host-initiated meetup scheduling and join requests
/// (ADR-013). Same `abstract interface class` + `Mock*`-for-tests pattern
/// as [AuthService] — see `CLAUDE.md`'s "Service-contract pattern". Methods
/// map 1:1 to the backend's RPCs (backend/meetup-scheduling-PLAN.md Step C).
///
/// The client never decides trust or capacity — it only displays what the
/// server returns; every gate (trust level, capacity, ownership) is
/// re-checked server-side regardless of what this app's own UI already
/// disables.
abstract interface class MeetupService {
  Future<Meetup> createMeetup({
    required IntentType intent,
    required DateTime windowStart,
    required DateTime windowEnd,
    required double locationLat,
    required double locationLng,
    required String locationLabel,
    required int capacity,
  });

  /// cursor null means the first page.
  Future<PagedResult<Meetup>> listOpenMeetups({
    required IntentType intent,
    String? cursor,
  });

  Future<Meetup> getMeetup(String meetupId);

  /// Returns (hosted, requested) — kept separate rather than one merged
  /// list, matching the backend's ListMyMeetups response shape.
  Future<({List<Meetup> hosted, List<Meetup> requested})> listMyMeetups();

  /// The host's request-management view — every request (any status) on
  /// meetupId, with requester display info.
  Future<List<MeetupRequestModel>> listMeetupRequests(String meetupId);

  Future<MeetupRequestModel> requestToJoin(String meetupId);
  Future<void> withdrawRequest(String requestId);
  Future<MeetupRequestModel> respondToRequest(
    String requestId, {
    required bool accept,
  });

  Future<void> registerDeviceToken(String fcmToken);

  /// Throws [MeetupNotFoundException] if the meetup has no Safety Gate
  /// state yet (no accepted request) — distinct from a zero-value
  /// [SafetyState], so a caller reopening this meetup later can tell "not
  /// started" from "started, nothing acknowledged yet."
  Future<SafetyState> getSafetyState(String meetupId);
  Future<SafetyState> acknowledgeSafetyChecklist(String meetupId);
  Future<SafetyState> setLiveLocationOptIn(String meetupId, bool optIn);
  Future<SafetyState> checkIn(String meetupId);
  Future<void> submitMeetupFeedback(
    String meetupId, {
    required bool happened,
    bool? feltSafe,
    bool? profileAccurate,
    bool? wouldMeetAgain,
    String? notes,
  });

  /// The other participants (host + accepted requesters, excluding the
  /// caller) of a meetup the caller can rate — each flagged with whether
  /// the caller already rated them (ADR-015,
  /// docs/02-domain/domain-model.md § Rating).
  Future<List<RatableParticipant>> listRatableParticipants(String meetupId);

  /// Rates ratedUserId 1-5 for meetupId. Throws [MeetupForbiddenException]
  /// if the caller hasn't yet confirmed (submitMeetupFeedback,
  /// happened=true) that the meetup happened, and
  /// [MeetupConflictException] on a duplicate submission for the same
  /// pair.
  Future<void> submitRating(
    String meetupId, {
    required String ratedUserId,
    required int score,
  });

  /// Host-only "meetup is done" action (ADR-016), reviving the previously-
  /// unused `completed` status. Throws [MeetupForbiddenException] if the
  /// caller isn't the host or the window hasn't started yet, and
  /// [MeetupConflictException] if the meetup is already closed/cancelled.
  /// Independent of rating eligibility — closing has zero effect on who can
  /// rate whom (ADR-015's `meetup_feedback.happened` gate is unchanged).
  Future<Meetup> closeMeetup(String meetupId);

  Future<void> cancelMeetup(String meetupId);
}

/// Typed errors a [MeetupService] can throw — mirrors [AuthException]'s
/// shape so the UI can show a real message instead of a generic failure.
sealed class MeetupException implements Exception {
  const MeetupException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// 403 — the caller's trust level doesn't meet the intent's floor, or
/// they're not the host/requester an action requires.
class MeetupForbiddenException extends MeetupException {
  const MeetupForbiddenException(super.message);
}

/// 409 — a state conflict: meetup full/cancelled, request already
/// resolved, already requested to join, checklist not acknowledged before
/// check-in.
class MeetupConflictException extends MeetupException {
  const MeetupConflictException(super.message);
}

/// 404 — the meetup or request no longer exists.
class MeetupNotFoundException extends MeetupException {
  const MeetupNotFoundException(super.message);
}

/// 401 — same meaning as [AuthService]'s SessionExpiredException; kept as
/// a separate type in this file so `meetup_service.dart` doesn't need to
/// import `auth_service.dart` just for one exception type.
class MeetupSessionExpiredException extends MeetupException {
  const MeetupSessionExpiredException(super.message);
}

class MeetupNetworkException extends MeetupException {
  const MeetupNetworkException([
    super.message = 'Something went wrong. Please try again.',
  ]);
}

/// Simulates server latency for widget tests — kept in the codebase for
/// that purpose even though [HttpMeetupService] is what `app_providers.dart`
/// wires up for real use (mirrors MockAuthService's own doc comment).
class MockMeetupService implements MeetupService {
  static const Duration latency = Duration(milliseconds: 400);

  final List<Meetup> _meetups = [];
  final List<MeetupRequestModel> _requests = [];
  final Map<String, SafetyState> _safetyStates = {};
  int _nextId = 0;

  Meetup _mockMeetup(IntentType intent) => Meetup(
    id: 'mock-meetup-${_nextId++}',
    hostUserId: 'mock-host',
    hostFullName: 'Mock Host',
    hostTrustLevel: 2,
    intent: intent,
    windowStart: DateTime.now().add(const Duration(hours: 1)),
    windowEnd: DateTime.now().add(const Duration(hours: 3)),
    locationLat: 6.9271,
    locationLng: 79.8612,
    locationLabel: 'Mock Cafe',
    capacity: 2,
    acceptedCount: 0,
    status: MeetupStatus.open,
    createdAt: DateTime.now(),
  );

  @override
  Future<Meetup> createMeetup({
    required IntentType intent,
    required DateTime windowStart,
    required DateTime windowEnd,
    required double locationLat,
    required double locationLng,
    required String locationLabel,
    required int capacity,
  }) async {
    await Future<void>.delayed(latency);
    final meetup = Meetup(
      id: 'mock-meetup-${_nextId++}',
      hostUserId: 'mock-host',
      hostFullName: 'Mock Host',
      hostTrustLevel: 2,
      intent: intent,
      windowStart: windowStart,
      windowEnd: windowEnd,
      locationLat: locationLat,
      locationLng: locationLng,
      locationLabel: locationLabel,
      capacity: capacity,
      acceptedCount: 0,
      status: MeetupStatus.open,
      createdAt: DateTime.now(),
      isHostedByMe: true,
    );
    _meetups.add(meetup);
    return meetup;
  }

  @override
  Future<PagedResult<Meetup>> listOpenMeetups({
    required IntentType intent,
    String? cursor,
  }) async {
    await Future<void>.delayed(latency);
    if (_meetups.isEmpty) _meetups.add(_mockMeetup(intent));
    return PagedResult(
      items: _meetups.where((m) => m.intent == intent).toList(),
    );
  }

  @override
  Future<Meetup> getMeetup(String meetupId) async {
    await Future<void>.delayed(latency);
    return _meetups.firstWhere(
      (m) => m.id == meetupId,
      orElse: () => throw const MeetupNotFoundException('Meetup not found.'),
    );
  }

  @override
  Future<({List<Meetup> hosted, List<Meetup> requested})>
  listMyMeetups() async {
    await Future<void>.delayed(latency);
    return (
      hosted: _meetups.where((m) => m.isHostedByMe).toList(),
      requested: <Meetup>[],
    );
  }

  @override
  Future<List<MeetupRequestModel>> listMeetupRequests(String meetupId) async {
    await Future<void>.delayed(latency);
    return _requests.where((r) => r.meetupId == meetupId).toList();
  }

  @override
  Future<MeetupRequestModel> requestToJoin(String meetupId) async {
    await Future<void>.delayed(latency);
    final request = MeetupRequestModel(
      id: 'mock-request-${_nextId++}',
      meetupId: meetupId,
      requesterId: 'mock-requester',
      requesterFullName: 'Mock Requester',
      requesterTrustLevel: 2,
      status: MeetupRequestStatus.pending,
      createdAt: DateTime.now(),
    );
    _requests.add(request);
    return request;
  }

  @override
  Future<void> withdrawRequest(String requestId) async {
    await Future<void>.delayed(latency);
  }

  @override
  Future<MeetupRequestModel> respondToRequest(
    String requestId, {
    required bool accept,
  }) async {
    await Future<void>.delayed(latency);
    return MeetupRequestModel(
      id: requestId,
      meetupId: 'mock-meetup-0',
      requesterId: 'mock-requester',
      requesterFullName: 'Mock Requester',
      requesterTrustLevel: 2,
      status: accept
          ? MeetupRequestStatus.accepted
          : MeetupRequestStatus.rejected,
      createdAt: DateTime.now(),
      resolvedAt: DateTime.now(),
    );
  }

  @override
  Future<void> registerDeviceToken(String fcmToken) async {
    await Future<void>.delayed(latency);
  }

  @override
  Future<SafetyState> getSafetyState(String meetupId) async {
    await Future<void>.delayed(latency);
    final state = _safetyStates[meetupId];
    if (state == null) {
      throw const MeetupNotFoundException(
        'Safety Gate has not started for this meetup yet.',
      );
    }
    return state;
  }

  @override
  Future<SafetyState> acknowledgeSafetyChecklist(String meetupId) async {
    await Future<void>.delayed(latency);
    final state = SafetyState(
      meetupId: meetupId,
      checklistAckAt: DateTime.now(),
    );
    _safetyStates[meetupId] = state;
    return state;
  }

  @override
  Future<SafetyState> setLiveLocationOptIn(String meetupId, bool optIn) async {
    await Future<void>.delayed(latency);
    final current = _safetyStates[meetupId] ?? SafetyState(meetupId: meetupId);
    final state = SafetyState(
      meetupId: meetupId,
      checklistAckAt: current.checklistAckAt,
      liveLocationOptIn: optIn,
      checkedInAt: current.checkedInAt,
    );
    _safetyStates[meetupId] = state;
    return state;
  }

  @override
  Future<SafetyState> checkIn(String meetupId) async {
    await Future<void>.delayed(latency);
    final current = _safetyStates[meetupId];
    if (current == null || !current.checklistAcknowledged) {
      throw const MeetupConflictException(
        'Acknowledge the safety checklist before checking in.',
      );
    }
    final state = SafetyState(
      meetupId: meetupId,
      checklistAckAt: current.checklistAckAt,
      liveLocationOptIn: current.liveLocationOptIn,
      checkedInAt: DateTime.now(),
    );
    _safetyStates[meetupId] = state;
    return state;
  }

  @override
  Future<void> submitMeetupFeedback(
    String meetupId, {
    required bool happened,
    bool? feltSafe,
    bool? profileAccurate,
    bool? wouldMeetAgain,
    String? notes,
  }) async {
    await Future<void>.delayed(latency);
  }

  @override
  Future<List<RatableParticipant>> listRatableParticipants(
    String meetupId,
  ) async {
    await Future<void>.delayed(latency);
    return const [];
  }

  @override
  Future<void> submitRating(
    String meetupId, {
    required String ratedUserId,
    required int score,
  }) async {
    await Future<void>.delayed(latency);
  }

  @override
  Future<Meetup> closeMeetup(String meetupId) async {
    await Future<void>.delayed(latency);
    final index = _meetups.indexWhere((m) => m.id == meetupId);
    if (index == -1) {
      throw const MeetupNotFoundException('Meetup not found.');
    }
    final closed = _meetups[index].copyWith(
      status: MeetupStatus.completed,
      closedAt: DateTime.now(),
    );
    _meetups[index] = closed;
    return closed;
  }

  @override
  Future<void> cancelMeetup(String meetupId) async {
    await Future<void>.delayed(latency);
  }
}
