import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/verification/widgets/otp_entry.dart';

enum _Step { email, otp, password }

/// Email + password signup (ADR-014 decision #2) — email field →
/// `startEmailSignupOtp` → OTP entry (reusing [OtpEntry], the same widget
/// phone/personal-email verification already use, not a rebuild) →
/// password field → `signUpWithEmail`. The OTP step here doesn't verify
/// against the server on its own — there's no separate "verify signup
/// code" endpoint — [OtpEntry.onSubmit] just advances locally to the
/// password step; the code is verified server-side together with the
/// password in one `CompleteEmailSignup` call once both are collected.
///
/// Pops `true` on a successful signup so the caller (`OnboardingFlow`)
/// knows to proceed to [AppShell]; pops nothing (`null`) if the user backs
/// out.
class EmailSignupStep extends ConsumerStatefulWidget {
  const EmailSignupStep({super.key, required this.ageConfirmedOver18});

  final bool ageConfirmedOver18;

  @override
  ConsumerState<EmailSignupStep> createState() => _EmailSignupStepState();
}

class _EmailSignupStepState extends ConsumerState<EmailSignupStep> {
  _Step _step = _Step.email;
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  String _code = '';
  bool _submittingPassword = false;

  @override
  void initState() {
    super.initState();
    _emailController.addListener(_onFieldChanged);
    _passwordController.addListener(_onFieldChanged);
  }

  void _onFieldChanged() => setState(() {});

  @override
  void dispose() {
    _emailController.removeListener(_onFieldChanged);
    _passwordController.removeListener(_onFieldChanged);
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  bool get _emailLooksValid => _emailController.text.trim().contains('@');

  // Optimistic transition (UX improvement) — flips to the OTP entry screen
  // immediately, instead of waiting on the network call first; OtpEntry
  // sends the code itself once mounted, via [_startSignupOtp], and shows
  // its own optimistic countdown/loading/error state for that — see
  // OtpEntry's own doc comment for why a slow or failing backend no longer
  // stalls this screen.
  void _showOtp() {
    if (!_emailLooksValid) return;
    setState(() => _step = _Step.otp);
  }

  Future<int> _startSignupOtp() => ref
      .read(authServiceProvider)
      .startEmailSignupOtp(_emailController.text.trim());

  Future<void> _onOtpEntered(String code) async {
    setState(() {
      _code = code;
      _step = _Step.password;
    });
  }

  Future<void> _submitPassword() async {
    if (_submittingPassword || _passwordController.text.length < 8) return;
    setState(() => _submittingPassword = true);
    try {
      await ref
          .read(authSessionProvider.notifier)
          .signUpWithEmail(
            email: _emailController.text.trim(),
            code: _code,
            password: _passwordController.text,
            ageConfirmedOver18: widget.ageConfirmedOver18,
          );
      if (!mounted) return;
      Navigator.pop(context, true);
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
      if (mounted) setState(() => _submittingPassword = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      appBar: AppBar(title: const Text('SIGN UP WITH EMAIL')),
      body: AppBackground(
        imageOpacity: 0.35,
        child: SafeArea(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
            child: switch (_step) {
              _Step.email => _emailStep(),
              _Step.otp => _otpStep(),
              _Step.password => _passwordStep(),
            },
          ),
        ),
      ),
    );
  }

  Widget _emailStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 8),
        const Text(
          'What’s your email?',
          style: TextStyle(
            fontSize: 22,
            fontWeight: FontWeight.w800,
            color: AppPalette.textPrimary,
          ),
        ),
        const SizedBox(height: 20),
        GlassTextField(
          controller: _emailController,
          icon: Icons.alternate_email_rounded,
          hint: 'you@example.com',
          keyboardType: TextInputType.emailAddress,
        ),
        const SizedBox(height: 16),
        GradientButton(
          label: 'SEND CODE',
          onPressed: _emailLooksValid ? _showOtp : null,
        ),
      ],
    );
  }

  Widget _otpStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 8),
        Text(
          'Enter the code we sent to ${_emailController.text.trim()}',
          style: const TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.w700,
            color: AppPalette.textPrimary,
          ),
        ),
        const SizedBox(height: 20),
        OtpEntry(onSend: _startSignupOtp, onSubmit: _onOtpEntered),
      ],
    );
  }

  Widget _passwordStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 8),
        const Text(
          'Set a password',
          style: TextStyle(
            fontSize: 22,
            fontWeight: FontWeight.w800,
            color: AppPalette.textPrimary,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'At least 8 characters. Preferrably a mix of letters, numbers, and symbols. This helps us keep your account secure and allows you to'
          'decide whether it’s accepted.',
          style: TextStyle(
            fontSize: 12,
            color: AppPalette.textSecondary.withValues(alpha: 0.85),
          ),
        ),
        const SizedBox(height: 20),
        GlassTextField(
          controller: _passwordController,
          icon: Icons.lock_outline_rounded,
          hint: 'Password',
          obscureText: true,
        ),
        const SizedBox(height: 16),
        GradientButton(
          label: 'CREATE ACCOUNT',
          isLoading: _submittingPassword,
          onPressed: _passwordController.text.length >= 8
              ? _submitPassword
              : null,
        ),
      ],
    );
  }
}
