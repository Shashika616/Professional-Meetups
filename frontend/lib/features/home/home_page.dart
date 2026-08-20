import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/home/widgets/availability_search_bar.dart';
import 'package:professional_connections_platform/features/home/widgets/home_header.dart';
import 'package:professional_connections_platform/features/home/widgets/intent_grid.dart';
import 'package:professional_connections_platform/features/home/widgets/network_insights_row.dart';
import 'package:professional_connections_platform/features/home/widgets/safety_tip_card.dart';
import 'package:professional_connections_platform/features/home/widgets/upcoming_meetup_card.dart';
import 'package:professional_connections_platform/features/meetups/schedule_flow.dart';

class HomePage extends ConsumerStatefulWidget {
  const HomePage({super.key});

  @override
  ConsumerState<HomePage> createState() => _HomePageState();
}

class _HomePageState extends ConsumerState<HomePage> {
  final TextEditingController availabilityController = TextEditingController();

  @override
  void dispose() {
    availabilityController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final selectedIntent = ref.watch(selectedIntentProvider);

    // Same pattern ProfilePage already uses (frontend/PLAN.md Step 13) —
    // fullName/profilePhotoUrl come from the cached session, "Member" is
    // the fallback while it's still loading or genuinely absent, and an
    // empty (not just null) photo URL is treated as "no photo" so
    // ProfessionalAvatar doesn't try to load an empty Image.network src.
    final profile = ref.watch(authSessionProvider).value?.profile;
    final fullName = profile?.fullName;
    final displayName = (fullName == null || fullName.isEmpty)
        ? 'Member'
        : fullName;
    final imageUrl = (profile?.profilePhotoUrl.isNotEmpty ?? false)
        ? profile!.profilePhotoUrl
        : null;
    // Level 0 (no profile resolved yet) is the safe default while loading —
    // ADR-014 made Level 0 (Apple/Google/email, no LinkedIn) a real,
    // reachable account state, not just "still loading," so this must never
    // assume a higher level than what's actually confirmed.
    final trustLevel = profile?.trustLevel ?? 0;

    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SafeArea(
        child: ListView(
          padding: EdgeInsets.zero,
          addAutomaticKeepAlives: true,
          addRepaintBoundaries: true,
          children: [
            HomeHeader(userName: displayName, imageUrl: imageUrl),
            AvailabilitySearchBar(
              controller: availabilityController,
              onFilterTap: () => showSnack(
                context,
                'Advanced filters arrive with Premium.',
                type: ToastType.info,
              ),
            ),
            IntentGrid(trustLevel: trustLevel),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 24, 20, 4),
              child: GradientButton(
                label: 'FIND MATCHES',
                onPressed: () {
                  if (!selectedIntent.isUnlockedFor(trustLevel)) {
                    showSnack(
                      context,
                      '${selectedIntent.label} requires Level ${selectedIntent.requiredTrustLevel} trust, Verify your phone, personal email, and details in Profile to unlock it.',
                      type: ToastType.locked,
                    );
                    return;
                  }
                  // Browse open meetups for the selected intent — the
                  // Matches tab (index 1) is the real surface for this now
                  // (ADR-013 § 7), not a toast.
                  ref.read(currentTabIndexProvider.notifier).state = 1;
                },
              ),
            ),
            // A separate entry point from FIND MATCHES, not a "+" icon
            // buried in the browse page's AppBar (where it used to live) —
            // someone opening the browse list wants to see other people's
            // open meetups, not stumble into hosting one. ScheduleFlowPage
            // has its own intent step with the same locked/unlocked
            // gating as the grid above, so no trust-level check is
            // duplicated here.
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 10, 20, 4),
              child: OutlinedButton.icon(
                onPressed: () async {
                  await Navigator.of(context).push(
                    MaterialPageRoute(builder: (_) => const ScheduleFlowPage()),
                  );
                  ref.invalidate(openMeetupsProvider(selectedIntent));
                  ref.invalidate(myMeetupsProvider);
                },
                icon: const Icon(
                  Icons.add_circle_outline,
                  size: 16,
                  color: AppPalette.verified,
                ),
                label: const Text(
                  'HOST YOUR OWN MEETUP',
                  style: TextStyle(
                    color: AppPalette.verified,
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 1.0,
                  ),
                ),
                style: OutlinedButton.styleFrom(
                  minimumSize: const Size.fromHeight(46),
                  backgroundColor: AppPalette.verified.withValues(alpha: 0.08),
                  side: BorderSide(
                    color: AppPalette.verified.withValues(alpha: 0.55),
                  ),
                ),
              ),
            ),
            const UpcomingMeetupCard(),
            const NetworkInsightsRow(),
            const SafetyTipCard(),
            // Clearance so the last card never hides behind the floating nav bar.
            const SizedBox(height: 96),
          ],
        ),
      ),
    );
  }
}
