import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:professional_connections_platform/features/meetups/widgets/stadia_map_location_step.dart';

/// A minimal GeoJSON FeatureCollection matching Stadia's real response
/// shape (confirmed directly against a live request during this
/// addendum's bug diagnosis — see TESTING-NOTES.md).
String _featureCollection(List<(String label, double lat, double lon)> places) {
  final features = places
      .map(
        (p) =>
            '{"type":"Feature","geometry":{"type":"Point","coordinates":[${p.$3},${p.$2}]},'
            '"properties":{"label":"${p.$1}"}}',
      )
      .join(',');
  return '{"type":"FeatureCollection","features":[$features]}';
}

void main() {
  setUp(() => debugStadiaApiKeyOverride = 'test-key');
  tearDown(() => debugStadiaApiKeyOverride = null);

  testWidgets(
    'debounces: several rapid keystrokes collapse into exactly one request',
    (tester) async {
      var requestCount = 0;
      final client = MockClient((request) async {
        requestCount++;
        return http.Response(
          _featureCollection([('The Coffee Shop', 6.9213, 79.8756)]),
          200,
        );
      });

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: StadiaMapLocationStep(
              onSubmit: (_, _, _) {},
              httpClient: client,
            ),
          ),
        ),
      );
      await tester.pump();

      final field = find.widgetWithText(
        TextField,
        'Search for a cafe, restaurant, or venue',
      );
      // Five keystrokes, each only a single frame apart — well within the
      // 300ms debounce window, so none of them should fire their own
      // request.
      for (final partial in ['c', 'co', 'cof', 'coff', 'coffee']) {
        await tester.enterText(field, partial);
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(requestCount, 0, reason: 'no request before the debounce settles');

      // Let the debounce window elapse.
      await tester.pump(const Duration(milliseconds: 300));
      await tester.pump();

      expect(
        requestCount,
        1,
        reason: 'one debounce-settled request, not one per keystroke',
      );
    },
  );

  testWidgets('selecting a suggestion fills the field, recenters, and CONTINUE '
      'submits those coordinates', (tester) async {
    double? submittedLat;
    double? submittedLng;
    String? submittedLabel;
    final client = MockClient((request) async {
      return http.Response(
        _featureCollection([('The Coffee Shop', 6.9213, 79.8756)]),
        200,
      );
    });

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StadiaMapLocationStep(
            onSubmit: (lat, lng, label) {
              submittedLat = lat;
              submittedLng = lng;
              submittedLabel = label;
            },
            httpClient: client,
          ),
        ),
      ),
    );
    await tester.pump();

    await tester.enterText(
      find.widgetWithText(TextField, 'Search for a cafe, restaurant, or venue'),
      'coffee',
    );
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();

    expect(find.text('The Coffee Shop'), findsOneWidget);
    await tester.tap(find.text('The Coffee Shop'));
    await tester.pump();
    await tester.pump();

    await tester.tap(find.text('CONTINUE'));
    await tester.pump();

    expect(submittedLat, closeTo(6.9213, 0.0001));
    expect(submittedLng, closeTo(79.8756, 0.0001));
    expect(submittedLabel, 'The Coffee Shop');
  });

  testWidgets(
    'typing a full query and pressing search submits directly, without '
    'picking a suggestion, and CONTINUE submits those coordinates',
    (tester) async {
      double? submittedLat;
      double? submittedLng;
      final client = MockClient((request) async {
        expect(request.url.path, '/geocoding/v1/search');
        return http.Response(
          _featureCollection([('Department of Coffee', 6.9172, 79.8634)]),
          200,
        );
      });

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: StadiaMapLocationStep(
              onSubmit: (lat, lng, _) {
                submittedLat = lat;
                submittedLng = lng;
              },
              httpClient: client,
            ),
          ),
        ),
      );
      await tester.pump();

      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.enterText(
        find.widgetWithText(
          TextField,
          'Search for a cafe, restaurant, or venue',
        ),
        'Department of Coffee',
      );
      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.pump();
      await tester.pump();

      expect(find.text('Department of Coffee'), findsWidgets);

      await tester.tap(find.text('CONTINUE'));
      await tester.pump();

      expect(submittedLat, closeTo(6.9172, 0.0001));
      expect(submittedLng, closeTo(79.8634, 0.0001));
    },
  );

  testWidgets(
    'the scrim behind an open dropdown blocks taps to CONTINUE and closes '
    'the dropdown instead of submitting',
    (tester) async {
      var submitted = false;
      final client = MockClient((request) async {
        return http.Response(
          _featureCollection([('The Coffee Shop', 6.9213, 79.8756)]),
          200,
        );
      });

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: StadiaMapLocationStep(
              onSubmit: (_, _, _) => submitted = true,
              httpClient: client,
            ),
          ),
        ),
      );
      await tester.pump();

      await tester.enterText(
        find.widgetWithText(
          TextField,
          'Search for a cafe, restaurant, or venue',
        ),
        'coffee',
      );
      await tester.pump(const Duration(milliseconds: 300));
      await tester.pump();

      expect(find.text('The Coffee Shop'), findsOneWidget);
      // CONTINUE is enabled (the field has text) but sits underneath the
      // scrim while the dropdown is open — before the scrim, this tap
      // reached CONTINUE directly, which is exactly the bug being fixed
      // (submitting instead of picking a suggestion).
      await tester.tap(find.text('CONTINUE'), warnIfMissed: false);
      await tester.pump();

      expect(
        submitted,
        false,
        reason: 'the tap should hit the scrim, not CONTINUE underneath it',
      );
      expect(
        find.text('The Coffee Shop'),
        findsNothing,
        reason: 'tapping the scrim dismisses the dropdown',
      );
    },
  );

  testWidgets('a non-200 response leaves the suggestions list empty', (
    tester,
  ) async {
    final client = MockClient((request) async => http.Response('', 401));

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StadiaMapLocationStep(
            onSubmit: (_, _, _) {},
            httpClient: client,
          ),
        ),
      ),
    );
    await tester.pump();

    await tester.enterText(
      find.widgetWithText(TextField, 'Search for a cafe, restaurant, or venue'),
      'coffee',
    );
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();

    expect(find.byType(ListTile), findsNothing);
  });
}
