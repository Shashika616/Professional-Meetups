import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/meetup_status_badge.dart';
import 'package:professional_connections_platform/features/meetups/my_meetups_page.dart';

import 'support/scripted_meetup_service.dart';

Meetup _hostedMeetup({String id = 'meetup-1'}) => Meetup(
  id: id,
  hostUserId: 'me',
  hostFullName: 'Me',
  hostTrustLevel: 3,
  intent: IntentType.coffee,
  windowStart: DateTime.now().add(const Duration(hours: 1)),
  windowEnd: DateTime.now().add(const Duration(hours: 3)),
  locationLat: 6.9271,
  locationLng: 79.8612,
  locationLabel: 'Colombo Fort Cafe',
  capacity: 4,
  acceptedCount: 0,
  status: MeetupStatus.open,
  createdAt: DateTime.now(),
  isHostedByMe: true,
);

Meetup _hostedMeetupStarted({String id = 'meetup-1'}) => Meetup(
  id: id,
  hostUserId: 'me',
  hostFullName: 'Me',
  hostTrustLevel: 3,
  intent: IntentType.coffee,
  windowStart: DateTime.now().subtract(const Duration(minutes: 15)),
  windowEnd: DateTime.now().add(const Duration(hours: 1)),
  locationLat: 6.9271,
  locationLng: 79.8612,
  locationLabel: 'Colombo Fort Cafe',
  capacity: 4,
  acceptedCount: 0,
  status: MeetupStatus.open,
  createdAt: DateTime.now(),
  isHostedByMe: true,
);

MeetupRequestModel _pendingRequest({
  String id = 'request-1',
  String requesterFullName = 'Grace Hopper',
  int requesterTrustLevel = 2,
}) => MeetupRequestModel(
  id: id,
  meetupId: 'meetup-1',
  requesterId: 'requester-1',
  requesterFullName: requesterFullName,
  requesterTrustLevel: requesterTrustLevel,
  status: MeetupRequestStatus.pending,
  createdAt: DateTime.now(),
);

