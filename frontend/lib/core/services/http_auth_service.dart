import 'dart:async';
import 'dart:convert';

import 'package:app_links/app_links.dart';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:url_launcher/url_launcher.dart';

import 'package:professional_connections_platform/core/config/app_config.dart';
import 'package:professional_connections_platform/core/models/auth_session.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/services/oauth_state.dart';

/// How long to wait for the user to complete the out-of-app LinkedIn login
/// and be redirected back before giving up and treating it as cancelled.
const Duration _redirectTimeout = Duration(minutes: 5);

/// Real [AuthService] wired to the gateway (`backend/PLAN.md` Step 5's REST
/// contract) and LinkedIn's OAuth authorization endpoint directly
/// (`client_id` is public — no backend round trip needed just to start the
/// flow, per `backend/PLAN.md` Step 4).
class HttpAuthService implements AuthService {
  HttpAuthService({
    http.Client? httpClient,
    AppLinks? appLinks,
    String? baseUrl,
    Future<String?> Function()? getAccessToken,
  }) : _httpClient = httpClient ?? http.Client(),
       _appLinks = appLinks ?? AppLinks(),
       _baseUrl = baseUrl ?? AppConfig.gatewayBaseUrl,
       _getAccessToken = getAccessToken ?? (() async => null);

  final http.Client _httpClient;
  final AppLinks _appLinks;
  final String _baseUrl;

  /// Supplies the current access token for the `/v1/verification/*` and
  /// `/v1/users/me` calls below, which the gateway's new auth middleware
  /// requires (backend/PLAN.md's Level 2/3 addendum, Step F). Reads from
  /// wherever the caller's session state actually lives (wired in
  /// `app_providers.dart` to secure storage) rather than this service
  /// holding its own copy — [HttpAuthService] otherwise has no session
  /// state of its own, and this keeps it that way.
  final Future<String?> Function() _getAccessToken;

  @override
  Future<AuthSession> signInWithLinkedIn() async {
    final state = OAuthState.generate();

    // Deliberately no code_challenge/code_challenge_method (PKCE) here —
    // LinkedIn's Sign In with LinkedIn / OpenID Connect product rejects the
    // token exchange outright when they're present (confirmed via direct
    // testing against LinkedIn's real endpoint: identical requests succeed
    // with PKCE omitted, fail with 401 invalid_client when included).
    // Security is still sound without it: this is a confidential client —
    // the LinkedIn client_secret lives only in the backend, never in this
    // app — which is what PKCE exists to substitute for on a public client
    // that can't hold a secret.
    final authorizationUrl =
        Uri.https('www.linkedin.com', '/oauth/v2/authorization', {
          'response_type': 'code',
          'client_id': AppConfig.linkedInClientId,
          'redirect_uri': AppConfig.linkedInRedirectUri,
          'state': state,
          'scope': 'openid profile email',
        });

    // Opens the system browser, not an in-app WebView — a WebView can't be
    // trusted the same way for OAuth (backend/PLAN.md's frontend plan,
    // Step 1).
    final launched = await launchUrl(
      authorizationUrl,
      mode: LaunchMode.externalApplication,
    );
    if (!launched) {
      throw const AuthNetworkException(
        'Could not open the browser to sign in.',
      );
    }

    final redirect = await _awaitRedirect(state);

    if (redirect.queryParameters.containsKey('error')) {
      throw InvalidGrantException(
        redirect.queryParameters['error_description'] ??
            redirect.queryParameters['error']!,
      );
    }

    final returnedState = redirect.queryParameters['state'];
    // CSRF protection: verify state before the authorization code is used
    // for anything else. Belt-and-suspenders — _awaitRedirect already only
    // ever returns a URI whose state matches, but the code below is about
    // to spend an authorization code, so this stays as the explicit final
    // check regardless of how the URI got here.
    if (returnedState == null || returnedState != state) {
      throw const InvalidGrantException(
        'Sign-in response did not match the request that started it.',
      );
    }

    final code = redirect.queryParameters['code'];
    if (code == null || code.isEmpty) {
      throw const InvalidGrantException(
        'LinkedIn did not return an authorization code.',
      );
    }

    return completeLinkedInOnboarding(
      authorizationCode: code,
      redirectUri: AppConfig.linkedInRedirectUri,
    );
  }

