import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/verification/widgets/otp_entry.dart';
import 'package:professional_connections_platform/features/verification/widgets/verification_scaffold.dart';

/// Personal email entry → shared OTP widget. Reachable both from the
/// post-LinkedIn onboarding sequence and independently from `ProfilePage`
/// (`frontend/PLAN.md`'s Level 2/3 addendum, Step 6).
class PersonalEmailVerificationPage extends ConsumerStatefulWidget {
  const PersonalEmailVerificationPage({super.key});

  @override
  ConsumerState<PersonalEmailVerificationPage> createState() =>
      _PersonalEmailVerificationPageState();
}

class _PersonalEmailVerificationPageState
    extends ConsumerState<PersonalEmailVerificationPage> {
  final _emailController = TextEditingController();
  bool _sending = false;
  int? _resendAfterSeconds;

  @override
  void initState() {
    super.initState();
    // GlassTextField doesn't expose onChanged — listening on the
    // controller directly is what makes SEND CODE react as the user types.
    _emailController.addListener(_onFieldChanged);
  }

  void _onFieldChanged() => setState(() {});

  @override
  void dispose() {
    _emailController.removeListener(_onFieldChanged);
    _emailController.dispose();
    super.dispose();
  }

  String get _email => _emailController.text.trim();

  Future<void> _sendCode() async {
    if (_sending || _email.isEmpty) return;
    setState(() => _sending = true);
    try {
      final resendAfterSeconds = await ref
          .read(authServiceProvider)
          .startPersonalEmailVerification(_email);
      if (!mounted) return;
      setState(() => _resendAfterSeconds = resendAfterSeconds);
      showSnack(context, 'We sent a code to $_email', type: ToastType.success);
    } on SessionExpiredException {
      // No local error shown — AppShell's listener navigates to LandingPage
      // and shows the "session expired" message itself.
      if (mounted) ref.read(authSessionProvider.notifier).forceSignOut();
    } catch (error) {
      if (mounted) {
        showSnack(
          context,
          error is AuthException
              ? error.message
              : 'Something went wrong. Please try again.',
          type: ToastType.error,
        );
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  Future<void> _verify(String code) async {
    try {
      final session = await ref
          .read(authServiceProvider)
          .verifyPersonalEmailCode(_email, code);
      await ref
          .read(authSessionProvider.notifier)
          .completeVerification(session);
      if (!mounted) return;
      Navigator.pop(context);
    } on SessionExpiredException {
      // Rethrown so OtpEntry's own catch still surfaces the (specific,
      // non-generic) message while AppShell's listener navigates away.
      if (mounted) ref.read(authSessionProvider.notifier).forceSignOut();
      rethrow;
    }
  }

  Future<int> _resend() async {
    try {
      return await ref
          .read(authServiceProvider)
          .startPersonalEmailVerification(_email);
    } on SessionExpiredException {
      if (mounted) ref.read(authSessionProvider.notifier).forceSignOut();
      rethrow;
    }
  }

  @override
  Widget build(BuildContext context) {
    return VerificationScaffold(
      icon: Icons.email_outlined,
      headline: 'Verify Your Email',
      trustBenefit:
          'A verified personal email gives you an account-recovery path '
          'that doesn\'t depend on LinkedIn or your phone.',
      onSkip: () => Navigator.pop(context),
      child: _resendAfterSeconds == null
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                GlassTextField(
                  controller: _emailController,
                  icon: Icons.email_outlined,
                  hint: 'you@example.com',
                  keyboardType: TextInputType.emailAddress,
                ),
                const SizedBox(height: 16),
                GradientButton(
                  label: 'SEND CODE',
                  isLoading: _sending,
                  onPressed: _email.isNotEmpty ? _sendCode : null,
                ),
              ],
            )
          : OtpEntry(
              initialResendAfterSeconds: _resendAfterSeconds!,
              onSubmit: _verify,
              onResend: _resend,
            ),
    );
  }
}