void main() {
  testWidgets(
    'tapping a hosted meetup opens request management, rendering Accept/Reject',
    (tester) async {
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [_hostedMeetup()], requested: const []),
        meetupRequests: [_pendingRequest()],
        meetupDetail: _hostedMeetup(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Colombo Fort Cafe'), findsOneWidget);
      await tester.tap(find.text('Colombo Fort Cafe'));
      await tester.pumpAndSettle();

      expect(find.text('Grace Hopper'), findsOneWidget);
      expect(find.text('ACCEPT'), findsOneWidget);
      expect(find.text('REJECT'), findsOneWidget);
    },
  );

  testWidgets(
    'renders on the app background image, both on the list and the pushed '
    'request-management page — this page is reached via Navigator.push, '
    'not one of AppShell\'s own bottom-nav tabs, so it needs its own '
    'AppBackground rather than inheriting AppShell\'s',
    (tester) async {
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [_hostedMeetup()], requested: const []),
        meetupRequests: [_pendingRequest()],
        meetupDetail: _hostedMeetup(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(AppBackground), findsOneWidget);

      await tester.tap(find.text('Colombo Fort Cafe'));
      await tester.pumpAndSettle();

      // The previous route (MyMeetupsPage) stays built underneath by
      // default (PageRoute.maintainState), so this is >=1, not exactly
      // one — the point is _RequestManagementPage contributes its own
      // AppBackground rather than rendering with none at all.
      expect(find.byType(AppBackground), findsWidgets);
      expect(find.text('Grace Hopper'), findsOneWidget);
    },
  );

  testWidgets(
    'tapping ACCEPT calls respondToRequest with that request\'s id and accept:true',
    (tester) async {
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [_hostedMeetup()], requested: const []),
        meetupRequests: [_pendingRequest(id: 'request-99')],
        meetupDetail: _hostedMeetup(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Colombo Fort Cafe'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('ACCEPT'));
      await tester.pumpAndSettle();

      expect(service.lastRespondToRequestId, 'request-99');
      expect(service.lastRespondToRequestAccept, isTrue);
    },
  );

  testWidgets(
    'tapping REJECT calls respondToRequest with that request\'s id and accept:false',
    (tester) async {
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [_hostedMeetup()], requested: const []),
        meetupRequests: [_pendingRequest(id: 'request-7')],
        meetupDetail: _hostedMeetup(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Colombo Fort Cafe'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('REJECT'));
      await tester.pumpAndSettle();

      expect(service.lastRespondToRequestId, 'request-7');
      expect(service.lastRespondToRequestAccept, isFalse);
    },
  );

  testWidgets('the Hosting tab shows a status badge on each meetup card', (
    tester,
  ) async {
    final service = ScriptedMeetupService(
      myMeetups: (hosted: [_hostedMeetup()], requested: const []),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [meetupServiceProvider.overrideWithValue(service)],
        child: const MaterialApp(home: MyMeetupsPage()),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      tester.widget<MeetupStatusBadge>(find.byType(MeetupStatusBadge)).status,
      MeetupStatus.open,
    );
  });

  testWidgets(
    'the request-management screen reached from the Hosting tab shows '
    'Cancel/Close — the exact gap this addendum fixes: that screen used to '
    'be Accept/Reject only, with no way to control the meetup itself '
    '(ADR-016 addendum, 2026-08-20)',
    (tester) async {
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [_hostedMeetupStarted()], requested: const []),
        meetupDetail: _hostedMeetupStarted(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      // The normal path: calendar icon (MyMeetupsPage itself, already
      // reached) → HOSTING (the default-selected tab) → a hosted meetup.
      await tester.tap(find.text('Colombo Fort Cafe'));
      await tester.pumpAndSettle();

      expect(find.text('CANCEL MEETUP'), findsOneWidget);
      expect(find.text('CLOSE MEETUP'), findsOneWidget);
      expect(
        tester
            .widgetList<MeetupStatusBadge>(find.byType(MeetupStatusBadge))
            .map((b) => b.status),
        contains(MeetupStatus.open),
      );
    },
  );

  testWidgets(
    'CANCEL MEETUP on the request-management screen updates the header '
    'badge to CANCELLED after confirming — a real state change, not just a '
    'visually-present dialog',
    (tester) async {
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [_hostedMeetupStarted()], requested: const []),
        meetupDetail: _hostedMeetupStarted(),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Colombo Fort Cafe'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('CANCEL MEETUP'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(TextButton, 'CANCEL MEETUP'));
      await tester.pumpAndSettle();

      expect(service.lastCancelMeetupId, 'meetup-1');
      expect(find.text('CANCELLED'), findsOneWidget);
      // Both actions disappear once cancelled — neither applies anymore.
      expect(find.text('CANCEL MEETUP'), findsNothing);
      expect(find.text('CLOSE MEETUP'), findsNothing);

      await tester.pump(const Duration(seconds: 3));
    },
  );

  testWidgets(
    'the REQUESTED tab distinguishes auto-reject from an explicit host rejection',
    (tester) async {
      final autoRejected = _hostedMeetup(
        id: 'meetup-auto',
      ).copyWithRequestStatus(MeetupRequestStatus.rejected, autoRejected: true);
      final explicitlyRejected = _hostedMeetup(id: 'meetup-explicit')
          .copyWithRequestStatus(
            MeetupRequestStatus.rejected,
            autoRejected: false,
          );

      final service = ScriptedMeetupService(
        myMeetups: (
          hosted: const [],
          requested: [autoRejected, explicitlyRejected],
        ),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: MyMeetupsPage()),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('REQUESTED'));
      await tester.pumpAndSettle();

      expect(find.textContaining('NOT SELECTED'), findsOneWidget);
      expect(find.text('DECLINED BY HOST'), findsOneWidget);
    },
  );
}

extension on Meetup {
  Meetup copyWithRequestStatus(
    MeetupRequestStatus status, {
    required bool autoRejected,
  }) => Meetup(
    id: id,
    hostUserId: hostUserId,
    hostFullName: hostFullName,
    hostTrustLevel: hostTrustLevel,
    intent: intent,
    windowStart: windowStart,
    windowEnd: windowEnd,
    locationLat: locationLat,
    locationLng: locationLng,
    locationLabel: locationLabel,
    capacity: capacity,
    acceptedCount: acceptedCount,
    status: MeetupStatus.open,
    createdAt: createdAt,
    isHostedByMe: false,
    myRequestStatus: status,
    myRequestAutoRejected: autoRejected,
  );
}
