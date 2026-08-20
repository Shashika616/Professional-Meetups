import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/app_shell.dart';
import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/features/verification/corporate_email_verification_page.dart';
import 'package:professional_connections_platform/features/verification/personal_details_page.dart';
import 'package:professional_connections_platform/features/verification/personal_email_verification_page.dart';
import 'package:professional_connections_platform/features/verification/phone_verification_page.dart';

Widget _buildPhoneVerificationPage(BuildContext context) =>
    const PhoneVerificationPage();
Widget _buildPersonalEmailVerificationPage(BuildContext context) =>
    const PersonalEmailVerificationPage();
Widget _buildPersonalDetailsPage(BuildContext context) =>
    const PersonalDetailsPage();
Widget _buildCorporateEmailVerificationPage(BuildContext context) =>
    const CorporateEmailVerificationPage();

/// After a successful LinkedIn sign-in, the sequence phone → personal
/// email → personal details → corporate email runs before landing in
/// [AppShell] — each step individually skippable (`frontend/PLAN.md`'s
/// Level 2/3 addendum, Step 5). Deliberately simple: a plain step-index
/// push chain, not a `PageView` or a wizard with back-navigation/progress-
/// saving — if the app is killed mid-sequence, the user lands back in
/// [AppShell] next launch (the LinkedIn session already exists) and
/// finishes any skipped steps from `ProfilePage` instead.
///
/// Filtered against [profile]'s already-verified flags — a returning user
/// (signing back in after a voluntary sign-out, or after the session-
/// refresh addendum's involuntary forceSignOut() path) must not be walked
/// back through steps the backend already has recorded as done. `profile`
/// is null only if the fetch right after sign-in itself failed, in which
/// case the safe fallback is the full sequence, same as a brand-new user —
/// not silently skipping steps we have no actual confirmation are done.
List<WidgetBuilder> _pendingVerificationSteps(UserProfile? profile) {
  return [
    if (profile?.phoneVerified != true) _buildPhoneVerificationPage,
    if (profile?.personalEmailVerified != true)
      _buildPersonalEmailVerificationPage,
    if (profile?.personalDetailsComplete != true) _buildPersonalDetailsPage,
    if (profile?.workEmailVerified != true)
      _buildCorporateEmailVerificationPage,
  ];
}

class OnboardingFlow extends ConsumerStatefulWidget {
  const OnboardingFlow({super.key});

  @override
  ConsumerState<OnboardingFlow> createState() => _OnboardingFlowState();
}

class _OnboardingFlowState extends ConsumerState<OnboardingFlow> {
  bool _busy = false;

  Future<void> _continueWithLinkedIn() async {
    if (_busy) return; // guards against a slow tap double-firing the flow
    setState(() => _busy = true);

    try {
      await ref.read(authSessionProvider.notifier).signInWithLinkedIn();
      if (!mounted) return;
      final profile = ref.read(authSessionProvider).value?.profile;
      await _runVerificationSequence(_pendingVerificationSteps(profile));
      if (!mounted) return;
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (context) => const AppShell()),
      );
    } catch (error, stackTrace) {
      // Logged so the underlying cause is visible in the console — the
      // toast itself only ever shows a user-safe message, never raw
      // exception detail.
      debugPrint('signInWithLinkedIn failed: $error\n$stackTrace');
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
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Pushes each still-pending verification screen in turn, awaiting the
  /// pop before pushing the next one — every screen pops itself (Skip or a
  /// successful verify), so by the time this returns we're back on
  /// [OnboardingFlow] with the whole sequence behind us, ready for the
  /// final `pushReplacement` to [AppShell]. Already-verified steps aren't
  /// in [steps] at all (see [_pendingVerificationSteps]), so a returning
  /// user with some or all of Level 2/3 already done sees only what's
  /// actually left, or nothing.
  Future<void> _runVerificationSequence(List<WidgetBuilder> steps) async {
    for (final buildScreen in steps) {
      if (!mounted) return;
      await Navigator.push(context, MaterialPageRoute(builder: buildScreen));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: AppBackground(
        imageOpacity: 0.35,
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(child: _welcomeStep()),
                const SizedBox(height: 16),
                _trustMicrocopy(),
                const SizedBox(height: 20),
                GradientButton(
                  label: 'CONTINUE WITH LINKEDIN',
                  isLoading: _busy,
                  onPressed: _continueWithLinkedIn,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _welcomeStep() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              border: Border.all(
                color: AppPalette.candyBlue.withValues(alpha: 0.3),
                width: 2,
              ),
            ),
            child: const Icon(
              Icons.handshake_outlined,
              size: 64,
              color: AppPalette.candyBlue,
            ),
          ),
          const SizedBox(height: 32),
          const Text(
            'Connect Beyond\nThe Office.',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 32,
              fontWeight: FontWeight.w800,
              color: AppPalette.textPrimary,
              height: 1.2,
              letterSpacing: -0.5,
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Meet verified professionals in real life.\nYour next coffee, mentor, or co-founder is nearby.',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 14,
              color: AppPalette.textSecondary,
              height: 1.5,
            ),
          ),
        ],
      ),
    );
  }

  /// Trust reassurance directly above the auth button — states why LinkedIn
  /// specifically (identity, not just an OAuth checkbox) and pre-empts the
  /// most common worry about connecting it. The "never post on your behalf
  /// or access your connections" claim must stay accurate to the actual
  /// requested scope (`openid profile email` in `http_auth_service.dart`,
  /// which grants neither) — revisit this copy in the same PR if that scope
  /// ever changes.
  Widget _trustMicrocopy() {
    return SizedBox(
      width: double.infinity,
      child: Text(
        'We verify your LinkedIn to confirm you’re a real, working '
        'professional — the foundation of a safer community. We never '
        'post on your behalf or access your connections.',
        textAlign: TextAlign.center,
        style: TextStyle(
          fontSize: 11,
          color: AppPalette.textSecondary.withValues(alpha: 0.85),
          height: 1.4,
        ),
      ),
    );
  }
}
