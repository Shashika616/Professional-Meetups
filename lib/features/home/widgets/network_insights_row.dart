import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/core/widgets/skeleton_box.dart';

class NetworkInsightsRow extends ConsumerWidget {
  const NetworkInsightsRow({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final statsAsync = ref.watch(homeStatsProvider);

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionLabel('NETWORK INSIGHTS'),
          const SizedBox(height: 16),
          statsAsync.when(
            loading: () => const Row(
              children: [
                Expanded(child: _StatSkeleton()),
                SizedBox(width: 12),
                Expanded(child: _StatSkeleton()),
                SizedBox(width: 12),
                Expanded(child: _StatSkeleton()),
              ],
            ),
            error: (_, __) => const SizedBox.shrink(),
            data: (stats) => Row(
              children: [
                Expanded(child: _StatCard(label: 'VERIFIED NEARBY', value: '${stats['nearby']}', icon: Icons.people_alt_rounded)),
                const SizedBox(width: 12),
                Expanded(child: _StatCard(label: 'MEETUPS TODAY', value: '${stats['meetups']}', icon: Icons.coffee_rounded)),
                const SizedBox(width: 12),
                Expanded(child: _StatCard(label: 'TRUST SCORE', value: '${stats['trustScore']}', icon: Icons.verified_user_rounded)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({required this.label, required this.value, required this.icon});

  final String label;
  final String value;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 16,
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: AppPalette.candyBlue),
          const SizedBox(height: 12),
          Text(
            value,
            style: const TextStyle(
              color: AppPalette.textPrimary,
              fontWeight: FontWeight.w800,
              fontSize: 20,
              letterSpacing: -0.5,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            label,
            style: const TextStyle(
              fontSize: 8,
              letterSpacing: 1.2,
              color: AppPalette.textSecondary,
              fontWeight: FontWeight.w700,
            ),
          ),
        ],
      ),
    );
  }
}

class _StatSkeleton extends StatelessWidget {
  const _StatSkeleton();

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 16,
      padding: const EdgeInsets.all(14),
      child: const Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SkeletonBox(width: 20, height: 20),
          SizedBox(height: 12),
          SkeletonBox(width: 40, height: 20, opacity: 0.08),
          SizedBox(height: 8),
          SkeletonBox(width: 60, height: 10),
        ],
      ),
    );
  }
}