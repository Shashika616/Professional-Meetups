import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/user_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/core/widgets/professional_avatar.dart';
import 'package:professional_connections_platform/core/widgets/verification_badges.dart';
import 'package:professional_connections_platform/features/landing/landing_page.dart';
import 'package:professional_connections_platform/features/onboarding/onboarding_flow.dart';
import 'package:professional_connections_platform/features/verification/corporate_email_verification_page.dart';
import 'package:professional_connections_platform/features/verification/personal_details_page.dart';
import 'package:professional_connections_platform/features/verification/personal_email_verification_page.dart';
import 'package:professional_connections_platform/features/verification/phone_verification_page.dart';

class ProfilePage extends ConsumerWidget {
  const ProfilePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // The one real verification step this slice has (LinkedIn) — fullName/
    // profilePhotoUrl/trustLevel come from the one-time linkedin/callback
    // response, cached in AuthSessionState (see backend/PLAN.md Step 3 /
    // frontend/PLAN.md Step 3). Null while the session is still loading or
    // if somehow unauthenticated; the block below falls back sensibly.
    final profile = ref.watch(authSessionProvider).value?.profile;

    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SafeArea(
        child: ListView(
          padding: EdgeInsets.zero,
          children: [
            const SizedBox(height: 28),
            _avatarBlock(profile),
            const SizedBox(height: 22),
            _statsRow(profile),
            // Level 0 (ADR-014) — visible only for an account that hasn't
            // connected LinkedIn yet (Apple/Google/email signup, or a
            // LinkedIn link that hasn't happened). LinkedIn is the ONLY
            // path to Level 1+, so this is the one place a Level 0 account
            // can unlock matching/messaging/meetups.
            if (profile != null && !profile.linkedInConnected) ...[
              const SizedBox(height: 20),
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 20),
                child: _ConnectLinkedInBanner(),
              ),
            ],
            const SizedBox(height: 28),
            _section(
              'VERIFICATION',
              children: [
                _verificationRow(
                  context,
                  icon: Icons.phone_android,
                  title: 'Phone',
                  done: profile?.phoneVerified ?? false,
                  locked: !(profile?.linkedInConnected ?? false),
                  buildScreen: (context) => const PhoneVerificationPage(),
                ),
                const _Divider(),
                _Row(
                  icon: Icons.work_outline,
                  title: 'LinkedIn',
                  // Genuinely conditional on profile.linkedInConnected
                  // (trustLevel >= 1) — LinkedIn is the sole path to Level
                  // 1+ (ADR-014 §1), not merely "a profile resolved," since
                  // Level 0 (Apple/Google/email, no LinkedIn) is now a real,
                  // common account state. No "VERIFY" action to offer
                  // either way — the banner above is the actual entry
                  // point for connecting it.
                  subtitle: (profile?.linkedInConnected ?? false)
                      ? 'LinkedIn Verified'
                      : 'Not connected',
                  trailing: (profile?.linkedInConnected ?? false)
                      ? const Icon(
                          Icons.check_circle_rounded,
                          size: 18,
                          color: AppPalette.verified,
                        )
                      : const SizedBox.shrink(),
                ),
                const _Divider(),
                _verificationRow(
                  context,
                  icon: Icons.alternate_email_rounded,
                  title: 'Personal Email',
                  done: profile?.personalEmailVerified ?? false,
                  locked: !(profile?.linkedInConnected ?? false),
                  buildScreen: (context) =>
                      const PersonalEmailVerificationPage(),
                ),
                const _Divider(),
                _verificationRow(
                  context,
                  icon: Icons.badge_outlined,
                  title: 'Personal Details',
                  done: profile?.personalDetailsComplete ?? false,
                  locked: !(profile?.linkedInConnected ?? false),
                  buildScreen: (context) => const PersonalDetailsPage(),
                ),
                const _Divider(),
                _verificationRow(
                  context,
                  icon: Icons.email_outlined,
                  title: 'Work Email',
                  done: profile?.workEmailVerified ?? false,
                  locked: !(profile?.linkedInConnected ?? false),
                  buildScreen: (context) =>
                      const CorporateEmailVerificationPage(),
                ),
              ],
            ),
            const SizedBox(height: 24),
            _section(
              'PREFERENCES',
              children: const [
                _Row(
                  icon: Icons.lock_outline,
                  title: 'Privacy Controls',
                  subtitle: 'Visibility and location',
                  trailing: Icon(
                    Icons.chevron_right_rounded,
                    size: 18,
                    color: AppPalette.textSecondary,
                  ),
                ),
                _Divider(),
                _Row(
                  icon: Icons.notifications_none_rounded,
                  title: 'Notifications',
                  subtitle: 'Push and email alerts',
                  trailing: Icon(
                    Icons.chevron_right_rounded,
                    size: 18,
                    color: AppPalette.textSecondary,
                  ),
                ),
                _Divider(),
                _Row(
                  icon: Icons.shield_outlined,
                  title: 'Safety Center',
                  subtitle: 'Trusted contacts and SOS',
                  trailing: Icon(
                    Icons.chevron_right_rounded,
                    size: 18,
                    color: AppPalette.textSecondary,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: GestureDetector(
                onTap: () => _confirmSignOut(context, ref),
                child: Glass(
                  radius: 16,
                  tint: AppPalette.danger.withValues(alpha: 0.08),
                  border: AppPalette.danger.withValues(alpha: 0.3),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  child: const Center(
                    child: Text(
                      'SIGN OUT',
                      style: TextStyle(
                        fontSize: 11,
                        letterSpacing: 1.8,
                        fontWeight: FontWeight.w800,
                        color: AppPalette.danger,
                      ),
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 96),
          ],
        ),
      ),
    );
  }

  /// Gates [_signOut] behind an explicit confirm tap — a mis-tap on SIGN
  /// OUT used to end the session immediately with no way back. Mirrors
  /// `safety_page.dart`'s SOS confirmation dialog styling (the one existing
  /// confirm-dialog pattern in this codebase), not a new dialog style.
  Future<void> _confirmSignOut(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: AppPalette.card,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
        title: const Text(
          'SIGN OUT',
          style: TextStyle(
            color: AppPalette.danger,
            letterSpacing: 1.6,
            fontSize: 15,
          ),
        ),
        content: const Text(
          'Sign out of Professional Connections?',
          style: TextStyle(color: AppPalette.textSecondary, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text(
              'CANCEL',
              style: TextStyle(color: AppPalette.textSecondary),
            ),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text(
              'SIGN OUT',
              style: TextStyle(
                color: AppPalette.danger,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    if (!context.mounted) return;
    await _signOut(context, ref);
  }

  Future<void> _signOut(BuildContext context, WidgetRef ref) async {
    // Clears the local session regardless of whether the logout network
    // call succeeds — see AuthSessionNotifier.signOut.
    await ref.read(authSessionProvider.notifier).signOut();
    if (!context.mounted) return;
    // Clears the nav stack so the back button can't return to AppShell.
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (context) => const LandingPage()),
      (route) => false,
    );
  }

  Widget _avatarBlock(UserProfile? profile) {
    final fullName = profile?.fullName;
    final displayName = (fullName == null || fullName.isEmpty)
        ? 'Member'
        : fullName;

    return Center(
      child: Column(
        children: [
          ProfessionalAvatar(
            size: 92,
            name: fullName,
            imageUrl: (profile?.profilePhotoUrl.isNotEmpty ?? false)
                ? profile!.profilePhotoUrl
                : null,
          ),
          const SizedBox(height: 14),
          Text(
            displayName,
            style: const TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w800,
              color: AppPalette.textPrimary,
              letterSpacing: -0.3,
            ),
          ),
          const SizedBox(height: 8),
          VerificationBadges(trustLevel: profile?.trustLevel ?? 0),
          // headline (e.g. "SWE • Colombo") has no backend source in this
          // slice — LinkedIn's OIDC userinfo call doesn't return one, and
          // there's nowhere else to get it from yet (UserProfile.headline
          // doc comment). Nothing shown here rather than inventing data.
        ],
      ),
    );
  }

  Widget _statsRow(UserProfile? profile) {
    final trustLevel = profile?.trustLevel;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Row(
        children: [
          const Expanded(
            child: _StatChip(value: '12', label: 'MEETUPS'),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _StatChip(
              value: (profile?.ratingCount ?? 0) == 0
                  ? '—'
                  : profile!.ratingAverage.toStringAsFixed(1),
              label: 'RATING',
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _StatChip(
              value: trustLevel == null ? '—' : 'L$trustLevel',
              label: 'TRUST',
            ),
          ),
        ],
      ),
    );
  }

  Widget _section(String title, {required List<Widget> children}) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SectionLabel(title),
          const SizedBox(height: 12),
          Glass(
            radius: 20,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
            child: Column(children: children),
          ),
        ],
      ),
    );
  }

