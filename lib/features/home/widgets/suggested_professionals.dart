import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/match_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/core/widgets/skeleton_box.dart';

class SuggestedProfessionals extends ConsumerWidget {
  const SuggestedProfessionals({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final intent = ref.watch(selectedIntentProvider);
    final matchesAsync = ref.watch(matchesProvider(intent));

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionLabel('SUGGESTED FOR YOU'),
          const SizedBox(height: 16),
          SizedBox(
            height: 150,
            child: matchesAsync.when(
              loading: () => ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: 3,
                separatorBuilder: (_, __) => const SizedBox(width: 12),
                itemBuilder: (_, __) => const _SuggestedSkeleton(),
              ),
              error: (_, __) => const SizedBox.shrink(),
              data: (page) => page.items.isEmpty
                  ? const Text(
                      'No suggestions yet for this intent.',
                      style: TextStyle(color: AppPalette.textSecondary, fontSize: 12),
                    )
                  : ListView.separated(
                      scrollDirection: Axis.horizontal,
                      itemCount: page.items.length,
                      separatorBuilder: (_, __) => const SizedBox(width: 12),
                      itemBuilder: (context, index) => _SuggestedCard(
                        match: page.items[index],
                        onConnect: () => showSnack(context, 'Meetup request sent to ${page.items[index].name}'),
                      ),
                    ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SuggestedCard extends StatelessWidget {
  const _SuggestedCard({required this.match, required this.onConnect});

  final MatchProfile match;
  final VoidCallback onConnect;

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 20,
      padding: const EdgeInsets.all(14),
      child: SizedBox(
        width: 160,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 18,
                  backgroundColor: AppPalette.deepBlue,
                  child: Text(
                    match.initials,
                    style: const TextStyle(color: AppPalette.candyBlue, fontWeight: FontWeight.w700, fontSize: 12),
                  ),
                ),
                const SizedBox(width: 8),
                const Icon(Icons.verified, size: 14, color: AppPalette.verified),
              ],
            ),
            const SizedBox(height: 10),
            Text(
              match.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: AppPalette.textPrimary, fontSize: 13, fontWeight: FontWeight.w700),
            ),
            const SizedBox(height: 2),
            Text(
              match.role,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: AppPalette.textSecondary, fontSize: 11),
            ),
            const Spacer(),
            Row(
              children: [
                Text(
                  '${match.formattedDistance} AWAY',
                  style: const TextStyle(fontSize: 8, letterSpacing: 1, color: AppPalette.textSecondary, fontWeight: FontWeight.w700),
                ),
                const Spacer(),
                GestureDetector(
                  onTap: onConnect,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(14),
                      gradient: const LinearGradient(colors: [AppPalette.candyBlue, AppPalette.steelBlue]),
                    ),
                    child: const Text(
                      'CONNECT',
                      style: TextStyle(fontSize: 8, letterSpacing: 1.2, fontWeight: FontWeight.w900, color: AppPalette.onyx),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SuggestedSkeleton extends StatelessWidget {
  const _SuggestedSkeleton();

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 20,
      padding: const EdgeInsets.all(14),
      child: const SizedBox(
        width: 160,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SkeletonBox(width: 36, height: 36, radius: 18),
            SizedBox(height: 12),
            SkeletonBox(width: 100, height: 12, opacity: 0.08),
            SizedBox(height: 6),
            SkeletonBox(width: 70, height: 10),
            Spacer(),
            SkeletonBox(width: 130, height: 26, radius: 14),
          ],
        ),
      ),
    );
  }
}