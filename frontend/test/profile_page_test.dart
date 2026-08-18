import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_secure_storage_platform_interface/flutter_secure_storage_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/auth_session.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/storage/session_storage.dart';
import 'package:professional_connections_platform/features/landing/landing_page.dart';
import 'package:professional_connections_platform/features/profile/profile_page.dart';
import 'package:professional_connections_platform/features/verification/corporate_email_verification_page.dart';
import 'package:professional_connections_platform/features/verification/personal_details_page.dart';
import 'package:professional_connections_platform/features/verification/personal_email_verification_page.dart';
import 'package:professional_connections_platform/features/verification/phone_verification_page.dart';

import 'support/fake_secure_storage_platform.dart';

class _FakeAuthService implements AuthService {
  _FakeAuthService({this.logoutShouldThrow = false, UserProfile? profile})
    : _profile =
          profile ?? const UserProfile(id: 'user-1', fullName: 'Ada Lovelace');

  final bool logoutShouldThrow;
  final UserProfile _profile;
  int logoutCallCount = 0;
  String? lastLogoutRefreshToken;

  @override
  Future<AuthSession> signInWithLinkedIn() async => throw UnimplementedError();

  @override
  Future<AuthSession> refreshSession(String refreshToken) async =>
      throw UnimplementedError();

  @override
  Future<void> logout(String refreshToken) async {
    logoutCallCount++;
    lastLogoutRefreshToken = refreshToken;
    if (logoutShouldThrow) {
      throw const AuthNetworkException('backend unreachable');
    }
  }

  // authSessionProvider.build() calls getProfile() on every session load
  // now (frontend/PLAN.md's Level 2/3 addendum, Step 3) — every test using
  // this fake exercises this, so it needs a real implementation, not
  // UnimplementedError.
  @override
  Future<UserProfile> getProfile() async => _profile;

  @override
  Future<int> startPhoneVerification(String phoneNumber) async =>
      throw UnimplementedError();

  @override
  Future<AuthSession> verifyPhoneCode(String phoneNumber, String code) async =>
      throw UnimplementedError();

  @override
  Future<int> startPersonalEmailVerification(String email) async =>
      throw UnimplementedError();

  @override
  Future<AuthSession> verifyPersonalEmailCode(
    String email,
    String code,
  ) async => throw UnimplementedError();

  @override
  Future<AuthSession> submitPersonalDetails(
    String legalName,
    String address,
  ) async => throw UnimplementedError();

  @override
  Future<int> startCorporateEmailVerification(String email) async =>
      throw UnimplementedError();

  @override
  Future<AuthSession> verifyCorporateEmailCode(
    String email,
    String code,
  ) async => throw UnimplementedError();
}

final _testSession = AuthSession(
  userId: 'user-1',
  accessToken: 'a.b.c',
  refreshToken: 'refresh-token-abc',
  trustLevel: 1,
  isNewUser: false,
  accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
  fullName: 'Ada Lovelace',
  profilePhotoUrl: '',
);

/// Pumps until `ProfilePage` is gone (sign-out navigated away) or a bounded
/// number of pumps elapses. Not `pumpAndSettle` — the destination,
/// `LandingPage`, contains `OrbitingIntents`, a perpetually-repeating
/// animation (same reason `widget_test.dart`'s smoke test uses bounded
/// pumps), so `pumpAndSettle` would never converge there. A fixed pump
/// count is fragile against how many async hops `_signOut` actually takes;
/// polling for the actual outcome isn't.
Future<void> _pumpUntilSignedOut(WidgetTester tester) async {
  for (var i = 0; i < 20; i++) {
    if (find.byType(ProfilePage).evaluate().isEmpty) return;
    await tester.pump(const Duration(milliseconds: 100));
  }
}

/// Taps the original SIGN OUT tile (a `Glass`/`GestureDetector`, not a
/// `TextButton`) to open the confirmation dialog. `find.text('SIGN OUT')`
/// is unambiguous at this point — only the trigger tile has that text
/// before the dialog exists — so this must only ever run pre-dialog.
Future<void> _tapSignOutTrigger(WidgetTester tester) async {
  await tester.scrollUntilVisible(
    find.text('SIGN OUT'),
    500.0,
    scrollable: find.byType(Scrollable),
  );
  await tester.tap(find.text('SIGN OUT'));
  await tester.pumpAndSettle();
}

