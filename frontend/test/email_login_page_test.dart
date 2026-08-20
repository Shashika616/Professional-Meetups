import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_secure_storage_platform_interface/flutter_secure_storage_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/app_shell.dart';
import 'package:professional_connections_platform/core/models/auth_session.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/storage/session_storage.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/auth/email_login_page.dart';

import 'support/fake_meetup_service.dart';
import 'support/fake_secure_storage_platform.dart';

class _FakeAuthService implements AuthService {
  _FakeAuthService({this.error});

  final Object? error;
  String? lastEmail;
  String? lastPassword;

  @override
  Future<AuthSession> loginWithEmail({
    required String email,
    required String password,
  }) async {
    lastEmail = email;
    lastPassword = password;
    if (error != null) throw error!;
    return AuthSession(
      userId: 'user-1',
      accessToken: 'a.b.c',
      refreshToken: 'refresh-token',
      trustLevel: 0,
      isNewUser: false,
      accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
      fullName: 'Ada Lovelace',
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
  Future<AuthSession> signUpWithEmail({
    required String email,
    required String code,
    required String password,
    required bool ageConfirmedOver18,
  }) async => throw UnimplementedError();

  @override
  Future<AuthSession> linkLinkedIn() async => throw UnimplementedError();

  @override
  Future<int> startEmailSignupOtp(String email) async =>
      throw UnimplementedError();

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
      const UserProfile(id: 'user-1', fullName: 'Ada Lovelace');
}

Widget _appWith(AuthService authService) {
  return ProviderScope(
    overrides: [
      authServiceProvider.overrideWithValue(authService),
      sessionStorageProvider.overrideWithValue(
        SecureSessionStorage(storage: const FlutterSecureStorage()),
      ),
      meetupServiceProvider.overrideWithValue(ImmediateMeetupService()),
      homeStatsProvider.overrideWith(
        (ref) async => const {'nearby': 0, 'meetups': 0, 'trustScore': 0.0},
      ),
    ],
    child: const MaterialApp(home: EmailLoginPage()),
  );
}

void main() {
  setUp(() {
    FlutterSecureStoragePlatform.instance = FakeSecureStoragePlatform();
  });

  testWidgets('SIGN IN is disabled until both fields are filled', (
    tester,
  ) async {
    final auth = _FakeAuthService();
    await tester.pumpWidget(_appWith(auth));
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(GradientButton, 'SIGN IN'));
    await tester.pumpAndSettle();
    expect(auth.lastEmail, isNull);

    final fields = find.byType(TextFormField);
    await tester.enterText(fields.at(0), 'ada@example.com');
    await tester.enterText(fields.at(1), 'hunter22');
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(GradientButton, 'SIGN IN'));
    await tester.pumpAndSettle();

    expect(auth.lastEmail, 'ada@example.com');
    expect(auth.lastPassword, 'hunter22');
    expect(find.byType(AppShell), findsOneWidget);
  });

  testWidgets('invalid credentials shows the mapped error and stays put', (
    tester,
  ) async {
    final auth = _FakeAuthService(
      error: const InvalidCredentialsException('invalid email or password'),
    );
    await tester.pumpWidget(_appWith(auth));
    await tester.pumpAndSettle();

    final fields = find.byType(TextFormField);
    await tester.enterText(fields.at(0), 'ada@example.com');
    await tester.enterText(fields.at(1), 'wrong');
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(GradientButton, 'SIGN IN'));
    await tester.pumpAndSettle();

    expect(find.text('invalid email or password'), findsOneWidget);
    expect(find.byType(EmailLoginPage), findsOneWidget);
    expect(find.byType(AppShell), findsNothing);
  });
}
