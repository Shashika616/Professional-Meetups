import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/widgets/professional_avatar.dart';
import 'package:professional_connections_platform/features/home/home_page.dart';
import 'package:professional_connections_platform/features/home/widgets/home_header.dart';
import 'package:professional_connections_platform/features/meetups/schedule_flow.dart';

import 'support/fake_meetup_service.dart';

/// Resolves immediately to a fixed [AuthSessionState] instead of reading
/// secure storage — HomePage only ever reads `.profile` off this provider,
/// so there's no need for ProfilePage's full secure-storage-seeding setup.
class _FakeAuthSessionNotifier extends AuthSessionNotifier {
  _FakeAuthSessionNotifier(this._state);

  final AuthSessionState _state;

  @override
  Future<AuthSessionState> build() async => _state;
}

void main() {
  group('HomeHeader (frontend/PLAN.md Step 13)', () {
    testWidgets(
      'renders the profile photo via ProfessionalAvatar when imageUrl is provided',
      (tester) async {
        await tester.pumpWidget(
          const MaterialApp(
            home: Scaffold(
              body: HomeHeader(
                userName: 'Ada Lovelace',
                imageUrl: 'https://example.com/photo.jpg',
              ),
            ),
          ),
        );
        await tester.pump();

        final avatar = tester.widget<ProfessionalAvatar>(
          find.byType(ProfessionalAvatar),
        );
        expect(avatar.imageUrl, 'https://example.com/photo.jpg');

        // Scoped to ProfessionalAvatar specifically — HomeHeader's AppIcon
        // also renders an Image (its asset logo), so an unscoped
        // find.byType(Image) would match both.
        final avatarImage = find.descendant(
          of: find.byType(ProfessionalAvatar),
          matching: find.byType(Image),
        );
        final image = tester.widget<Image>(avatarImage);
        expect(
          (image.image as NetworkImage).url,
          'https://example.com/photo.jpg',
        );
      },
    );

    testWidgets('falls back to initials when imageUrl is null', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(body: HomeHeader(userName: 'Ada Lovelace')),
        ),
      );
      await tester.pump();

      final avatar = tester.widget<ProfessionalAvatar>(
        find.byType(ProfessionalAvatar),
      );
      expect(avatar.imageUrl, isNull);
      expect(
        find.descendant(
          of: find.byType(ProfessionalAvatar),
          matching: find.byType(Image),
        ),
        findsNothing,
      );
    });
  });

  group('HomePage (frontend/PLAN.md Step 13)', () {
    testWidgets(
      "shows the real signed-in user's name instead of any hardcoded string",
      (tester) async {
        const profile = UserProfile(
          id: 'user-1',
          fullName: 'Grace Hopper',
          profilePhotoUrl: 'https://example.com/grace.jpg',
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authSessionProvider.overrideWith(
                () => _FakeAuthSessionNotifier(
                  const AuthSessionState(profile: profile),
                ),
              ),
              // UpcomingMeetupCard reads myMeetupsProvider (backed by this)
              // — without an override it defaults to the real
              // HttpMeetupService and attempts a live network call.
              meetupServiceProvider.overrideWithValue(ImmediateMeetupService()),
              // NetworkInsightsRow's homeStatsProvider has its own
              // real 1s Future.delayed — pumpAndSettle doesn't reliably
              // advance fake time far enough to flush a bare unscheduled
              // Timer that nothing else keeps re-triggering, so left
              // un-overridden it leaks a pending timer past test teardown.
              homeStatsProvider.overrideWith(
                (ref) async => const {
                  'nearby': 0,
                  'meetups': 0,
                  'trustScore': 0.0,
                },
              ),
            ],
            child: const MaterialApp(home: HomePage()),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Grace Hopper'), findsOneWidget);
        expect(find.text('Shashika Fernando'), findsNothing);

        final header = tester.widget<HomeHeader>(find.byType(HomeHeader));
        expect(header.imageUrl, 'https://example.com/grace.jpg');
      },
    );

    testWidgets('falls back to "Member" when no profile is loaded yet', (
      tester,
    ) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authSessionProvider.overrideWith(
              () => _FakeAuthSessionNotifier(const AuthSessionState()),
            ),
            meetupServiceProvider.overrideWithValue(ImmediateMeetupService()),
            homeStatsProvider.overrideWith(
              (ref) async => const {
                'nearby': 0,
                'meetups': 0,
                'trustScore': 0.0,
              },
            ),
          ],
          child: const MaterialApp(home: HomePage()),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Member'), findsOneWidget);
      expect(find.text('Shashika Fernando'), findsNothing);
    });

    testWidgets(
      'HOST YOUR OWN MEETUP opens ScheduleFlowPage directly — hosting used '
      'to be reachable only via a "+" icon on the browse/Matches page, '
      'which this button replaces as the primary entry point',
      (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authSessionProvider.overrideWith(
                () => _FakeAuthSessionNotifier(const AuthSessionState()),
              ),
              meetupServiceProvider.overrideWithValue(ImmediateMeetupService()),
              homeStatsProvider.overrideWith(
                (ref) async => const {
                  'nearby': 0,
                  'meetups': 0,
                  'trustScore': 0.0,
                },
              ),
            ],
            child: const MaterialApp(home: HomePage()),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('HOST YOUR OWN MEETUP'), findsOneWidget);
        await tester.tap(find.text('HOST YOUR OWN MEETUP'));
        await tester.pumpAndSettle();

        expect(find.byType(ScheduleFlowPage), findsOneWidget);
      },
    );
  });
}
