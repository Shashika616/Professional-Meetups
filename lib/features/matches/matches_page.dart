import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/match_profile.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';

class MatchesPage extends ConsumerWidget {
  const MatchesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final intent = ref.watch(selectedIntentProvider);
    final matchesAsync = ref.watch(matchesProvider(intent));

    return Scaffold(
      backgroundColor: Colors.transparent,
      appBar: AppBar(title: const Text('NEARBY PROFESSIONALS')),
      body: matchesAsync.when(
        loading: () => const _MatchesSkeleton(),
        error: (error, stack) => Center(
          child: Glass(
            radius: 20,
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.wifi_off_outlined, color: AppPalette.textSecondary, size: 28),
                const SizedBox(height: 10),
                const Text(
                  'Could not load matches.',
                  style: TextStyle(color: AppPalette.textPrimary, fontSize: 13),
                ),
                const SizedBox(height: 14),
                GradientButton(
                  label: 'RETRY',
                  height: 40,
                  onPressed: () => ref.invalidate(matchesProvider(intent)),
                ),
              ],
            ),
          ),
        ),
        data: (page) {
          if (page.items.isEmpty) {
            return const Center(
              child: Text(
                'No professionals nearby for this intent yet.',
                style: TextStyle(color: AppPalette.textSecondary, fontSize: 13),
              ),
            );
          }
          return ListView.builder(
            padding: const EdgeInsets.fromLTRB(20, 4, 20, 100),
            itemCount: page.items.length,
            itemBuilder: (context, index) => _MatchCard(match: page.items[index]),
          );
        },
      ),
    );
  }
}

class _MatchCard extends StatelessWidget {
  const _MatchCard({required this.match});

  final MatchProfile match;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Glass(
        radius: 22,
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 24,
                  backgroundColor: AppPalette.deepBlue,
                  child: Text(
                    match.initials,
                    style: const TextStyle(color: AppPalette.candyBlue, fontWeight: FontWeight.w700),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Removed hardcoded verified badge - server owns the truth
                      Text(
                        match.name,
                        style: const TextStyle(
                          color: AppPalette.textPrimary,
                          fontWeight: FontWeight.w600,
                          fontSize: 15,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        match.role,
                        style: const TextStyle(color: AppPalette.textSecondary, fontSize: 12),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 14),
            Row(
              children: [
                _tag(match.intent.label),
                const SizedBox(width: 8),
                _tag('${match.formattedDistance} AWAY'),
              ],
            ),
            const SizedBox(height: 14),
            Row(
              children: [
                Expanded(
                  child: GradientButton(
                    label: 'REQUEST MEETUP',
                    height: 42,
                    onPressed: () => showSnack(
                      context,
                      'Meetup request sent to ${match.name}',
                      type: ToastType.success,
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                _iconButton(
                  Icons.flag_outlined,
                  'Report',
                  context,
                  () => showSnack(
                    context,
                    'Report submitted. Our safety team will review it.',
                    type: ToastType.warning,
                  ),
                ),
                const SizedBox(width: 6),
                _iconButton(
                  Icons.block,
                  'Block',
                  context,
                  () => showSnack(
                    context,
                    'User blocked. They can no longer contact you.',
                    type: ToastType.locked,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _tag(String text) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        color: AppPalette.candyBlue.withValues(alpha: 0.10),
        border: Border.all(color: AppPalette.candyBlue.withValues(alpha: 0.30)),
      ),
      child: Text(
        text,
        style: const TextStyle(
          fontSize: 9,
          letterSpacing: 1.2,
          fontWeight: FontWeight.w700,
          color: AppPalette.candyBlue,
        ),
      ),
    );
  }

  Widget _iconButton(
    IconData icon,
    String tooltip,
    BuildContext context,
    VoidCallback onTap,
  ) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(10),
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: Colors.white.withValues(alpha: 0.05),
          border: Border.all(color: AppPalette.glassBorder),
        ),
        child: Icon(icon, size: 16, color: AppPalette.textSecondary),
      ),
    );
  }
}

class _MatchesSkeleton extends StatelessWidget {
  const _MatchesSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(20, 4, 20, 100),
      itemCount: 3,
      itemBuilder: (context, index) => Padding(
        padding: const EdgeInsets.only(bottom: 14),
        child: Glass(
          radius: 22,
          padding: const EdgeInsets.all(16),
          child: Column(
            children: [
              Row(
                children: [
                  Container(
                    width: 48,
                    height: 48,
                    decoration: const BoxDecoration(
                      shape: BoxShape.circle,
                      color: Color(0x10FFFFFF),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Container(height: 12, width: 120, color: const Color(0x14FFFFFF)),
                        const SizedBox(height: 6),
                        Container(height: 10, width: 80, color: const Color(0x0FFFFFFF)),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Container(height: 38, color: const Color(0x0CFFFFFF)),
            ],
          ),
        ),
      ),
    );
  }
}