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
import 'package:professional_connections_platform/features/onboarding/email_signup_step.dart';

import 'support/fake_secure_storage_platform.dart';

/// Only the two methods EmailSignupStep actually calls have real behavior —
/// everything else throws if reached, matching this suite's existing
/// _FakeAuthService convention (app_shell_test.dart etc.).
class _FakeAuthService implements AuthService {
  int startOtpCallCount = 0;
  String? lastSignUpEmail;
  String? lastSignUpCode;
  String? lastSignUpPassword;
  bool? lastAgeConfirmed;
  Object? signUpError;

  @override
  Future<int> startEmailSignupOtp(String email) async {
    startOtpCallCount++;
    return 30;
  }

  @override
  Future<AuthSession> signUpWithEmail({
    required String email,
    required String code,
    required String password,
    required bool ageConfirmedOver18,
  }) async {
    lastSignUpEmail = email;
    lastSignUpCode = code;
    lastSignUpPassword = password;
    lastAgeConfirmed = ageConfirmedOver18;
    if (signUpError != null) throw signUpError!;
    return AuthSession(
      userId: 'user-1',
      accessToken: 'a.b.c',
      refreshToken: 'refresh-token',
      trustLevel: 0,
      isNewUser: true,
      accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
      fullName: '',
      profilePhotoUrl: '',
    );
  }

  @override
  Future<AuthSession> signInWithLinkedIn({
    required bool ageConfirmedOver18,
  }) async => throw UnimplementedError();

  @override
  Future<AuthSession> signInWithApple({
    required bool ageConfirmedOver18,
  }) async => throw UnimplementedError();

  @override
  Future<AuthSession> signInWithGoogle({
    required bool ageConfirmedOver18,
  }) async => throw UnimplementedError();

  @override
  Future<AuthSession> loginWithEmail({
    required String email,
    required String password,
  }) async => throw UnimplementedError();

  @override
  Future<AuthSession> linkLinkedIn() async => throw UnimplementedError();

  @override
  Future<AuthSession> refreshSession(String refreshToken) async =>
      throw UnimplementedError();

  @override
  Future<void> logout(String refreshToken) async {}

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

  @override
  Future<UserProfile> getProfile() async =>
      const UserProfile(id: 'user-1', fullName: '');
}

Widget _appWith(AuthService authService) {
  return ProviderScope(
    overrides: [
      authServiceProvider.overrideWithValue(authService),
      sessionStorageProvider.overrideWithValue(
        SecureSessionStorage(storage: const FlutterSecureStorage()),
      ),
    ],
    child: const MaterialApp(home: EmailSignupStep(ageConfirmedOver18: true)),
  );
}

void main() {
  setUp(() {
    FlutterSecureStoragePlatform.instance = FakeSecureStoragePlatform();
  });

  testWidgets('email step: SEND CODE is disabled until an email is entered', (
    tester,
  ) async {
    final auth = _FakeAuthService();
    await tester.pumpWidget(_appWith(auth));
    await tester.pumpAndSettle();

    expect(find.text('SEND CODE'), findsOneWidget);
    await tester.tap(find.text('SEND CODE'));
    await tester.pumpAndSettle();
    expect(auth.startOtpCallCount, 0);

    await tester.enterText(find.byType(TextFormField), 'ada@example.com');
    await tester.pumpAndSettle();
    await tester.tap(find.text('SEND CODE'));
    await tester.pumpAndSettle();

    expect(auth.startOtpCallCount, 1);
  });

  testWidgets('full flow: email → OTP entry (local, no network) → password → '
      'signUpWithEmail carries the entered code and password', (tester) async {
    final auth = _FakeAuthService();
    await tester.pumpWidget(_appWith(auth));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextFormField), 'ada@example.com');
    await tester.pumpAndSettle();
    await tester.tap(find.text('SEND CODE'));
    await tester.pumpAndSettle();

    // Now on the OTP step (reuses OtpEntry) — entering 6 digits and
    // tapping VERIFY just advances locally, no network call for this
    // step (there is no separate "verify signup code" endpoint).
    await tester.enterText(find.byType(TextFormField), '123456');
    await tester.pumpAndSettle();
    await tester.tap(find.text('VERIFY'));
    await tester.pumpAndSettle();

    expect(find.text('Set a password'), findsOneWidget);
    expect(find.text('CREATE ACCOUNT'), findsOneWidget);

    // CREATE ACCOUNT stays disabled below the 8-character hint.
    await tester.enterText(find.byType(TextFormField), 'short');
    await tester.pumpAndSettle();
    await tester.tap(find.text('CREATE ACCOUNT'));
    await tester.pumpAndSettle();
    expect(auth.lastSignUpPassword, isNull);

    await tester.enterText(find.byType(TextFormField), 'longenoughpw');
    await tester.pumpAndSettle();
    await tester.tap(find.text('CREATE ACCOUNT'));
    await tester.pumpAndSettle();

    expect(auth.lastSignUpEmail, 'ada@example.com');
    expect(auth.lastSignUpCode, '123456');
    expect(auth.lastSignUpPassword, 'longenoughpw');
    expect(auth.lastAgeConfirmed, isTrue);
  });
}