/// Taps the dialog's destructive "SIGN OUT" action. Scoped to `TextButton`
/// specifically — after the dialog opens, plain `find.text('SIGN OUT')`
/// matches both the (now-obscured) trigger tile and the dialog's own
/// title/button, but only the button is a `TextButton`.
Future<void> _confirmSignOutInDialog(WidgetTester tester) async {
  await tester.tap(find.widgetWithText(TextButton, 'SIGN OUT'));
  await _pumpUntilSignedOut(tester);
}

Widget _appWith(AuthService authService) {
  return ProviderScope(
    overrides: [
      authServiceProvider.overrideWithValue(authService),
      sessionStorageProvider.overrideWithValue(
        SecureSessionStorage(storage: const FlutterSecureStorage()),
      ),
    ],
    child: const MaterialApp(home: ProfilePage()),
  );
}

void main() {
  late FakeSecureStoragePlatform fakePlatform;

  setUp(() async {
    fakePlatform = FakeSecureStoragePlatform();
    FlutterSecureStoragePlatform.instance = fakePlatform;
    // Seed a signed-in session so authSessionProvider's build() (which
    // ProfilePage's sign-out reads the refresh token from) has one to load,
    // same as a real signed-in user landing on this page.
    await SecureSessionStorage(
      storage: const FlutterSecureStorage(),
    ).saveSession(_testSession);
  });

  testWidgets(
    'sign out calls logout, clears the session, and navigates to LandingPage',
    (tester) async {
      final auth = _FakeAuthService();
      await tester.pumpWidget(_appWith(auth));
      await tester.pumpAndSettle();

      // full_name genuinely reaches the UI from the cached session
      // (frontend/PLAN.md Step 10 self-review item), not just parsed and
      // discarded.
      expect(find.text('Ada Lovelace'), findsOneWidget);

      await _tapSignOutTrigger(tester);
      await _confirmSignOutInDialog(tester);

      expect(auth.logoutCallCount, 1);
      expect(auth.lastLogoutRefreshToken, 'refresh-token-abc');

      expect(find.byType(LandingPage), findsOneWidget);
      expect(find.byType(ProfilePage), findsNothing);

      final storage = SecureSessionStorage(
        storage: const FlutterSecureStorage(),
      );
      expect(await storage.loadSession(), isNull);
    },
  );

  testWidgets(
    'sign out clears the session and navigates even if the logout call fails',
    (tester) async {
      final auth = _FakeAuthService(logoutShouldThrow: true);
      await tester.pumpWidget(_appWith(auth));
      await tester.pumpAndSettle();

      await _tapSignOutTrigger(tester);
      await _confirmSignOutInDialog(tester);

      expect(auth.logoutCallCount, 1);

      // Idempotent-logout spirit (frontend/PLAN.md Step 8): a failed
      // network call must not leave the user stuck signed in locally.
      expect(find.byType(LandingPage), findsOneWidget);
      expect(find.byType(ProfilePage), findsNothing);

      final storage = SecureSessionStorage(
        storage: const FlutterSecureStorage(),
      );
      expect(await storage.loadSession(), isNull);
    },
  );

  testWidgets('back button cannot return to the signed-out page', (
    tester,
  ) async {
    final auth = _FakeAuthService();
    await tester.pumpWidget(_appWith(auth));
    await tester.pumpAndSettle();

    await _tapSignOutTrigger(tester);
    await _confirmSignOutInDialog(tester);

    // pushAndRemoveUntil((route) => false) clears the whole stack, so
    // there is nothing left for a back gesture to pop to.
    final navigator = tester.state<NavigatorState>(find.byType(Navigator));
    expect(await navigator.maybePop(), isFalse);
    expect(find.byType(LandingPage), findsOneWidget);
  });

  group('sign-out confirmation (frontend/PLAN.md Step 12)', () {
    testWidgets(
      'tapping SIGN OUT shows a confirmation dialog without signing out',
      (tester) async {
        final auth = _FakeAuthService();
        await tester.pumpWidget(_appWith(auth));
        await tester.pumpAndSettle();

        await _tapSignOutTrigger(tester);

        expect(
          find.text('Sign out of Professional Connections?'),
          findsOneWidget,
        );
        expect(auth.logoutCallCount, 0);
        expect(find.byType(ProfilePage), findsOneWidget);

        final storage = SecureSessionStorage(
          storage: const FlutterSecureStorage(),
        );
        expect(await storage.loadSession(), isNotNull);
      },
    );

    testWidgets('Cancel dismisses the dialog and leaves the session intact', (
      tester,
    ) async {
      final auth = _FakeAuthService();
      await tester.pumpWidget(_appWith(auth));
      await tester.pumpAndSettle();

      await _tapSignOutTrigger(tester);
      await tester.tap(find.widgetWithText(TextButton, 'CANCEL'));
      await tester.pumpAndSettle();

      expect(auth.logoutCallCount, 0);
      expect(find.byType(ProfilePage), findsOneWidget);
      expect(find.byType(LandingPage), findsNothing);
      // The dialog itself is gone, not just invisible.
      expect(find.text('Sign out of Professional Connections?'), findsNothing);

      final storage = SecureSessionStorage(
        storage: const FlutterSecureStorage(),
      );
      expect(await storage.loadSession(), isNotNull);
    });
  });

  group('LinkedIn verification row (frontend/PLAN.md Step 14)', () {
    testWidgets(
      'shows Connected with no VERIFY chip once signed in; the other four '
      'rows are unaffected',
      (tester) async {
        await tester.pumpWidget(_appWith(_FakeAuthService()));
        await tester.pumpAndSettle();

        expect(find.text('Connected'), findsOneWidget);
        expect(find.text('Not connected'), findsNothing);

        // Only LinkedIn's check icon — Phone/Personal Email/Personal
        // Details/Work Email are all unverified on this fake's default
        // profile, so they show VERIFY chips instead.
        expect(find.byIcon(Icons.check_circle_rounded), findsOneWidget);
        expect(find.text('VERIFY'), findsNWidgets(4));
        expect(find.text('Not verified'), findsNWidgets(4));
      },
    );
  });

  group('Verification rows (frontend/PLAN.md Level 2/3 addendum, Step 6)', () {
    testWidgets(
      'all four rows show Verified with a check icon once UserProfile '
      'reports them done, and no VERIFY chip remains',
      (tester) async {
        final auth = _FakeAuthService(
          profile: const UserProfile(
            id: 'user-1',
            fullName: 'Ada Lovelace',
            phoneVerified: true,
            personalEmailVerified: true,
            personalDetailsComplete: true,
            workEmailVerified: true,
          ),
        );
        await tester.pumpWidget(_appWith(auth));
        await tester.pumpAndSettle();

        // LinkedIn's own check icon plus the four now-verified rows.
        expect(find.byIcon(Icons.check_circle_rounded), findsNWidgets(5));
        expect(find.text('Verified'), findsNWidgets(4));
        expect(find.text('VERIFY'), findsNothing);
        expect(find.text('Not verified'), findsNothing);
      },
    );

    testWidgets('tapping Phone\'s VERIFY chip opens PhoneVerificationPage', (
      tester,
    ) async {
      await tester.pumpWidget(_appWith(_FakeAuthService()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('VERIFY').first);
      await tester.pumpAndSettle();

      expect(find.byType(PhoneVerificationPage), findsOneWidget);
    });

    testWidgets('tapping Personal Email\'s VERIFY chip opens '
        'PersonalEmailVerificationPage', (tester) async {
      await tester.pumpWidget(_appWith(_FakeAuthService()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('VERIFY').at(1));
      await tester.pumpAndSettle();

      expect(find.byType(PersonalEmailVerificationPage), findsOneWidget);
    });

    testWidgets(
      'tapping Personal Details\' VERIFY chip opens PersonalDetailsPage',
      (tester) async {
        await tester.pumpWidget(_appWith(_FakeAuthService()));
        await tester.pumpAndSettle();

        await tester.tap(find.text('VERIFY').at(2));
        await tester.pumpAndSettle();

        expect(find.byType(PersonalDetailsPage), findsOneWidget);
      },
    );

    testWidgets('tapping Work Email\'s VERIFY chip opens '
        'CorporateEmailVerificationPage', (tester) async {
      await tester.pumpWidget(_appWith(_FakeAuthService()));
      await tester.pumpAndSettle();

      await tester.tap(find.text('VERIFY').at(3));
      await tester.pumpAndSettle();

      expect(find.byType(CorporateEmailVerificationPage), findsOneWidget);
    });
  });
}
