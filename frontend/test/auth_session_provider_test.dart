import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_secure_storage_platform_interface/flutter_secure_storage_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/auth_session.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/services/token_refresher.dart';
import 'package:professional_connections_platform/core/storage/session_storage.dart';

import 'support/fake_secure_storage_platform.dart';

/// Every method throws except getProfile() — the notifier's build() path
/// under test never needs the others, and UnimplementedError makes it
/// obvious if that assumption ever stops holding.
class _FakeAuthService implements AuthService {
  _FakeAuthService(this._profile);

  final UserProfile _profile;

  @override
  Future<UserProfile> getProfile() async => _profile;

  @override
  Future<AuthSession> signInWithLinkedIn() async => throw UnimplementedError();

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
}

AuthSession _sessionExpiringIn(Duration delta, {String accessToken = 'a1'}) {
  return AuthSession(
    userId: 'user-1',
    accessToken: accessToken,
    refreshToken: 'refresh-1',
    trustLevel: 1,
    isNewUser: false,
    accessTokenExpiresAt: DateTime.now().add(delta),
    fullName: 'Ada Lovelace',
    profilePhotoUrl: '',
  );
}

void main() {
  late SecureSessionStorage storage;

  setUp(() {
    FlutterSecureStoragePlatform.instance = FakeSecureStoragePlatform();
    storage = SecureSessionStorage(storage: const FlutterSecureStorage());
  });

  test(
    'build() resolves to a logged-in state carrying the refreshed session, '
    'not the stale one, when the stored access token has already expired',
    () async {
      final stale = _sessionExpiringIn(const Duration(minutes: -5));
      await storage.saveSession(stale);
      final refreshed = _sessionExpiringIn(
        const Duration(minutes: 15),
        accessToken: 'fresh-access-token',
      );

      final container = ProviderContainer(
        overrides: [
          sessionStorageProvider.overrideWithValue(storage),
          tokenRefresherProvider.overrideWithValue(
            TokenRefresher(
              storage: storage,
              refreshSession: (token) async => refreshed,
            ),
          ),
          authServiceProvider.overrideWithValue(
            _FakeAuthService(
              const UserProfile(id: 'user-1', fullName: 'Ada Lovelace'),
            ),
          ),
        ],
      );
      addTearDown(container.dispose);

      final state = await container.read(authSessionProvider.future);

      expect(state.isLoggedIn, isTrue);
      expect(state.session!.accessToken, 'fresh-access-token');
    },
  );

  test('forceSignOut() moves state from logged-in to a logged-out '
      'AuthSessionState', () async {
    final valid = _sessionExpiringIn(const Duration(minutes: 15));
    await storage.saveSession(valid);

    final container = ProviderContainer(
      overrides: [
        sessionStorageProvider.overrideWithValue(storage),
        tokenRefresherProvider.overrideWithValue(
          TokenRefresher(
            storage: storage,
            refreshSession: (token) async =>
                throw StateError('should never be called'),
          ),
        ),
        authServiceProvider.overrideWithValue(
          _FakeAuthService(
            const UserProfile(id: 'user-1', fullName: 'Ada Lovelace'),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    final initial = await container.read(authSessionProvider.future);
    expect(initial.isLoggedIn, isTrue);

    container.read(authSessionProvider.notifier).forceSignOut();

    final after = container.read(authSessionProvider).value;
    expect(after, const AuthSessionState());
    expect(after!.isLoggedIn, isFalse);
  });
}
