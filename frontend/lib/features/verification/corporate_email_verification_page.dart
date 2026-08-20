import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/verification/widgets/otp_entry.dart';
import 'package:professional_connections_platform/features/verification/widgets/verification_scaffold.dart';

/// Free-mail domains this screen warns about client-side, before the user
/// even submits — a UX hint only, not enforcement (Verification Model §
/// 5): the backend is the real check and rejects the same list
/// server-side regardless of what this screen does or doesn't catch.
const _freeEmailDomains = {
  'gmail.com',
  'yahoo.com',
  'hotmail.com',
  'outlook.com',
  'protonmail.com',
  'zoho.com',
  'icloud.com',
};

/// Same shape as [PersonalEmailVerificationPage], against the
/// corporate-email endpoints. Reachable both from the post-LinkedIn
/// onboarding sequence and independently from `ProfilePage`
/// (`frontend/PLAN.md`'s Level 2/3 addendum, Step 6).
class CorporateEmailVerificationPage extends ConsumerStatefulWidget {
  const CorporateEmailVerificationPage({super.key});

  @override
  ConsumerState<CorporateEmailVerificationPage> createState() =>
      _CorporateEmailVerificationPageState();
}

class _CorporateEmailVerificationPageState
    extends ConsumerState<CorporateEmailVerificationPage> {
  final _emailController = TextEditingController();
  bool _showOtpEntry = false;

  @override
  void initState() {
    super.initState();
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

  /// Client-side hint only — mirrors the backend's free-domain list
  /// (Verification Model § 5) but never blocks submission; the backend
  /// remains the actual enforcement point. A server-side rejection
  /// (`WorkEmailDomainRejectedException`) still surfaces clearly — inline
  /// on the OTP screen via [OtpEntry]'s own error handling, since sending
  /// is now optimistic (see [_showOtp]) rather than gated on this call
  /// succeeding first.
  bool get _looksLikeFreeEmail {
    final parts = _email.split('@');
    if (parts.length != 2) return false;
    return _freeEmailDomains.contains(parts[1].toLowerCase());
  }

  // Optimistic transition (UX improvement) — flips to the OTP entry screen
  // immediately, instead of waiting on the network call first; OtpEntry
  // sends the code itself once mounted, via [_startVerification], and
  // shows its own optimistic countdown/loading/error state for that — see
  // OtpEntry's own doc comment for why a slow or failing backend no longer
  // stalls this screen.
  void _showOtp() {
    if (_showOtpEntry || _email.isEmpty) return;
    setState(() => _showOtpEntry = true);
  }

  Future<int> _startVerification() async {
    try {
      return await ref
          .read(authServiceProvider)
          .startCorporateEmailVerification(_email);
    } on SessionExpiredException {
      // No local error shown here — AppShell's listener navigates to
      // LandingPage and shows the "session expired" message itself;
      // rethrown so OtpEntry's own catch doesn't also surface a redundant
      // generic message.
      if (mounted) ref.read(authSessionProvider.notifier).forceSignOut();
      rethrow;
    }
    // Includes WorkEmailDomainRejectedException on a genuine rejection —
    // its .message is already the backend's specific rejection text,
    // shown verbatim by OtpEntry's error handling (self-review checklist).
  }

  Future<void> _verify(String code) async {
    try {
      final session = await ref
          .read(authServiceProvider)
          .verifyCorporateEmailCode(_email, code);
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

  @override
  Widget build(BuildContext context) {
    return VerificationScaffold(
      icon: Icons.work_outline_rounded,
      headline: 'Verify Your Work Email',
      trustBenefit:
          'Verifying a work email is the strongest trust signal short of '
          'KYC — it unlocks a verified badge other members can see.',
      onSkip: () => Navigator.pop(context),
      child: !_showOtpEntry
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                GlassTextField(
                  controller: _emailController,
                  icon: Icons.work_outline_rounded,
                  hint: 'firstname.lastname@company.com',
                  keyboardType: TextInputType.emailAddress,
                ),
                if (_looksLikeFreeEmail) ...[
                  const SizedBox(height: 8),
                  const Text(
                    'This looks like a personal email address — work '
                    'email verification needs your company address.',
                    style: TextStyle(
                      fontSize: 11,
                      color: AppPalette.textSecondary,
                    ),
                  ),
                ],
                const SizedBox(height: 16),
                GradientButton(
                  label: 'SEND CODE',
                  onPressed: _email.isNotEmpty ? _showOtp : null,
                ),
              ],
            )
          : OtpEntry(onSend: _startVerification, onSubmit: _verify),
    );
  }
}