  /// Builds a Phone/Personal-Email/Personal-Details/Work-Email row: a
  /// "Verified" check once [UserProfile] (via `profile`, watched in
  /// [build]) reports it done, otherwise a VERIFY chip that pushes
  /// [buildScreen] — the exact same screen class the post-LinkedIn
  /// onboarding sequence uses (`onboarding_flow.dart`'s
  /// `pendingVerificationSteps`), so there's no second implementation of
  /// any of these flows for the "reached from Profile" case
  /// (`frontend/PLAN.md`'s Level 2/3 addendum, Step 6). Reactive: popping
  /// back here after a successful verify re-renders from the freshly
  /// updated `authSessionProvider` state (no manual refresh needed).
  ///
  /// [locked] (ADR-014's Level 0 read-only audit, Step 6): every one of
  /// these four steps requires LinkedIn server-side (`requireLinkedIn`) —
  /// tapping VERIFY at Level 0 would just 403. Rather than let that happen,
  /// the chip is replaced with a locked hint pointing at the LinkedIn
  /// banner above.
  Widget _verificationRow(
    BuildContext context, {
    required IconData icon,
    required String title,
    required bool done,
    required bool locked,
    required WidgetBuilder buildScreen,
  }) {
    return _Row(
      icon: icon,
      title: title,
      subtitle: done
          ? 'Verified'
          : locked
          ? 'Connect LinkedIn first'
          : 'Not verified',
      trailing: done
          ? const Icon(
              Icons.check_circle_rounded,
              size: 18,
              color: AppPalette.verified,
            )
          : locked
          ? const Icon(
              Icons.lock_outline_rounded,
              size: 16,
              color: AppPalette.textSecondary,
            )
          : _verifyChip(context, buildScreen),
    );
  }

