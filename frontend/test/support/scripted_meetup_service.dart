// ignore_for_file: prefer_initializing_formals
// Public param names deliberately differ from private field names — see
// token_refresher.dart's own doc comment for the same tradeoff.
import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/models/paged_result.dart';
import 'package:professional_connections_platform/core/services/meetup_service.dart';

/// A [MeetupService] whose responses are scripted per-call by the test —
/// unlike [MockMeetupService] (which simulates real server behavior with
/// latency for UX-level tests), this is for widget tests that need to
/// assert *which* method was called with *which* arguments, immediately.
class ScriptedMeetupService implements MeetupService {
  ScriptedMeetupService({
    List<Meetup> openMeetups = const [],
    ({List<Meetup> hosted, List<Meetup> requested}) myMeetups = const (
      hosted: <Meetup>[],
      requested: <Meetup>[],
    ),
    List<MeetupRequestModel> meetupRequests = const [],
    Meetup? meetupDetail,
    SafetyState? safetyState,
    List<RatableParticipant> ratableParticipants = const [],
  }) : _openMeetups = openMeetups,
       _myMeetups = myMeetups,
       _meetupRequests = meetupRequests,
       _meetupDetail = meetupDetail,
       _safetyState = safetyState,
       _ratableParticipants = ratableParticipants;

  final List<Meetup> _openMeetups;
  final ({List<Meetup> hosted, List<Meetup> requested}) _myMeetups;
  final List<MeetupRequestModel> _meetupRequests;
  final Meetup? _meetupDetail;
  final SafetyState? _safetyState;
  final List<RatableParticipant> _ratableParticipants;

  String? lastRequestToJoinMeetupId;
  String? lastRespondToRequestId;
  bool? lastRespondToRequestAccept;
  String? lastSubmitRatingMeetupId;
  String? lastSubmitRatingRatedUserId;
  int? lastSubmitRatingScore;
  String? lastSubmitFeedbackNotes;
  String? lastCloseMeetupId;
  Meetup? closeMeetupResult;
  String? lastCancelMeetupId;
  MeetupException? cancelMeetupError;

  @override
  Future<Meetup> createMeetup({
    required IntentType intent,
    required DateTime windowStart,
    required DateTime windowEnd,
    required double locationLat,
    required double locationLng,
    required String locationLabel,
    required int capacity,
  }) => throw UnimplementedError();

  @override
  Future<PagedResult<Meetup>> listOpenMeetups({
    required IntentType intent,
    String? cursor,
  }) async => PagedResult(items: _openMeetups);

  @override
  Future<Meetup> getMeetup(String meetupId) async => _meetupDetail!;

  @override
  Future<({List<Meetup> hosted, List<Meetup> requested})>
  listMyMeetups() async => _myMeetups;

  @override
  Future<List<MeetupRequestModel>> listMeetupRequests(String meetupId) async =>
      _meetupRequests;

  @override
  Future<MeetupRequestModel> requestToJoin(String meetupId) async {
    lastRequestToJoinMeetupId = meetupId;
    return MeetupRequestModel(
      id: 'request-1',
      meetupId: meetupId,
      requesterId: 'me',
      requesterFullName: 'Me',
      requesterTrustLevel: 2,
      status: MeetupRequestStatus.pending,
      createdAt: DateTime.now(),
    );
  }

  @override
  Future<void> withdrawRequest(String requestId) async {}

  @override
  Future<MeetupRequestModel> respondToRequest(
    String requestId, {
    required bool accept,
  }) async {
    lastRespondToRequestId = requestId;
    lastRespondToRequestAccept = accept;
    return MeetupRequestModel(
      id: requestId,
      meetupId: 'meetup-1',
      requesterId: 'requester-1',
      requesterFullName: 'Requester',
      requesterTrustLevel: 2,
      status: accept
          ? MeetupRequestStatus.accepted
          : MeetupRequestStatus.rejected,
      createdAt: DateTime.now(),
      resolvedAt: DateTime.now(),
    );
  }

  @override
  Future<void> registerDeviceToken(String fcmToken) async {}

  @override
  Future<SafetyState> getSafetyState(String meetupId) async {
    final state = _safetyState;
    if (state == null) {
      throw const MeetupNotFoundException('not started yet');
    }
    return state;
  }

  @override
  Future<SafetyState> acknowledgeSafetyChecklist(String meetupId) async =>
      SafetyState(meetupId: meetupId, checklistAckAt: DateTime.now());

  @override
  Future<SafetyState> setLiveLocationOptIn(String meetupId, bool optIn) async =>
      SafetyState(meetupId: meetupId, liveLocationOptIn: optIn);

  @override
  Future<SafetyState> checkIn(String meetupId) async =>
      SafetyState(meetupId: meetupId, checkedInAt: DateTime.now());

  @override
  Future<void> submitMeetupFeedback(
    String meetupId, {
    required bool happened,
    bool? feltSafe,
    bool? profileAccurate,
    bool? wouldMeetAgain,
    String? notes,
  }) async {
    lastSubmitFeedbackNotes = notes;
  }

  @override
  Future<List<RatableParticipant>> listRatableParticipants(
    String meetupId,
  ) async => _ratableParticipants;

  @override
  Future<void> submitRating(
    String meetupId, {
    required String ratedUserId,
    required int score,
  }) async {
    lastSubmitRatingMeetupId = meetupId;
    lastSubmitRatingRatedUserId = ratedUserId;
    lastSubmitRatingScore = score;
  }

  @override
  Future<Meetup> closeMeetup(String meetupId) async {
    lastCloseMeetupId = meetupId;
    final result = closeMeetupResult;
    if (result == null) {
      throw const MeetupNotFoundException('not found');
    }
    return result;
  }

  @override
  Future<void> cancelMeetup(String meetupId) async {
    lastCancelMeetupId = meetupId;
    final error = cancelMeetupError;
    if (error != null) throw error;
  }
}
