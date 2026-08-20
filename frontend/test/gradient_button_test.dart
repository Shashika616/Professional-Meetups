import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/widgets/gradient_button.dart';

void main() {
  group('GradientButton overflow regression (ADR-016)', () {
    // Reproduces the exact reported RenderFlex-overflowed-by-16-pixels bug:
    // a bold, letter-spaced label ("IT HAPPENED") inside a GradientButton,
    // half-width in a Row on an iPhone-SE-class (~375 logical px) device —
    // the real layout meetup_detail_page.dart's "How did it go?" section
    // uses (GradientButton + a plain OutlinedButton, each Expanded, side by
    // side), not two GradientButtons — the fix is at the GradientButton
    // widget level regardless, since the overflow originates inside its own
    // Row/Text, not from what sits next to it.
    testWidgets('a long label on a narrow device does not overflow', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(375, 812);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: Padding(
              padding: EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: GradientButton(
                      label: 'IT HAPPENED',
                      height: 42,
                      onPressed: null,
                    ),
                  ),
                  SizedBox(width: 10),
                  Expanded(
                    child: GradientButton(
                      label: "DIDN'T HAPPEN",
                      height: 42,
                      onPressed: null,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'the label still renders (ellipsized, not hidden) when it does not fit',
      (tester) async {
        tester.view.physicalSize = const Size(200, 600);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        await tester.pumpWidget(
          const MaterialApp(
            home: Scaffold(
              body: Row(
                children: [
                  Expanded(
                    child: GradientButton(
                      label: 'A SIGNIFICANTLY LONGER LABEL THAN USUAL',
                      height: 42,
                      onPressed: null,
                    ),
                  ),
                  Expanded(
                    child: GradientButton(
                      label: 'ANOTHER LONG LABEL HERE TOO',
                      height: 42,
                      onPressed: null,
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byType(Text), findsWidgets);
      },
    );
  });
}
