import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/features/meetups/widgets/ios_map_location_step.dart';

const _channel = MethodChannel('professionalconnections/ios_local_search');

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(_channel, null);
  });

  testWidgets('debounces: several rapid keystrokes collapse into exactly one '
      'autocomplete call', (tester) async {
    var autocompleteCalls = 0;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(_channel, (call) async {
          if (call.method == 'autocomplete') {
            autocompleteCalls++;
            return [
              {'title': 'The Coffee Shop', 'subtitle': 'Colombo'},
            ];
          }
          return null;
        });

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: IosMapLocationStep(onSubmit: (_, _, _) {})),
      ),
    );
    await tester.pump();

    final field = find.widgetWithText(
      TextField,
      'Search for a cafe, restaurant, or venue',
    );
    for (final partial in ['c', 'co', 'cof', 'coff', 'coffee']) {
      await tester.enterText(field, partial);
      await tester.pump(const Duration(milliseconds: 50));
    }
    expect(
      autocompleteCalls,
      0,
      reason: 'no request before the debounce settles',
    );

    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();

    expect(
      autocompleteCalls,
      1,
      reason: 'one debounce-settled request, not one per keystroke',
    );
  });

  testWidgets(
    'selecting a completion resolves it, fills the field, recenters, and '
    'CONTINUE submits those coordinates',
    (tester) async {
      double? submittedLat;
      double? submittedLng;
      String? submittedLabel;
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(_channel, (call) async {
            switch (call.method) {
              case 'autocomplete':
                return [
                  {'title': 'The Coffee Shop', 'subtitle': 'Colombo'},
                ];
              case 'resolveCompletion':
                return {
                  'lat': 6.9213,
                  'lon': 79.8756,
                  'label': 'The Coffee Shop, Colombo',
                };
              default:
                return null;
            }
          });

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: IosMapLocationStep(
              onSubmit: (lat, lng, label) {
                submittedLat = lat;
                submittedLng = lng;
                submittedLabel = label;
              },
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

      expect(find.text('The Coffee Shop, Colombo'), findsOneWidget);
      await tester.tap(find.text('The Coffee Shop, Colombo'));
      await tester.pump();
      await tester.pump();

      await tester.tap(find.text('CONTINUE'));
      await tester.pump();

      expect(submittedLat, closeTo(6.9213, 0.0001));
      expect(submittedLng, closeTo(79.8756, 0.0001));
      expect(submittedLabel, 'The Coffee Shop, Colombo');
    },
  );

  testWidgets(
    'typing a full query and pressing search submits directly, without '
    'picking a completion, and CONTINUE submits those coordinates',
    (tester) async {
      double? submittedLat;
      double? submittedLng;
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(_channel, (call) async {
            if (call.method == 'search') {
              return {
                'lat': 6.9172,
                'lon': 79.8634,
                'label': 'Department of Coffee',
              };
            }
            return null;
          });

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: IosMapLocationStep(
              onSubmit: (lat, lng, _) {
                submittedLat = lat;
                submittedLng = lng;
              },
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
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(_channel, (call) async {
            if (call.method == 'autocomplete') {
              return [
                {'title': 'The Coffee Shop', 'subtitle': 'Colombo'},
              ];
            }
            return null;
          });

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: IosMapLocationStep(onSubmit: (_, _, _) => submitted = true),
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

      expect(find.text('The Coffee Shop, Colombo'), findsOneWidget);
      // CONTINUE is enabled (the field has text) but sits underneath the
      // scrim while the dropdown is open — before the scrim, this tap
      // reached CONTINUE directly, which is exactly the bug being fixed
      // (submitting instead of picking a completion).
      await tester.tap(find.text('CONTINUE'), warnIfMissed: false);
      await tester.pump();

      expect(
        submitted,
        false,
        reason: 'the tap should hit the scrim, not CONTINUE underneath it',
      );
      expect(
        find.text('The Coffee Shop, Colombo'),
        findsNothing,
        reason: 'tapping the scrim dismisses the dropdown',
      );
    },
  );

  testWidgets(
    'runs with no configuration at all — no API key, no AppConfig entry '
    'needed on iOS',
    (tester) async {
      // No debugStadiaApiKeyOverride, no AppConfig.stadiaMapsApiKey set
      // anywhere in this test — this file never even imports AppConfig,
      // confirming the iOS path has no such dependency (frontend/meetup-
      // scheduling-PLAN.md's 2026-08-18 platform-split addendum's own
      // self-review requirement).
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(_channel, (call) async => null);

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: IosMapLocationStep(onSubmit: (_, _, _) {})),
        ),
      );
      await tester.pump();

      expect(find.text('Where?'), findsOneWidget);
      expect(find.text('CONTINUE'), findsOneWidget);
      expect(find.textContaining('not configured'), findsNothing);
    },
  );
}