  /// The POST-and-parse half of [signInWithLinkedIn], split out so it's
  /// unit-testable (Step 8) without driving the actual out-of-app browser
  /// and deep-link round trip, which isn't automatable in `flutter test`.
  @visibleForTesting
  Future<AuthSession> completeLinkedInOnboarding({
    required String authorizationCode,
    required String redirectUri,
  }) async {
    final response = await _httpClient.post(
      Uri.parse('$_baseUrl/v1/auth/linkedin/callback'),
      headers: const {'Content-Type': 'application/json'},
      body: jsonEncode({
        'authorization_code': authorizationCode,
        'redirect_uri': redirectUri,
      }),
    );
    return _parseSessionResponse(response);
  }

  @override
  Future<AuthSession> refreshSession(String refreshToken) async {
    final response = await _httpClient.post(
      Uri.parse('$_baseUrl/v1/auth/refresh'),
      headers: const {'Content-Type': 'application/json'},
      body: jsonEncode({'refresh_token': refreshToken}),
    );
    return _parseSessionResponse(response, isRefresh: true);
  }

  @override
  Future<void> logout(String refreshToken) async {
    final response = await _httpClient.post(
      Uri.parse('$_baseUrl/v1/auth/logout'),
      headers: const {'Content-Type': 'application/json'},
      body: jsonEncode({'refresh_token': refreshToken}),
    );
    if (response.statusCode != 200) {
      throw AuthNetworkException(_errorMessage(response.body));
    }
  }

  @override
  Future<int> startPhoneVerification(String phoneNumber) => _startVerification(
    '/v1/verification/phone/start',
    {'phone_number': phoneNumber},
  );

  @override
  Future<AuthSession> verifyPhoneCode(String phoneNumber, String code) =>
      _verifyCode('/v1/verification/phone/verify', {
        'phone_number': phoneNumber,
        'code': code,
      });

  @override
  Future<int> startPersonalEmailVerification(String email) =>
      _startVerification('/v1/verification/personal-email/start', {
        'email': email,
      });

  @override
  Future<AuthSession> verifyPersonalEmailCode(String email, String code) =>
      _verifyCode('/v1/verification/personal-email/verify', {
        'email': email,
        'code': code,
      });

  @override
  Future<AuthSession> submitPersonalDetails(
    String legalName,
    String address,
  ) async {
    final response = await _authenticatedPost(
      '/v1/verification/personal-details',
      {'legal_name': legalName, 'address': address},
    );
    return _parseVerificationSession(response);
  }

  @override
  Future<int> startCorporateEmailVerification(String email) =>
      _startVerification('/v1/verification/corporate-email/start', {
        'email': email,
      }, on400: (message) => WorkEmailDomainRejectedException(message));

  @override
  Future<AuthSession> verifyCorporateEmailCode(String email, String code) =>
      _verifyCode('/v1/verification/corporate-email/verify', {
        'email': email,
        'code': code,
      });

  @override
  Future<UserProfile> getProfile() async {
    final headers = await _authHeaders();
    final response = await _httpClient.get(
      Uri.parse('$_baseUrl/v1/users/me'),
      headers: headers,
    );
    if (response.statusCode == 200) {
      return UserProfile.fromJson(
        jsonDecode(response.body) as Map<String, dynamic>,
      );
    }
    throw _mapVerificationError(response, on400: AuthNetworkException.new);
  }

  Future<int> _startVerification(
    String path,
    Map<String, String> body, {
    // A 400 on a Start call means something else entirely from a 400 on a
    // Verify call (there's no code yet to be "invalid") — generic by
    // default; StartCorporateEmailVerification overrides this with the one
    // Start-call-specific case that needs its own exception type.
    AuthException Function(String message) on400 = AuthNetworkException.new,
  }) async {
    final response = await _authenticatedPost(path, body);
    if (response.statusCode == 200) {
      final decoded = jsonDecode(response.body) as Map<String, dynamic>;
      return decoded['resend_after_seconds'] as int? ?? 0;
    }
    throw _mapVerificationError(response, on400: on400);
  }

  Future<AuthSession> _verifyCode(String path, Map<String, String> body) async {
    final response = await _authenticatedPost(path, body);
    return _parseVerificationSession(response);
  }

  AuthSession _parseVerificationSession(http.Response response) {
    if (response.statusCode == 200) {
      return AuthSession.fromJson(
        jsonDecode(response.body) as Map<String, dynamic>,
      );
    }
    throw _mapVerificationError(
      response,
      on400: InvalidVerificationCodeException.new,
    );
  }