  Widget _verifyChip(BuildContext context, WidgetBuilder buildScreen) {
    return GestureDetector(
      onTap: () =>
          Navigator.push(context, MaterialPageRoute(builder: buildScreen)),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(14),
          border: Border.all(
            color: AppPalette.candyBlue.withValues(alpha: 0.5),
          ),
        ),
        child: const Text(
          'VERIFY',
          style: TextStyle(
            fontSize: 8,
            letterSpacing: 1.4,
            fontWeight: FontWeight.w800,
            color: AppPalette.candyBlue,
          ),
        ),
      ),
    );
  }
}

/// Profile's "Connect LinkedIn" entry point (ADR-014 Step 7) — calls
/// [AuthSessionNotifier.linkLinkedIn], not `signInWithLinkedIn` (that
/// creates/resolves an account; this links to the caller's already-
/// authenticated one). On success, runs the same pending-verification
/// sequence a fresh-onboarding LinkedIn signup would have shown, so
/// connecting from Profile doesn't skip the Level 2 steps.
class _ConnectLinkedInBanner extends ConsumerStatefulWidget {
  const _ConnectLinkedInBanner();

  @override
  ConsumerState<_ConnectLinkedInBanner> createState() =>
      _ConnectLinkedInBannerState();
}

class _ConnectLinkedInBannerState
    extends ConsumerState<_ConnectLinkedInBanner> {
  bool _busy = false;

  Future<void> _connect() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await ref.read(authSessionProvider.notifier).linkLinkedIn();
      if (!mounted) return;
      final profile = ref.read(authSessionProvider).value?.profile;
      await runVerificationSequence(context, pendingVerificationSteps(profile));
    } catch (error) {
      if (mounted) {
        // A cancellation (closed the browser without finishing) gets a
        // softer, non-alarming toast, not the red error styling a genuine
        // network/server failure gets — same distinction
        // onboarding_flow.dart's sign-in error handling makes.
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
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 18,
      tint: AppPalette.candyBlue.withValues(alpha: 0.08),
      border: AppPalette.candyBlue.withValues(alpha: 0.3),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(
                Icons.work_outline,
                size: 18,
                color: AppPalette.candyBlue,
              ),
              const SizedBox(width: 8),
              const Text(
                'Connect LinkedIn',
                style: TextStyle(
                  color: AppPalette.textPrimary,
                  fontWeight: FontWeight.w800,
                  fontSize: 14,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          const Text(
            'Your account is restricted to view only until you connect LinkedIn...'
            'unlock matching, messaging, and meetups.',
            style: TextStyle(color: AppPalette.textSecondary, fontSize: 12),
          ),
          const SizedBox(height: 14),
          GradientButton(
            label: 'CONNECT LINKEDIN',
            height: 44,
            isLoading: _busy,
            onPressed: _connect,
          ),
        ],
      ),
    );
  }
}

class _StatChip extends StatelessWidget {
  const _StatChip({required this.value, required this.label});

  final String value;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 16,
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Column(
        children: [
          Text(
            value,
            style: const TextStyle(
              color: AppPalette.candyBlue,
              fontWeight: FontWeight.w800,
              fontSize: 16,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: const TextStyle(
              fontSize: 7,
              letterSpacing: 1.4,
              color: AppPalette.textSecondary,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    );
  }
}

class _Row extends StatelessWidget {
  const _Row({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.trailing,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final Widget trailing;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: Colors.white.withValues(alpha: 0.05),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(icon, size: 18, color: AppPalette.candyBlue),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    color: AppPalette.textPrimary,
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  subtitle,
                  style: const TextStyle(
                    color: AppPalette.textSecondary,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ),
          trailing,
        ],
      ),
    );
  }
}

class _Divider extends StatelessWidget {
  const _Divider();

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 1,
      color: AppPalette.glassBorder,
      margin: const EdgeInsets.symmetric(vertical: 2),
    );
  }
}
