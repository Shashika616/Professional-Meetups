import 'package:flutter/foundation.dart';
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
import 'package:professional_connections_platform/features/onboarding/age_confirmation_step.dart';
import 'package:professional_connections_platform/features/onboarding/email_signup_step.dart';
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
/// Only ever run after a LinkedIn-connecting path (direct signup here, or
/// Profile's "Connect LinkedIn") — every step in this sequence requires
/// LinkedIn server-side (`requireLinkedIn`, ADR-014 §4), so running it for
/// an Apple/Google/email-only Level 0 account would just walk the user
/// through screens that 403 immediately. Apple/Google/email signup goes
/// straight to [AppShell] instead (see `_OnboardingFlowState`'s dispatch
/// logic).
///
/// Filtered against [profile]'s already-verified flags — a returning user
/// (signing back in after a voluntary sign-out, or after the session-
/// refresh addendum's involuntary forceSignOut() path) must not be walked
/// back through steps the backend already has recorded as done. `profile`
/// is null only if the fetch right after sign-in itself failed, in which
/// case the safe fallback is the full sequence, same as a brand-new user —
/// not silently skipping steps we have no actual confirmation are done.
List<WidgetBuilder> pendingVerificationSteps(UserProfile? profile) {
  return [
    if (profile?.phoneVerified != true) _buildPhoneVerificationPage,
    if (profile?.personalEmailVerified != true)
      _buildPersonalEmailVerificationPage,
    if (profile?.personalDetailsComplete != true) _buildPersonalDetailsPage,
    if (profile?.workEmailVerified != true)
      _buildCorporateEmailVerificationPage,
  ];
}

/// Pushes each still-pending verification screen in turn, awaiting the pop
/// before pushing the next one — every screen pops itself (Skip or a
/// successful verify). Shared by [OnboardingFlow] (after a fresh LinkedIn
/// signup) and `ProfilePage`'s "Connect LinkedIn" banner (ADR-014 Step 7:
/// connecting from Profile shouldn't skip the Level 2 steps a
/// fresh-onboarding LinkedIn user would have seen) — one implementation of
/// this push-chain, not two.
Future<void> runVerificationSequence(
  BuildContext context,
  List<WidgetBuilder> steps,
) async {
  for (final buildScreen in steps) {
    if (!context.mounted) return;
    await Navigator.push(context, MaterialPageRoute(builder: buildScreen));
  }
}

class OnboardingFlow extends ConsumerStatefulWidget {
  const OnboardingFlow({super.key});

  @override
  ConsumerState<OnboardingFlow> createState() => _OnboardingFlowState();
}

enum _OnboardingStep { ageConfirmation, chooseMethod }

class _OnboardingFlowState extends ConsumerState<OnboardingFlow> {
  _OnboardingStep _step = _OnboardingStep.ageConfirmation;
  bool _busy;

  _OnboardingFlowState() : _busy = false;

  void _onAgeConfirmed() {
    setState(() => _step = _OnboardingStep.chooseMethod);
  }

  Future<void> _continueWithLinkedIn() async {
    if (_busy) return; // guards against a slow tap double-firing the flow
    setState(() => _busy = true);
    try {
      await ref
          .read(authSessionProvider.notifier)
          .signInWithLinkedIn(ageConfirmedOver18: true);
      if (!mounted) return;
      final profile = ref.read(authSessionProvider).value?.profile;
      await runVerificationSequence(context, pendingVerificationSteps(profile));
      if (!mounted) return;
      _goToAppShell();
    } catch (error, stackTrace) {
      _handleSignInError('signInWithLinkedIn', error, stackTrace);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _continueWithApple() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref
          .read(authSessionProvider.notifier)
          .signInWithApple(ageConfirmedOver18: true);
      if (!mounted) return;
      // Level 0 (Apple alone never grants trust, ADR-014 §1) — straight to
      // AppShell, no verification sequence attempted (every step in it
      // requires LinkedIn server-side and would just 403).
      _goToAppShell();
    } catch (error, stackTrace) {
      _handleSignInError('signInWithApple', error, stackTrace);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _continueWithGoogle() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref
          .read(authSessionProvider.notifier)
          .signInWithGoogle(ageConfirmedOver18: true);
      if (!mounted) return;
      _goToAppShell();
    } catch (error, stackTrace) {
      _handleSignInError('signInWithGoogle', error, stackTrace);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _openEmailSignup() async {
    final success = await Navigator.push<bool>(
      context,
      MaterialPageRoute(
        builder: (context) => const EmailSignupStep(ageConfirmedOver18: true),
      ),
    );
    if (success == true && mounted) _goToAppShell();
  }

  void _handleSignInError(String source, Object error, StackTrace stackTrace) {
    // Logged so the underlying cause is visible in the console — the
    // toast itself only ever shows a user-safe message, never raw
    // exception detail.
    debugPrint('$source failed: $error\n$stackTrace');
    if (!mounted) return;
    // A cancellation (backed out of the provider's picker, or closed the
    // browser without finishing LinkedIn) isn't really an "error" — a
    // softer, non-alarming toast, not the red error styling a genuine
    // network/server failure gets. Either way the button must always end
    // up clickable again (the `finally` in each _continueWithX already
    // does that) and the user must always see *something*, never be left
    // staring at a stalled spinner with no feedback at all.
    showSnack(
      context,
      error is AuthException
          ? error.message
          : 'Something went wrong. Please try again.',
      type: error is SignInCancelledException
          ? ToastType.info
          : ToastType.error,
    );
  }

  // pushAndRemoveUntil, not pushReplacement: OnboardingFlow itself was
  // pushed on top of LandingPage (LandingPage.push, not replace), so a
  // plain pushReplacement here would leave LandingPage sitting under
  // AppShell in the stack — any tab page with its own AppBar (Matches/
  // Safety/Chats) would then show a back arrow that pops AppShell and
  // strands the user on LandingPage instead of switching tabs. Clearing
  // the whole stack matches the same pattern already used for sign-out
  // (ProfilePage) and forced session expiry (AppShell's own listener).
  void _goToAppShell() {
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (context) => const AppShell()),
      (route) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: AppBackground(
        imageOpacity: 0.35,
        child: switch (_step) {
          _OnboardingStep.ageConfirmation => AgeConfirmationStep(
            onContinue: _onAgeConfirmed,
          ),
          _OnboardingStep.chooseMethod => _chooseMethodStep(),
        },
      ),
    );
  }

