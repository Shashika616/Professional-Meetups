import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod/legacy.dart' show StateProvider;

import 'package:professional_connections_platform/core/models/auth_session.dart';
import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/match_profile.dart';
import 'package:professional_connections_platform/core/models/paged_result.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/services/http_auth_service.dart';
import 'package:professional_connections_platform/core/services/matching_service.dart';
import 'package:professional_connections_platform/core/services/token_refresher.dart';
import 'package:professional_connections_platform/core/storage/session_storage.dart';

final sessionStorageProvider = Provider<SecureSessionStorage>(
  (ref) => SecureSessionStorage(),
);

// TokenRefresher needs HttpAuthService's own refreshSession() method, and
// HttpAuthService needs TokenRefresher.getValidAccessToken as its
// getAccessToken callback — a naive circular dependency if built as two
// independent providers. Both are constructed once here, in a single
// provider body (the `late final service` trick), and authServiceProvider/
// tokenRefresherProvider below just proxy into this shared bundle — never
// authSessionProvider, which this bundle must stay independent of (see the
// note on _AuthBundle itself).
final _authBundleProvider = Provider<_AuthBundle>((ref) {
  final storage = ref.read(sessionStorageProvider);
  late final HttpAuthService service;
  final refresher = TokenRefresher(
    storage: storage,
    refreshSession: (token) => service.refreshSession(token),
  );
  service = HttpAuthService(getAccessToken: refresher.getValidAccessToken);
  return _AuthBundle(service: service, refresher: refresher);
});

// Never reads authSessionProvider — HttpAuthService's verification/profile
// calls are invoked from inside AuthSessionNotifier itself (e.g. build()'s
// own getValidSession() call below), and ref.read(authSessionProvider) from
// within that provider's own build() would be a self-referential read on a
// provider that's still resolving. TokenRefresher-backed secure storage is
// the actual source of truth for "what's the current token" either way —
// AuthSessionNotifier keeps it in sync via saveSession()/completeVerification
// on every state change.
final authServiceProvider = Provider<AuthService>(
  (ref) => ref.read(_authBundleProvider).service,
);

final tokenRefresherProvider = Provider<TokenRefresher>(
  (ref) => ref.read(_authBundleProvider).refresher,
);

class _AuthBundle {
  const _AuthBundle({required this.service, required this.refresher});

  final HttpAuthService service;
  final TokenRefresher refresher;
}

final matchingServiceProvider = Provider<MatchingService>(
  (ref) => MockMatchingService(),
);

final selectedIntentProvider = StateProvider<IntentType>(
  (ref) => IntentType.coffee,
);

final matchesProvider = FutureProvider.autoDispose
    .family<PagedResult<MatchProfile>, IntentType>(
      (ref, intent) =>
          ref.watch(matchingServiceProvider).fetchMatches(intent: intent),
    );

final homeStatsProvider = FutureProvider.autoDispose<Map<String, dynamic>>((
  ref,
) async {
  await Future.delayed(const Duration(seconds: 1));
  return {'nearby': 128, 'meetups': 12, 'trustScore': 4.9};
});

/// Whether the app has a persisted session, and the profile cached from it.
/// Nothing tracked a logged-in user across app restarts before this —
/// without it, every relaunch would force a fresh LinkedIn login, defeating
/// the point of a 30-day refresh token.
class AuthSessionState {
  const AuthSessionState({this.session, this.profile});

  final AuthSession? session;
  final UserProfile? profile;

  /// A stored refresh token is the actual "logged in" signal — the short
  /// (15 min) access token being expired doesn't mean the session is gone,
  /// it means the next authenticated call should refresh first.
  bool get isLoggedIn => session != null;
}

class AuthSessionNotifier extends AsyncNotifier<AuthSessionState> {
  @override
  Future<AuthSessionState> build() async {
    // getValidSession() refreshes-and-persists first if the stored access
    // token is expired or about to be — a secure-storage read failure, an
    // already-dead refresh token, or a transient network error during that
    // refresh are all treated alike as "no session," same as before this
    // addendum: forcing a fresh LinkedIn login is the safe fallback, not a
    // hang. This is also why an idle-but-not-force-quit session (backgrounded
    // past the 15-minute access-token TTL but well within the 30-day
    // refresh-token life) now correctly comes back logged in on relaunch
    // instead of being forced to sign in again.
    try {
      final session = await ref.read(tokenRefresherProvider).getValidSession();
      if (session == null) return const AuthSessionState();
      return AuthSessionState(
        session: session,
        profile: await _fetchProfileOrFallback(session),
      );
    } catch (_) {
      return const AuthSessionState();
    }
  }

