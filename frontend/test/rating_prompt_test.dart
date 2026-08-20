import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/features/meetups/widgets/rating_prompt.dart';

import 'support/scripted_meetup_service.dart';

void main() {
  testWidgets('renders each not-yet-rated participant with a name and a 5-star '
      'picker, and an already-rated one as RATED with no picker', (
    tester,
  ) async {
    final service = ScriptedMeetupService(
      ratableParticipants: const [
        RatableParticipant(
          userId: 'user-2',
          fullName: 'Grace Hopper',
          trustLevel: 2,
        ),
        RatableParticipant(
          userId: 'user-3',
          fullName: 'Ada Lovelace',
          trustLevel: 3,
          alreadyRated: true,
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [meetupServiceProvider.overrideWithValue(service)],
        child: const MaterialApp(
          home: Scaffold(body: RatingPrompt(meetupId: 'meetup-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Grace Hopper'), findsOneWidget);
    expect(find.text('Ada Lovelace'), findsOneWidget);
    expect(find.text('RATED'), findsOneWidget);
    expect(find.byIcon(Icons.star_outline_rounded), findsNWidgets(5));
  });

  testWidgets('tapping a star shows a confirmation dialog first — ratings are '
      'immutable and one-shot (ADR-015), so this is the last chance to catch '
      'a mis-tap (ADR-016)', (tester) async {
    final service = ScriptedMeetupService(
      ratableParticipants: const [
        RatableParticipant(
          userId: 'user-2',
          fullName: 'Grace Hopper',
          trustLevel: 2,
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [meetupServiceProvider.overrideWithValue(service)],
        child: const MaterialApp(
          home: Scaffold(body: RatingPrompt(meetupId: 'meetup-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.star_outline_rounded).at(3));
    await tester.pumpAndSettle();

    expect(find.text('CONFIRM RATING'), findsOneWidget);
    expect(find.textContaining('Rate Grace Hopper 4 stars'), findsOneWidget);
    // Not submitted yet — still just showing the dialog.
    expect(service.lastSubmitRatingMeetupId, isNull);
  });

  testWidgets(
    'Cancel on the confirmation dialog does not call the API — the star '
    'picker stays tappable, nothing is marked RATED',
    (tester) async {
      final service = ScriptedMeetupService(
        ratableParticipants: const [
          RatableParticipant(
            userId: 'user-2',
            fullName: 'Grace Hopper',
            trustLevel: 2,
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(
            home: Scaffold(body: RatingPrompt(meetupId: 'meetup-1')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.star_outline_rounded).at(3));
      await tester.pumpAndSettle();

      await tester.tap(find.text('CANCEL'));
      await tester.pumpAndSettle();

      expect(service.lastSubmitRatingMeetupId, isNull);
      expect(find.text('RATED'), findsNothing);
      expect(find.byIcon(Icons.star_outline_rounded), findsNWidgets(5));
    },
  );

  testWidgets(
    'Confirm on the dialog submits exactly once, with the tapped score, '
    'and the picker becomes a RATED label',
    (tester) async {
      final service = ScriptedMeetupService(
        ratableParticipants: const [
          RatableParticipant(
            userId: 'user-2',
            fullName: 'Grace Hopper',
            trustLevel: 2,
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [meetupServiceProvider.overrideWithValue(service)],
          child: const MaterialApp(
            home: Scaffold(body: RatingPrompt(meetupId: 'meetup-1')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final stars = find.byIcon(Icons.star_outline_rounded);
      await tester.tap(stars.at(3)); // 4th star, 1-indexed score = 4
      await tester.pumpAndSettle();

      await tester.tap(find.text('CONFIRM'));
      await tester.pumpAndSettle();

      expect(service.lastSubmitRatingMeetupId, 'meetup-1');
      expect(service.lastSubmitRatingRatedUserId, 'user-2');
      expect(service.lastSubmitRatingScore, 4);
      expect(find.text('RATED'), findsOneWidget);
      expect(find.byIcon(Icons.star_outline_rounded), findsNothing);
    },
  );

  testWidgets('an empty ratable list renders nothing', (tester) async {
    final service = ScriptedMeetupService(ratableParticipants: const []);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [meetupServiceProvider.overrideWithValue(service)],
        child: const MaterialApp(
          home: Scaffold(body: RatingPrompt(meetupId: 'meetup-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('RATE WHO YOU MET'), findsNothing);
  });
}