  /// Shared status-code mapping for every `/v1/verification/*` and
  /// `/v1/users/me` call. [on400] is the one thing that genuinely differs
  /// per endpoint (a domain-rejection vs. a wrong/expired code) — every
  /// other status means the same thing regardless of which of these
  /// endpoints produced it.
  AuthException _mapVerificationError(
    http.Response response, {
    required AuthException Function(String message) on400,
  }) {
    final message = _errorMessage(response.body);
    switch (response.statusCode) {
      case 400:
        return on400(message);
      case 401:
        // The access token itself was missing/invalid/expired — the
        // gateway's auth middleware rejected the request before it ever
        // reached the auth service. Same "treat the session as gone"
        // handling as a rejected refresh token.
        return SessionExpiredException(message);
      case 429:
        return const ResendCooldownException();
      default:
        return AuthNetworkException(message);
    }
  }

  Future<http.Response> _authenticatedPost(
    String path,
    Map<String, String> body,
  ) async {
    final headers = await _authHeaders();
    return _httpClient.post(
      Uri.parse('$_baseUrl$path'),
      headers: headers,
      body: jsonEncode(body),
    );
  }

  Future<Map<String, String>> _authHeaders() async {
    final token = await _getAccessToken();
    return {
      'Content-Type': 'application/json',
      if (token != null) 'Authorization': 'Bearer $token',
    };
  }

  /// Waits for the deep-link redirect from the LinkedIn browser flow —
  /// checking a cold-start link first (the OS may have relaunched the app),
  /// then the live stream. [expectedState] scopes this to *this* sign-in
  /// attempt: [AppLinks.getInitialLink] reflects the most recent deep link
  /// the OS has delivered, not the one for this specific attempt — on
  /// Android's singleTop launch mode in particular, it keeps returning a
  /// prior attempt's already-consumed redirect until a genuinely new one
  /// arrives, so a retry right after a failed attempt would otherwise be
  /// "completed" by that stale leftover instead of the user's fresh browser
  /// round trip. See [resolveAuthRedirect] for the actual filtering logic.
  Future<Uri> _awaitRedirect(String expectedState) => resolveAuthRedirect(
    initialLink: _appLinks.getInitialLink(),
    redirectStream: _appLinks.uriLinkStream,
    expectedState: expectedState,
    timeout: _redirectTimeout,
  );

  AuthSession _parseSessionResponse(
    http.Response response, {
    bool isRefresh = false,
  }) {
    if (response.statusCode == 200) {
      return AuthSession.fromJson(
        jsonDecode(response.body) as Map<String, dynamic>,
      );
    }

    final message = _errorMessage(response.body);
    switch (response.statusCode) {
      case 400:
        throw InvalidGrantException(message);
      case 429:
        throw const RateLimitedException();
      case 401:
        // Only /v1/auth/refresh returns 401 per the REST contract — an
        // invalid, expired, or already-rotated refresh token.
        throw SessionExpiredException(message);
      default:
        throw AuthNetworkException(message);
    }
  }

  String _errorMessage(String body) {
    try {
      final decoded = jsonDecode(body);
      if (decoded is Map<String, dynamic> && decoded['error'] is String) {
        return decoded['error'] as String;
      }
    } catch (_) {
      // fall through to the generic message below
    }
    return 'Something went wrong. Please try again.';
  }
}

/// Picks the deep-link redirect that actually belongs to the sign-in
/// attempt identified by [expectedState], preferring [initialLink] if it's
/// a match and otherwise waiting on [redirectStream] for one that is.
///
/// Extracted from [HttpAuthService] so this filtering — the fix for a
/// retry-after-failure bug where a stale `getInitialLink()` value from a
/// prior attempt was accepted as the current attempt's response — is
/// testable without the real `app_links` plugin, which is a platform-backed
/// singleton that can't be faked in `flutter test`.
@visibleForTesting
Future<Uri> resolveAuthRedirect({
  required Future<Uri?> initialLink,
  required Stream<Uri> redirectStream,
  required String expectedState,
  required Duration timeout,
}) async {
  final initial = await initialLink;
  if (initial != null && _isAuthRedirectFor(initial, expectedState)) {
    return initial;
  }

  return redirectStream
      .firstWhere((uri) => _isAuthRedirectFor(uri, expectedState))
      .timeout(
        timeout,
        onTimeout: () => throw const SignInCancelledException(),
      );
}

bool _isAuthRedirectFor(Uri uri, String expectedState) =>
    uri.scheme == AppConfig.appRedirectScheme &&
    uri.queryParameters['state'] == expectedState;