  Widget _chooseMethodStep() {
    // iOS shows Apple+LinkedIn; Android shows Google+LinkedIn (scope note,
    // frontend/level0-federated-identity-PLAN.md: Apple has no native
    // Android SDK, and Google Sign-In has no first-class iOS placement
    // requirement the way Apple does on iOS). defaultTargetPlatform, not
    // dart:io Platform, so this stays testable in `flutter test`.
    final isIOS = defaultTargetPlatform == TargetPlatform.iOS;

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Expanded(child: _welcomeStep()),
            const SizedBox(height: 16),
            _trustMicrocopy(),
            const SizedBox(height: 20),
            // Co-equal buttons — same GradientButton widget/height for both,
            // differing only in icon/label/handler, so Apple's button is
            // genuinely equal in size and visual weight to LinkedIn's (Apple
            // Guideline 4.8's real placement requirement, not a style
            // choice) by construction rather than by eyeballing two
            // different button styles.
            if (isIOS) ...[
              GradientButton(
                label: 'CONTINUE WITH APPLE',
                icon: Icons.apple,
                isLoading: _busy,
                onPressed: _continueWithApple,
              ),
              const SizedBox(height: 12),
              GradientButton(
                key: const Key('continueWithLinkedIn'),
                label: 'CONTINUE WITH LINKEDIN',
                isLoading: _busy,
                onPressed: _continueWithLinkedIn,
              ),
            ] else ...[
              GradientButton(
                label: 'CONTINUE WITH GOOGLE',
                icon: Icons.g_mobiledata_rounded,
                isLoading: _busy,
                onPressed: _continueWithGoogle,
              ),
              const SizedBox(height: 12),
              GradientButton(
                key: const Key('continueWithLinkedIn'),
                label: 'CONTINUE WITH LINKEDIN',
                isLoading: _busy,
                onPressed: _continueWithLinkedIn,
              ),
            ],
            const SizedBox(height: 16),
            Center(
              child: GestureDetector(
                onTap: _busy ? null : _openEmailSignup,
                child: const Text(
                  'Sign up with email',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: AppPalette.textSecondary,
                    decoration: TextDecoration.underline,
                  ),
                ),
              ),
            ),
          ],
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

  /// ADR-014's microcopy — replaces the old LinkedIn-only trust copy, since
  /// LinkedIn is now optional-at-signup rather than mandatory. States
  /// plainly that skipping LinkedIn keeps the account read-only, and that
  /// it can be connected later from Profile (`ProfilePage`'s "Connect
  /// LinkedIn" banner, Step 7) — not a dead end.
  Widget _trustMicrocopy() {
    return const SizedBox(
      width: double.infinity,
      child: Text(
        'Signing in without LinkedIn keeps your account more restricted. We do this to ensure a private and secure experience. Connect '
        'LinkedIn anytime during setup or later from your profile to '
        'unlock matching, messaging, and meetups.',
        textAlign: TextAlign.center,
        style: TextStyle(
          fontSize: 11,
          color: AppPalette.textSecondary,
          height: 1.4,
        ),
      ),
    );
  }
}
