import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/verification/widgets/otp_entry.dart';

Widget _appWith({
  required int initialResendAfterSeconds,
  required Future<void> Function(String code) onSubmit,
  Future<int> Function()? onResend,
}) {
  return MaterialApp(
    home: Scaffold(
      body: OtpEntry(
        initialResendAfterSeconds: initialResendAfterSeconds,
        onSubmit: onSubmit,
        onResend: onResend ?? () async => 60,
      ),
    ),
  );
}

void main() {
  testWidgets('countdown seeded from initialResendAfterSeconds counts down '
      'and enables resend at zero', (tester) async {
    await tester.pumpWidget(
      _appWith(initialResendAfterSeconds: 3, onSubmit: (_) async {}),
    );

    expect(find.text('Resend code in 3s'), findsOneWidget);

    await tester.pump(const Duration(seconds: 1));
    expect(find.text('Resend code in 2s'), findsOneWidget);

    await tester.pump(const Duration(seconds: 1));
    expect(find.text('Resend code in 1s'), findsOneWidget);

    await tester.pump(const Duration(seconds: 1));
    expect(find.text('Resend code'), findsOneWidget);
    expect(find.text('Resend code in 0s'), findsNothing);
  });

  testWidgets('VERIFY stays disabled until all 6 digits are entered, then '
      'submits the code', (tester) async {
    String? submitted;
    await tester.pumpWidget(
      _appWith(
        initialResendAfterSeconds: 60,
        onSubmit: (code) async => submitted = code,
      ),
    );

    await tester.enterText(find.byType(TextField), '123');
    await tester.pump();
    await tester.tap(find.byType(GradientButton));
    await tester.pump();
    expect(submitted, isNull); // 3 digits — GradientButton ignored the tap.

    await tester.enterText(find.byType(TextField), '123456');
    await tester.pump();
    await tester.tap(find.byType(GradientButton));
    await tester.pump();
    await tester.pump();

    expect(submitted, '123456');
  });

  testWidgets(
    'a wrong code shows the error message but does not reset the timer',
    (tester) async {
      await tester.pumpWidget(
        _appWith(
          initialResendAfterSeconds: 30,
          onSubmit: (_) async =>
              throw const InvalidVerificationCodeException('Wrong code.'),
        ),
      );

      await tester.pump(const Duration(seconds: 5));
      expect(find.text('Resend code in 25s'), findsOneWidget);

      await tester.enterText(find.byType(TextField), '000000');
      await tester.pump();
      await tester.tap(find.byType(GradientButton));
      await tester.pump();
      await tester.pump();

      expect(find.text('Wrong code.'), findsOneWidget);
      // The countdown kept running through the failed attempt — no free
      // reset for a wrong guess (frontend/PLAN.md's addendum, Step 4).
      expect(find.text('Resend code in 25s'), findsOneWidget);
    },
  );

  testWidgets('resend before the cooldown elapses is a no-op; after it '
      'elapses it reseeds the timer from the server value', (tester) async {
    var resendCalls = 0;
    await tester.pumpWidget(
      _appWith(
        initialResendAfterSeconds: 2,
        onSubmit: (_) async {},
        onResend: () async {
          resendCalls++;
          return 45;
        },
      ),
    );

    // Cooldown still active — tapping "Resend code in 2s" must not fire.
    await tester.tap(
      find.textContaining('Resend code in'),
      warnIfMissed: false,
    );
    await tester.pump();
    expect(resendCalls, 0);

    await tester.pump(const Duration(seconds: 2));
    expect(find.text('Resend code'), findsOneWidget);

    await tester.tap(find.text('Resend code'));
    await tester.pump();
    await tester.pump();

    expect(resendCalls, 1);
    expect(find.text('Resend code in 45s'), findsOneWidget);
  });
}
