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
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/verification/personal_details_page.dart';

import 'support/fake_secure_storage_platform.dart';

class _FakeAuthService implements AuthService {
  _FakeAuthService({this.submitResult, this.submitError});

  final AuthSession? submitResult;
  final Object? submitError;
  int submitCallCount = 0;
  String? lastLegalName;
  String? lastAddress;

  @override
  Future<AuthSession> submitPersonalDetails(
    String legalName,
    String address,
  ) async {
    submitCallCount++;
    lastLegalName = legalName;
    lastAddress = address;
    if (submitError != null) throw submitError!;
    return submitResult!;
  }

  @override
  Future<AuthSession> signInWithLinkedIn() async => throw UnimplementedError();

  @override
  Future<AuthSession> refreshSession(String refreshToken) async =>
      throw UnimplementedError();

  @override
  Future<void> logout(String refreshToken) async {}

  @override
  Future<UserProfile> getProfile() async =>
      const UserProfile(id: 'user-1', fullName: 'Ada Lovelace');

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
  Future<int> startCorporateEmailVerification(String email) async =>
      throw UnimplementedError();

  @override
  Future<AuthSession> verifyCorporateEmailCode(
    String email,
    String code,
  ) async => throw UnimplementedError();
}

Widget _appWith(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      home: Navigator(
        onGenerateRoute: (settings) => MaterialPageRoute(
          builder: (context) => const PersonalDetailsPage(),
        ),
      ),
    ),
  );
}

ProviderContainer _containerWith(AuthService auth) {
  return ProviderContainer(
    overrides: [
      authServiceProvider.overrideWithValue(auth),
      sessionStorageProvider.overrideWithValue(
        SecureSessionStorage(storage: const FlutterSecureStorage()),
      ),
    ],
  );
}

void main() {
  setUp(() {
    FlutterSecureStoragePlatform.instance = FakeSecureStoragePlatform();
  });

  testWidgets(
    'Skip pops the screen without ever calling submitPersonalDetails',
    (tester) async {
      final auth = _FakeAuthService();
      final container = _containerWith(auth);
      addTearDown(container.dispose);

      await tester.pumpWidget(_appWith(container));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Skip for now'));
      await tester.pumpAndSettle();

      expect(find.byType(PersonalDetailsPage), findsNothing);
      expect(auth.submitCallCount, 0);
    },
  );

  testWidgets('CONTINUE stays disabled until both fields are filled', (
    tester,
  ) async {
    final auth = _FakeAuthService();
    final container = _containerWith(auth);
    addTearDown(container.dispose);

    await tester.pumpWidget(_appWith(container));
    await tester.pumpAndSettle();

    final fields = find.byType(TextField);
    expect(
      tester.widget<GradientButton>(find.byType(GradientButton)).onPressed,
      isNull,
    );

    await tester.enterText(fields.at(0), 'Ada Lovelace');
    await tester.pump();
    expect(
      tester.widget<GradientButton>(find.byType(GradientButton)).onPressed,
      isNull,
    );

    await tester.enterText(fields.at(1), '10 Downing Street');
    await tester.pump();
    expect(
      tester.widget<GradientButton>(find.byType(GradientButton)).onPressed,
      isNotNull,
    );
  });

  testWidgets('a submit failure shows the mapped error and stays on this '
      'screen', (tester) async {
    final auth = _FakeAuthService(
      submitError: const AuthNetworkException('Something went wrong.'),
    );
    final container = _containerWith(auth);
    addTearDown(container.dispose);

    await tester.pumpWidget(_appWith(container));
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField).at(0), 'Ada Lovelace');
    await tester.enterText(find.byType(TextField).at(1), '10 Downing Street');
    await tester.pump();
    await tester.tap(find.byType(GradientButton));
    await tester.pump();
    await tester.pump();

    expect(find.text('Something went wrong.'), findsOneWidget);
    expect(find.byType(PersonalDetailsPage), findsOneWidget);
  });

  testWidgets(
    'a successful submit updates the session and profile immediately, '
    'then pops',
    (tester) async {
      final updatedSession = AuthSession(
        userId: 'user-1',
        accessToken: 'new.a.b',
        refreshToken: 'new-refresh',
        trustLevel: 2,
        isNewUser: false,
        accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
        fullName: 'Ada Lovelace',
        profilePhotoUrl: '',
      );
      final auth = _FakeAuthService(submitResult: updatedSession);
      final container = _containerWith(auth);
      addTearDown(container.dispose);
      await container.read(authSessionProvider.future);

      await tester.pumpWidget(_appWith(container));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField).at(0), 'Ada Lovelace');
      await tester.enterText(find.byType(TextField).at(1), '10 Downing Street');
      await tester.pump();
      await tester.tap(find.byType(GradientButton));
      await tester.pumpAndSettle();

      expect(auth.submitCallCount, 1);
      expect(auth.lastLegalName, 'Ada Lovelace');
      expect(auth.lastAddress, '10 Downing Street');
      expect(find.byType(PersonalDetailsPage), findsNothing);

      final sessionState = container.read(authSessionProvider).value!;
      expect(sessionState.session!.accessToken, 'new.a.b');
      expect(sessionState.profile, isNotNull);
    },
  );
}
