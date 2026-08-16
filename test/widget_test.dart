import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/main.dart';

void main() {
  testWidgets('Onboarding smoke test', (WidgetTester tester) async {
    // Build our app and trigger a frame.
    // We must wrap it in ProviderScope because OnboardingFlow uses Riverpod.
    await tester.pumpWidget(
      const ProviderScope(child: ProfessionalConnectionsApp()),
    );

    // LandingPage (OrbitingIntents) and SplashScreen (CircularProgressIndicator)
    // both run indefinitely-repeating animations, so pumpAndSettle would never
    // converge here — advance time with bounded pumps instead.

    // SplashScreen waits 2s before navigating to LandingPage.
    await tester.pump(const Duration(seconds: 3));
    await tester.pump(const Duration(milliseconds: 400));

    // LandingPage requires tapping "GET STARTED" to reach OnboardingFlow.
    await tester.tap(find.text('GET STARTED'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    // Verify that the onboarding welcome screen builds without crashing.
    expect(find.text('CONTINUE'), findsOneWidget);
  });
}
