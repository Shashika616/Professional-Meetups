import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/meetup_service.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/core/widgets/meetup_status_badge.dart';
import 'package:professional_connections_platform/features/matches/matches_page.dart';

import 'support/scripted_meetup_service.dart';

/// Resolves immediately to a fixed [AuthSessionState] instead of reading
/// secure storage — same pattern as HomePage's/ScheduleFlowPage's own test
/// files (each defines this locally rather than sharing one, per this
/// suite's existing convention).
class _FakeAuthSessionNotifier extends AuthSessionNotifier {
  _FakeAuthSessionNotifier(this._state);

  final AuthSessionState _state;

  @override
  Future<AuthSessionState> build() async => _state;
}

Meetup _meetup({
  String id = 'meetup-1',
  int acceptedCount = 0,
  int capacity = 4,
  bool isHostedByMe = false,
  MeetupRequestStatus? myRequestStatus,
  MeetupStatus status = MeetupStatus.open,
}) => Meetup(
  id: id,
  hostUserId: 'host-1',
  hostFullName: 'Grace Hopper',
  hostTrustLevel: 3,
  intent: IntentType.coffee,
  windowStart: DateTime.now().add(const Duration(hours: 1)),
  windowEnd: DateTime.now().add(const Duration(hours: 3)),
  locationLat: 6.9271,
  locationLng: 79.8612,
  locationLabel: 'Colombo Fort Cafe',
  capacity: capacity,
  acceptedCount: acceptedCount,
  status: status,
  createdAt: DateTime.now(),
  isHostedByMe: isHostedByMe,
  myRequestStatus: myRequestStatus,
);

/// Wraps [MatchesPage] with the given meetup service and an
/// authSessionProvider fixed at [trustLevel] — coffee (this file's default
/// test intent) requires trust level 2, so most tests pass `trustLevel: 2`
/// to exercise the unlocked path; the dedicated Level 0 group below passes
/// 0 deliberately (ADR-014's Level 0 read-only audit, Step 6).
Widget _appWith(MeetupService service, {required int trustLevel}) {
  return ProviderScope(
    overrides: [
      meetupServiceProvider.overrideWithValue(service),
      authSessionProvider.overrideWith(
        () => _FakeAuthSessionNotifier(
          AuthSessionState(
            profile: UserProfile(
              id: 'user-1',
              fullName: 'Grace',
              trustLevel: trustLevel,
            ),
          ),
        ),
      ),
    ],
    child: const MaterialApp(home: MatchesPage()),
  );
}

