import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/home/widgets/availability_search_bar.dart';
import 'package:professional_connections_platform/features/home/widgets/home_header.dart';
import 'package:professional_connections_platform/features/home/widgets/intent_grid.dart';
import 'package:professional_connections_platform/features/home/widgets/network_insights_row.dart';
import 'package:professional_connections_platform/features/home/widgets/safety_tip_card.dart';
import 'package:professional_connections_platform/features/home/widgets/suggested_professionals.dart';
import 'package:professional_connections_platform/features/home/widgets/upcoming_meetup_card.dart';

class HomePage extends ConsumerStatefulWidget {
  const HomePage({super.key});

  @override
  ConsumerState<HomePage> createState() => _HomePageState();
}

class _HomePageState extends ConsumerState<HomePage> {
  final TextEditingController availabilityController = TextEditingController();

  // Note: replace with the authenticated user's trust level from the backend.
  static const int currentTrustLevel = 1;

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
            IntentGrid(trustLevel: currentTrustLevel),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 24, 20, 4),
              child: GradientButton(
                label: 'FIND MATCHES',
                onPressed: () => showSnack(
                  context,
                  'Finding ${selectedIntent.label} matches near you...',
                  type: ToastType.success,
                ),
              ),
            ),
            const UpcomingMeetupCard(),
            const SuggestedProfessionals(),
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