  Future<void> signInWithLinkedIn() async {
    state = const AsyncLoading();
    try {
      final session = await ref.read(authServiceProvider).signInWithLinkedIn();
      await ref.read(sessionStorageProvider).saveSession(session);
      state = AsyncData(
        AuthSessionState(
          session: session,
          profile: await _fetchProfileOrFallback(session),
        ),
      );
    } catch (error, stackTrace) {
      // Re-thrown (unlike AsyncValue.guard, which would swallow it) so the
      // caller — OnboardingFlow — can show the specific mapped error
      // message; state still reflects the failure for anything else
      // watching this provider.
      state = AsyncError(error, stackTrace);
      rethrow;
    }
  }

  /// Called after any verification-completing [AuthService] call succeeds
  /// (`verifyPhoneCode`, `submitPersonalDetails`, etc.) — saves the fresh
  /// session immediately (so the new trust level is live without waiting
  /// for the next natural token refresh) *and* re-fetches the full
  /// profile, so `UserProfile`'s booleans move together with `AuthSession`
  /// rather than drifting: without this, `ProfilePage`'s Phone row could
  /// still say "Not verified" immediately after phone verification
  /// actually succeeded, right after the trust-level badge elsewhere
  /// already updated (`frontend/PLAN.md`'s Level 2/3 addendum, Step 2).
  Future<void> completeVerification(AuthSession session) async {
    await ref.read(sessionStorageProvider).saveSession(session);
    state = AsyncData(
      AuthSessionState(
        session: session,
        profile: await _fetchProfileOrFallback(session),
      ),
    );
  }

  /// Clears the local session regardless of whether the network call
  /// succeeds — a failed logout call to the backend shouldn't leave the
  /// user stuck signed in locally, same idempotent-logout spirit as the
  /// backend's `/v1/auth/logout` (`frontend/PLAN.md` Step 7).
  Future<void> signOut() async {
    // Ensure build() has actually resolved before reading state.value —
    // ProfilePage never reads/watches this provider itself (only this
    // method does, lazily, on tap), so without this a sign-out fired before
    // the notifier's first build completes would see a stale/absent
    // session, silently skip the backend logout() call, and leave a live,
    // unrevoked refresh token server-side while the local session already
    // looks signed out. In the real app SplashScreen always reads this
    // provider first and primes it long before ProfilePage is reachable,
    // but this shouldn't depend on that navigation ordering to be correct.
    await future;
    final refreshToken = state.value?.session?.refreshToken;
    if (refreshToken != null) {
      try {
        await ref.read(authServiceProvider).logout(refreshToken);
      } catch (_) {
        // Ignored on purpose — see doc comment above.
      }
    }
    await ref.read(sessionStorageProvider).clearSession();
    state = const AsyncData(AuthSessionState());
  }

  /// Called when a caller catches `SessionExpiredException` from an
  /// authenticated call made mid-session (not during build()/launch, which
  /// already handles this itself via getValidSession()'s own catch) —
  /// TokenRefresher has already cleared storage by the time this runs
  /// (whatever call failed went through its getValidAccessToken), so this
  /// only needs to bring in-memory state into agreement with it; Riverpod
  /// doesn't notice a storage write it wasn't watching for on its own.
  /// Unlike [signOut], this must NOT call authService.logout() — the
  /// refresh token that call would send is already dead server-side
  /// (that's why we're here), and storage no longer has it to send anyway.
  void forceSignOut() {
    state = const AsyncData(AuthSessionState());
  }

  /// Tries a real `getProfile()` fetch; falls back to the minimal profile
  /// derivable from [session] alone (the pre-addendum behavior) if the
  /// fetch fails — e.g. offline right at launch. A transient failure here
  /// must not crash session loading; the next successful fetch catches up.
  Future<UserProfile> _fetchProfileOrFallback(AuthSession session) async {
    try {
      return await ref.read(authServiceProvider).getProfile();
    } catch (_) {
      return UserProfile(
        id: session.userId,
        fullName: session.fullName,
        profilePhotoUrl: session.profilePhotoUrl,
        trustLevel: session.trustLevel,
      );
    }
  }
}

final authSessionProvider =
    AsyncNotifierProvider<AuthSessionNotifier, AuthSessionState>(
      AuthSessionNotifier.new,
    );
