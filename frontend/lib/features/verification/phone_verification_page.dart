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

/// Phone number entry → shared OTP widget. Reachable both from the
/// post-LinkedIn onboarding sequence and, independently, from
/// `ProfilePage` for a user who skipped and wants to finish later — same
/// screen either way (`frontend/PLAN.md`'s Level 2/3 addendum, Step 6).
///
/// No country-code picker UI: the launch market is Sri Lanka only (see
/// `docs/00-project/vision.md`), so the `+94` prefix is simply fixed —
/// building a full multi-country picker isn't justified by a single-market
/// launch scope. The user types only their local number.
class PhoneVerificationPage extends ConsumerStatefulWidget {
  const PhoneVerificationPage({super.key});

  @override
  ConsumerState<PhoneVerificationPage> createState() =>
      _PhoneVerificationPageState();
}

class _PhoneVerificationPageState extends ConsumerState<PhoneVerificationPage> {
  static const _countryCode = '+94';

  final _numberController = TextEditingController();
  bool _sending = false;
  int? _resendAfterSeconds;

  @override
  void initState() {
    super.initState();
    // GlassTextField doesn't expose onChanged — listening on the
    // controller directly is what makes SEND CODE react as the user types.
    _numberController.addListener(_onFieldChanged);
  }

  void _onFieldChanged() => setState(() {});

  @override
  void dispose() {
    _numberController.removeListener(_onFieldChanged);
    _numberController.dispose();
    super.dispose();
  }

  String get _fullNumber => '$_countryCode${_numberController.text.trim()}';

  Future<void> _sendCode() async {
    if (_sending || _numberController.text.trim().isEmpty) return;
    setState(() => _sending = true);
    try {
      final resendAfterSeconds = await ref
          .read(authServiceProvider)
          .startPhoneVerification(_fullNumber);
      if (!mounted) return;
      setState(() => _resendAfterSeconds = resendAfterSeconds);
      showSnack(
        context,
        'We sent a code to $_fullNumber',
        type: ToastType.success,
      );
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
    final session = await ref
        .read(authServiceProvider)
        .verifyPhoneCode(_fullNumber, code);
    await ref.read(authSessionProvider.notifier).completeVerification(session);
    if (!mounted) return;
    Navigator.pop(context);
  }

  Future<int> _resend() =>
      ref.read(authServiceProvider).startPhoneVerification(_fullNumber);

  @override
  Widget build(BuildContext context) {
    return VerificationScaffold(
      icon: Icons.phone_iphone_rounded,
      headline: 'Verify Your Phone',
      trustBenefit:
          'Verifying your number unlocks messaging other members and '
          'helps keep the community scam-resistant.',
      onSkip: () => Navigator.pop(context),
      child: _resendAfterSeconds == null
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                GlassTextField(
                  controller: _numberController,
                  icon: Icons.phone_iphone_rounded,
                  hint: '$_countryCode 7X XXX XXXX',
                  keyboardType: TextInputType.phone,
                ),
                const SizedBox(height: 16),
                GradientButton(
                  label: 'SEND CODE',
                  isLoading: _sending,
                  onPressed: _numberController.text.trim().isNotEmpty
                      ? _sendCode
                      : null,
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