void main() {
  testWidgets('renders real Meetup data from openMeetupsProvider', (
    tester,
  ) async {
    final service = ScriptedMeetupService(
      openMeetups: [_meetup(acceptedCount: 1, capacity: 4)],
    );

    await tester.pumpWidget(_appWith(service, trustLevel: 2));
    await tester.pumpAndSettle();

    expect(find.text('Grace Hopper'), findsOneWidget);
    expect(find.text('Colombo Fort Cafe'), findsOneWidget);
    expect(find.text('1/4 JOINED'), findsOneWidget);
    expect(find.text('REQUEST TO JOIN'), findsOneWidget);
  });

  testWidgets(
    'tapping REQUEST TO JOIN calls requestToJoin with the meetup id',
    (tester) async {
      final service = ScriptedMeetupService(
        openMeetups: [_meetup(id: 'meetup-42')],
      );

      await tester.pumpWidget(_appWith(service, trustLevel: 2));
      await tester.pumpAndSettle();

      await tester.tap(find.text('REQUEST TO JOIN'));
      await tester.pumpAndSettle();

      expect(service.lastRequestToJoinMeetupId, 'meetup-42');
      expect(find.text('Request sent.'), findsOneWidget);
    },
  );

  testWidgets('a full meetup shows FULL instead of an enabled request button', (
    tester,
  ) async {
    final service = ScriptedMeetupService(
      openMeetups: [_meetup(acceptedCount: 4, capacity: 4)],
    );

    await tester.pumpWidget(_appWith(service, trustLevel: 2));
    await tester.pumpAndSettle();

    expect(find.text('FULL'), findsOneWidget);
    expect(find.text('REQUEST TO JOIN'), findsNothing);
  });

  testWidgets('the browse card shows the meetup\'s own lifecycle status badge, '
      'additively alongside the JOINED count (ADR-016 addendum, 2026-08-20)', (
    tester,
  ) async {
    final service = ScriptedMeetupService(
      openMeetups: [_meetup(status: MeetupStatus.full)],
    );

    await tester.pumpWidget(_appWith(service, trustLevel: 2));
    await tester.pumpAndSettle();

    expect(
      tester.widget<MeetupStatusBadge>(find.byType(MeetupStatusBadge)).status,
      MeetupStatus.full,
    );
  });

  testWidgets('an empty list shows the empty-state message, not a spinner', (
    tester,
  ) async {
    final service = ScriptedMeetupService(openMeetups: const []);

    await tester.pumpWidget(_appWith(service, trustLevel: 2));
    await tester.pumpAndSettle();

    expect(find.text('No open meetups for this intent yet.'), findsOneWidget);
  });

  testWidgets('no longer has an AppBar "+" — hosting moved to a separate entry '
      'point on HomePage so browsing and hosting aren\'t mixed on one page', (
    tester,
  ) async {
    final service = ScriptedMeetupService(openMeetups: const []);

    await tester.pumpWidget(_appWith(service, trustLevel: 2));
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.add_circle_outline), findsNothing);
  });

  testWidgets(
    'intent tabs switch selectedIntentProvider without leaving the page',
    (tester) async {
      final service = ScriptedMeetupService(openMeetups: const []);
      late final ProviderContainer container;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            meetupServiceProvider.overrideWithValue(service),
            authSessionProvider.overrideWith(
              () => _FakeAuthSessionNotifier(
                const AuthSessionState(
                  profile: UserProfile(
                    id: 'user-1',
                    fullName: 'Grace',
                    trustLevel: 2,
                  ),
                ),
              ),
            ),
          ],
          child: Consumer(
            builder: (context, ref, _) {
              container = ProviderScope.containerOf(context);
              return const MaterialApp(home: MatchesPage());
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(container.read(selectedIntentProvider), IntentType.coffee);
      expect(find.text('NETWORKING'), findsOneWidget);

      await tester.tap(find.text('NETWORKING'));
      await tester.pumpAndSettle();

      expect(container.read(selectedIntentProvider), IntentType.networking);
    },
  );

  group('Level 0 read-only audit (ADR-014)', () {
    testWidgets(
      'still renders the meetup list (read-only browse is the intended '
      'Level 0 capability), but REQUEST TO JOIN is disabled with an upsell',
      (tester) async {
        final service = ScriptedMeetupService(
          openMeetups: [_meetup(acceptedCount: 1, capacity: 4)],
        );

        await tester.pumpWidget(_appWith(service, trustLevel: 0));
        await tester.pumpAndSettle();

        // The list itself is not trust-gated at all.
        expect(find.text('Grace Hopper'), findsOneWidget);
        expect(find.text('Colombo Fort Cafe'), findsOneWidget);
        expect(find.text('1/4 JOINED'), findsOneWidget);

        // The action button is present but disabled, with an explanatory
        // upsell rather than silently vanishing or reaching the server.
        final button = tester.widget<GradientButton>(
          find.widgetWithText(GradientButton, 'REQUEST TO JOIN'),
        );
        expect(button.onPressed, isNull);
        expect(find.textContaining('Requires Level 2 trust'), findsOneWidget);
      },
    );

    testWidgets(
      'tapping the disabled REQUEST TO JOIN never reaches the server',
      (tester) async {
        final service = ScriptedMeetupService(
          openMeetups: [_meetup(id: 'meetup-99')],
        );

        await tester.pumpWidget(_appWith(service, trustLevel: 0));
        await tester.pumpAndSettle();

        await tester.tap(find.text('REQUEST TO JOIN'));
        await tester.pumpAndSettle();

        expect(service.lastRequestToJoinMeetupId, isNull);
      },
    );
  });
}
