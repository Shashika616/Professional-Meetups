import 'package:professional_connections_platform/core/models/auth_session.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';

/// Contract for authentication and Level 2/3 verification (ADR-011,
/// ADR-012, `frontend/PLAN.md`'s matching addendum).
///
/// The client never decides trust. It only displays what the server
/// returns. Every verification-completing method returns a fresh
/// [AuthSession] (new access token reflecting the updated trust level) —
/// callers must save it and update session state immediately, the same way
/// a token refresh already does, rather than waiting for the next natural
/// refresh.
abstract interface class AuthService {
  Future<AuthSession> signInWithLinkedIn();
  Future<AuthSession> refreshSession(String refreshToken);
  Future<void> logout(String refreshToken);

  /// Returns the server's resend cooldown, in seconds — the client's own
  /// countdown timer is seeded from this, never hardcoded, since it's the
  /// server that actually enforces it.
  Future<int> startPhoneVerification(String phoneNumber);
  Future<AuthSession> verifyPhoneCode(String phoneNumber, String code);
  Future<int> startPersonalEmailVerification(String email);
  Future<AuthSession> verifyPersonalEmailCode(String email, String code);
  Future<AuthSession> submitPersonalDetails(String legalName, String address);
  Future<int> startCorporateEmailVerification(String email);
  Future<AuthSession> verifyCorporateEmailCode(String email, String code);

  /// Never returns a raw phone number or email address — only
  /// booleans/derived fields (backend's deliberate choice, Verification
  /// Model § 1).
  Future<UserProfile> getProfile();
}

/// Typed errors an [AuthService] can throw, so the UI can show a real
/// message instead of a generic "something went wrong" — in particular a
/// 429 must read as "you're being rate limited," not a generic failure.
sealed class AuthException implements Exception {
  const AuthException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// 400 from the gateway: invalid/expired authorization code, or a PKCE
/// verifier/challenge mismatch.
class InvalidGrantException extends AuthException {
  const InvalidGrantException(super.message);
}

/// 429 from the gateway — distinct from other failures so the UI can tell
/// the user to wait, specifically.
class RateLimitedException extends AuthException {
  const RateLimitedException([
    super.message =
        'You’re being rate limited. Please wait a moment and try again.',
  ]);
}

/// 401 on `/v1/auth/refresh` — the refresh token is invalid, expired, or
/// was already rotated (a possible theft signal server-side, ADR-009). The
/// client's only correct response is to treat the local session as gone.
class SessionExpiredException extends AuthException {
  const SessionExpiredException(super.message);
}

/// The user backed out of the LinkedIn browser flow, or it never completed
/// within the timeout — not a server error, nothing to retry against.
class SignInCancelledException extends AuthException {
  const SignInCancelledException([
    super.message = 'Sign-in was not completed.',
  ]);
}

/// A `StartXVerification` resend attempted before the server's cooldown
/// has elapsed (429, backend/PLAN.md's addendum Step G) — distinct from
/// [RateLimitedException] because the UI response differs: this isn't a
/// generic "you're being rate limited" failure to show the user, it means
/// the client's own countdown timer is out of sync with the server's and
/// should just keep counting down rather than surfacing an error.
class ResendCooldownException extends AuthException {
  const ResendCooldownException([
    super.message = 'Please wait before requesting another code.',
  ]);
}

/// `StartCorporateEmailVerification` rejected a free-mail or role-based
/// address (400) — safe to show verbatim (Verification Model § 5 — this
/// only reveals something about the domain the user themselves just typed,
/// not about any account's existence), matches the backend's exact message.
class WorkEmailDomainRejectedException extends AuthException {
  const WorkEmailDomainRejectedException(super.message);
}

/// A `VerifyXCode` call failed (400) — wrong code, expired code, or the
/// attempt cap was hit. Distinct from [InvalidGrantException], which is
/// LinkedIn-specific vocabulary.
class InvalidVerificationCodeException extends AuthException {
  const InvalidVerificationCodeException(super.message);
}

/// Anything else: network failure, unexpected status code, malformed
/// response.
class AuthNetworkException extends AuthException {
  const AuthNetworkException([
    super.message = 'Something went wrong. Please try again.',
  ]);
}

/// Simulates server latency for widget tests (Step 8) — kept in the
/// codebase for that purpose even though [HttpAuthService] is what
/// `app_providers.dart` wires up for real use.
class MockAuthService implements AuthService {
  static const Duration latency = Duration(milliseconds: 600);

  @override
  Future<AuthSession> signInWithLinkedIn() async {
    await Future<void>.delayed(latency);
    return AuthSession(
      userId: 'mock-user-1',
      accessToken: 'mock-access-token',
      refreshToken: 'mock-refresh-token',
      trustLevel: 1,
      isNewUser: true,
      accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
      fullName: 'Mock User',
      profilePhotoUrl: '',
    );
  }

  @override
  Future<AuthSession> refreshSession(String refreshToken) async {
    await Future<void>.delayed(latency);
    return AuthSession(
      userId: 'mock-user-1',
      accessToken: 'mock-access-token-2',
      refreshToken: 'mock-refresh-token-2',
      trustLevel: 1,
      isNewUser: false,
      accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
      fullName: 'Mock User',
      profilePhotoUrl: '',
    );
  }

  @override
  Future<void> logout(String refreshToken) async {
    await Future<void>.delayed(latency);
  }

  Future<AuthSession> _mockVerifiedSession() async {
    await Future<void>.delayed(latency);
    return AuthSession(
      userId: 'mock-user-1',
      accessToken: 'mock-access-token-verified',
      refreshToken: 'mock-refresh-token-verified',
      trustLevel: 1,
      isNewUser: false,
      accessTokenExpiresAt: DateTime.now().add(const Duration(minutes: 15)),
      fullName: 'Mock User',
      profilePhotoUrl: '',
    );
  }

  @override
  Future<int> startPhoneVerification(String phoneNumber) async {
    await Future<void>.delayed(latency);
    return 60;
  }

  @override
  Future<AuthSession> verifyPhoneCode(String phoneNumber, String code) =>
      _mockVerifiedSession();

  @override
  Future<int> startPersonalEmailVerification(String email) async {
    await Future<void>.delayed(latency);
    return 60;
  }

  @override
  Future<AuthSession> verifyPersonalEmailCode(String email, String code) =>
      _mockVerifiedSession();

  @override
  Future<AuthSession> submitPersonalDetails(String legalName, String address) =>
      _mockVerifiedSession();

  @override
  Future<int> startCorporateEmailVerification(String email) async {
    await Future<void>.delayed(latency);
    return 60;
  }

  @override
  Future<AuthSession> verifyCorporateEmailCode(String email, String code) =>
      _mockVerifiedSession();

  @override
  Future<UserProfile> getProfile() async {
    await Future<void>.delayed(latency);
    return const UserProfile(id: 'mock-user-1', fullName: 'Mock User');
  }
}
