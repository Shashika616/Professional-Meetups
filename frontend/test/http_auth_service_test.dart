import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/services/http_auth_service.dart';

const _baseUrl = 'http://localhost:8080';

HttpAuthService _serviceWith(http.Client client) =>
    HttpAuthService(httpClient: client, baseUrl: _baseUrl);

void main() {
  group('completeLinkedInOnboarding', () {
    test('200 parses into an AuthSession', () async {
      final client = MockClient((request) async {
        expect(request.url.toString(), '$_baseUrl/v1/auth/linkedin/callback');
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body['authorization_code'], 'code');
        expect(body.containsKey('code_verifier'), isFalse);
        expect(body['redirect_uri'], 'app://callback');

        return http.Response(
          jsonEncode({
            'user_id': 'user-1',
            'access_token': 'a.b.c',
            'refresh_token': 'refresh-token',
            'expires_in': 900,
            'is_new_user': true,
            'full_name': 'Ada Lovelace',
            'profile_photo_url': 'https://example.com/p.jpg',
          }),
          200,
        );
      });

      final session = await _serviceWith(client).completeLinkedInOnboarding(
        authorizationCode: 'code',
        redirectUri: 'app://callback',
      );

      expect(session.userId, 'user-1');
      expect(session.fullName, 'Ada Lovelace');
    });

    test('400 throws InvalidGrantException with the server message', () async {
      final client = MockClient(
        (request) async => http.Response(
          jsonEncode({'error': 'linkedin rejected the code'}),
          400,
        ),
      );

      expect(
        () => _serviceWith(client).completeLinkedInOnboarding(
          authorizationCode: 'bad',
          redirectUri: 'app://callback',
        ),
        throwsA(
          isA<InvalidGrantException>().having(
            (e) => e.message,
            'message',
            'linkedin rejected the code',
          ),
        ),
      );
    });

    test('429 throws RateLimitedException, not a generic error', () async {
      final client = MockClient(
        (request) async =>
            http.Response(jsonEncode({'error': 'rate limited'}), 429),
      );

      expect(
        () => _serviceWith(client).completeLinkedInOnboarding(
          authorizationCode: 'code',
          redirectUri: 'app://callback',
        ),
        throwsA(isA<RateLimitedException>()),
      );
    });
  });

  group('refreshSession', () {
    test('401 throws SessionExpiredException', () async {
      final client = MockClient(
        (request) async => http.Response(
          jsonEncode({'error': 'refresh token already used'}),
          401,
        ),
      );

      expect(
        () => _serviceWith(client).refreshSession('old-token'),
        throwsA(isA<SessionExpiredException>()),
      );
    });
  });

  group('logout', () {
    test('200 completes without throwing', () async {
      final client = MockClient(
        (request) async => http.Response(jsonEncode({'success': true}), 200),
      );

      await expectLater(
        _serviceWith(client).logout('refresh-token'),
        completes,
      );
    });

    test('non-200 throws AuthNetworkException', () async {
      final client = MockClient((request) async => http.Response('', 500));

      expect(
        () => _serviceWith(client).logout('refresh-token'),
        throwsA(isA<AuthNetworkException>()),
      );
    });
  });

  group('resolveAuthRedirect', () {
    // Regression test for a retry-after-failure bug seen in real device
    // testing: AppLinks.getInitialLink() returns the most recently
    // delivered deep link, not a one-shot value scoped to the current
    // sign-in attempt — on a second attempt right after a failed first one,
    // it can hand back the first attempt's already-consumed redirect
    // (wrong, stale state) before the user has even completed the retry's
    // browser round trip. resolveAuthRedirect must ignore that and wait for
    // a redirect matching *this* attempt's state instead.
    test('a stale initial link from a prior attempt is ignored in favor of '
        'the live stream\'s matching redirect', () async {
      final staleFromFirstAttempt = Uri.parse(
        'professionalconnections://auth/linkedin/callback'
        '?code=first-attempt-code&state=first-attempt-state',
      );
      final freshFromSecondAttempt = Uri.parse(
        'professionalconnections://auth/linkedin/callback'
        '?code=second-attempt-code&state=second-attempt-state',
      );

      final result = await resolveAuthRedirect(
        initialLink: Future.value(staleFromFirstAttempt),
        redirectStream: Stream.value(freshFromSecondAttempt),
        expectedState: 'second-attempt-state',
        timeout: const Duration(seconds: 1),
      );

      expect(result, freshFromSecondAttempt);
    });

    test('an initial link matching this attempt\'s state is returned without '
        'waiting on the stream', () async {
      final matching = Uri.parse(
        'professionalconnections://auth/linkedin/callback'
        '?code=code&state=expected-state',
      );

      final result = await resolveAuthRedirect(
        initialLink: Future.value(matching),
        // Never emits — proves the initial link was used directly rather
        // than waiting on this stream.
        redirectStream: const Stream.empty(),
        expectedState: 'expected-state',
        timeout: const Duration(seconds: 1),
      );

      expect(result, matching);
    });

    test('no initial link and no matching stream event times out as '
        'cancelled', () async {
      // A never-closed controller mirrors AppLinks.uriLinkStream's real
      // behavior — it stays open for the app's lifetime rather than
      // completing — so this actually exercises the timeout path instead
      // of firstWhere's "no element" error on an already-closed stream.
      final controller = StreamController<Uri>();
      addTearDown(controller.close);

      await expectLater(
        resolveAuthRedirect(
          initialLink: Future.value(null),
          redirectStream: controller.stream,
          expectedState: 'expected-state',
          timeout: const Duration(milliseconds: 50),
        ),
        throwsA(isA<SignInCancelledException>()),
      );
    });
  });
}
