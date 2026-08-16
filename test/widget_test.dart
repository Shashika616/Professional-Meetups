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
    
    // Verify that the onboarding welcome screen builds without crashing.
    expect(find.text('CONTINUE'), findsOneWidget);
  });
}