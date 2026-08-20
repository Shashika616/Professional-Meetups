import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/widgets/meetup_status_badge.dart';
import 'package:professional_connections_platform/features/home/widgets/upcoming_meetup_card.dart';

import 'support/scripted_meetup_service.dart';

void main() {
  testWidgets(
    'the host name and location each get their own line instead of being '
    'concatenated with the intent label/date into one ellipsized string — '
    'the old layout truncated real names down to e.g. "Shashik..." and cut '
    'off the location entirely',
    (tester) async {
      const longName = 'Shashika Fernando Wickramasinghe';
      const longLocation =
          'Pizza Hut, No.321A, Union Place, Colombo 02, Sri Lanka';
      final meetup = Meetup(
        id: 'meetup-1',
        hostUserId: 'host-1',
        hostFullName: longName,
        hostTrustLevel: 2,
        intent: IntentType.networking,
        windowStart: DateTime.now().add(const Duration(hours: 1)),
        windowEnd: DateTime.now().add(const Duration(hours: 3)),
        locationLat: 6.9271,
        locationLng: 79.8612,
        locationLabel: longLocation,
        capacity: 4,
        acceptedCount: 1,
        status: MeetupStatus.open,
        createdAt: DateTime.now(),
        isHostedByMe: true,
      );
      final service = ScriptedMeetupService(
        myMeetups: (hosted: [meetup], requested: const []),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(home: Scaffold(body: UpcomingMeetupCard())),
        ),
      );
      await tester.pumpAndSettle();

      // Old combined format is gone.
      expect(
        find.text('${IntentType.networking.label} • $longName'),
        findsNothing,
      );

      final nameFinder = find.text(longName);
      expect(nameFinder, findsOneWidget);
      final nameWidget = tester.widget<Text>(nameFinder);
      expect(
        nameWidget.maxLines,
        1,
        reason:
            'the name is on its own line now, not sharing one with the '
            'intent label',
      );

      final locationFinder = find.text(longLocation);
      expect(locationFinder, findsOneWidget);
      final locationWidget = tester.widget<Text>(locationFinder);
      expect(
        locationWidget.maxLines,
        2,
        reason:
            'location wraps instead of ellipsizing on one line — where '
            'to go is crucial info',
      );
      expect(
        locationWidget.data,
        longLocation,
        reason:
            'not prefixed with the date anymore, so it has more room '
            'to actually show the address',
      );
    },
  );

  testWidgets('shows the meetup\'s lifecycle status badge alongside CONFIRMED '
      '(ADR-016 addendum, 2026-08-20)', (tester) async {
    final meetup = Meetup(
      id: 'meetup-1',
      hostUserId: 'host-1',
      hostFullName: 'Grace Hopper',
      hostTrustLevel: 2,
      intent: IntentType.coffee,
      windowStart: DateTime.now().add(const Duration(hours: 1)),
      windowEnd: DateTime.now().add(const Duration(hours: 3)),
      locationLat: 6.9271,
      locationLng: 79.8612,
      locationLabel: 'Colombo Fort Cafe',
      capacity: 4,
      acceptedCount: 3,
      status: MeetupStatus.full,
      createdAt: DateTime.now(),
      isHostedByMe: true,
    );
    final service = ScriptedMeetupService(
      myMeetups: (hosted: [meetup], requested: const []),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [meetupServiceProvider.overrideWithValue(service)],
        child: const MaterialApp(home: Scaffold(body: UpcomingMeetupCard())),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      tester.widget<MeetupStatusBadge>(find.byType(MeetupStatusBadge)).status,
      MeetupStatus.full,
    );
  });
}
