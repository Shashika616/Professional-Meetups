import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/models/paged_result.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/meetup_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/core/widgets/meetup_status_badge.dart';
import 'package:professional_connections_platform/core/widgets/professional_avatar.dart';
import 'package:professional_connections_platform/core/widgets/skeleton_box.dart';
import 'package:professional_connections_platform/core/widgets/star_rating.dart';
import 'package:professional_connections_platform/core/widgets/trust_level_badge.dart';
import 'package:professional_connections_platform/core/widgets/verification_badges.dart';
import 'package:professional_connections_platform/features/meetups/meetup_detail_page.dart';

/// Browse open meetups for the selected intent — replaces the old mock
/// "nearby professionals" list (ADR-013, frontend/meetup-scheduling-
/// PLAN.md Step 6). Kept as the same route/mount point as the retired
/// MatchesPage for minimal navigation churn.
///
/// Hosting a new meetup used to live behind a "+" icon in this page's own
/// AppBar — confusing, since a user opens this page to browse *other*
/// people's open meetups, not to host their own. That entry point moved to
/// a dedicated "HOST YOUR OWN MEETUP" button on HomePage instead; this page
/// only browses now.
class MatchesPage extends ConsumerWidget {
  const MatchesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final intent = ref.watch(selectedIntentProvider);
    // Level 0 is the safe default while the profile hasn't resolved yet —
    // ADR-014 made Level 0 (Apple/Google/email, no LinkedIn) a real account
    // state, so this must never assume a higher level than confirmed.
    final trustLevel =
        ref.watch(authSessionProvider).value?.profile?.trustLevel ?? 0;
    final meetupsAsync = ref.watch(openMeetupsProvider(intent));

    return Scaffold(
      backgroundColor: Colors.transparent,
      appBar: AppBar(title: const Text('OPEN MEETUPS')),
      body: Column(
        children: [
          // Switching the browsed intent used to mean leaving this page
          // entirely (back to Home's intent grid, then FIND MATCHES
          // again) — these tabs let it happen without leaving the list.
          _IntentTabsBar(
            selected: intent,
            trustLevel: trustLevel,
            onSelect: (picked) {
              if (!picked.isUnlockedFor(trustLevel)) {
                showSnack(
                  context,
                  '${picked.label} requires Level ${picked.requiredTrustLevel} trust. Verify your phone, personal email, and details in Profile to unlock it.',
                  type: ToastType.locked,
                );
                return;
              }
              ref.read(selectedIntentProvider.notifier).state = picked;
            },
          ),
          Expanded(
            child: _buildBody(context, ref, intent, trustLevel, meetupsAsync),
          ),
        ],
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    WidgetRef ref,
    IntentType intent,
    int trustLevel,
    AsyncValue<PagedResult<Meetup>> meetupsAsync,
  ) {
    return meetupsAsync.when(
      loading: () => const _MeetupsSkeleton(),
      error: (error, stack) => Center(
        child: Glass(
          radius: 20,
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(
                Icons.wifi_off_outlined,
                color: AppPalette.textSecondary,
                size: 28,
              ),
              const SizedBox(height: 10),
              const Text(
                'Could not load meetups.',
                style: TextStyle(color: AppPalette.textPrimary, fontSize: 13),
              ),
              const SizedBox(height: 14),
              GradientButton(
                label: 'RETRY',
                height: 40,
                onPressed: () => ref.invalidate(openMeetupsProvider(intent)),
              ),
            ],
          ),
        ),
      ),
      data: (page) {
        if (page.items.isEmpty) {
          return const Center(
            child: Text(
              'No open meetups for this intent yet.',
              style: TextStyle(color: AppPalette.textSecondary, fontSize: 13),
            ),
          );
        }
        return ListView.builder(
          padding: const EdgeInsets.fromLTRB(20, 4, 20, 100),
          itemCount: page.items.length,
          itemBuilder: (context, index) => _MeetupCard(
            meetup: page.items[index],
            trustLevel: trustLevel,
            onRequestToJoin: () =>
                _requestToJoin(context, ref, page.items[index]),
            onTap: () async {
              await Navigator.of(context).push(
                MaterialPageRoute(
                  builder: (_) =>
                      MeetupDetailPage(meetupId: page.items[index].id),
                ),
              );
              ref.invalidate(openMeetupsProvider(intent));
            },
          ),
        );
      },
    );
  }

  Future<void> _requestToJoin(
    BuildContext context,
    WidgetRef ref,
    Meetup meetup,
  ) async {
    try {
      await ref.read(meetupServiceProvider).requestToJoin(meetup.id);
      if (!context.mounted) return;
      showSnack(context, 'Request sent.', type: ToastType.success);
      ref.invalidate(openMeetupsProvider(meetup.intent));
    } catch (error) {
      if (!context.mounted) return;
      showSnack(
        context,
        error is MeetupException
            ? error.message
            : 'Something went wrong. Please try again.',
        type: ToastType.error,
      );
    }
  }
}

class _MeetupCard extends StatelessWidget {
  const _MeetupCard({
    required this.meetup,
    required this.trustLevel,
    required this.onRequestToJoin,
    required this.onTap,
  });

  final Meetup meetup;
  final int trustLevel;
  final VoidCallback onRequestToJoin;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final full = meetup.acceptedCount >= meetup.capacity;
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: GestureDetector(
        onTap: onTap,
        child: Glass(
          radius: 22,
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  ProfessionalAvatar(
                    name: meetup.hostFullName,
                    imageUrl: meetup.hostProfilePhotoUrl.isEmpty
                        ? null
                        : meetup.hostProfilePhotoUrl,
                    size: 48,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Flexible(
                              child: Text(
                                meetup.hostFullName,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: AppPalette.textPrimary,
                                  fontWeight: FontWeight.w600,
                                  fontSize: 15,
                                ),
                              ),
                            ),
                            const SizedBox(width: 6),
                            TrustLevelBadge(trustLevel: meetup.hostTrustLevel),
                            const SizedBox(width: 6),
                            StarRating(
                              average: meetup.hostRatingAverage,
                              count: meetup.hostRatingCount,
                            ),
                          ],
                        ),
                        const SizedBox(height: 2),
                        Text(
                          meetup.locationLabel,
                          style: const TextStyle(
                            color: AppPalette.textSecondary,
                            fontSize: 12,
                          ),
                        ),
                        const SizedBox(height: 6),
                        VerificationBadges(trustLevel: meetup.hostTrustLevel),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Row(
                children: [
                  Expanded(
                    child: Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        _tag(meetup.intent.label),
                        _tag(meetup.formattedWindow),
                        _tag(
                          '${meetup.acceptedCount}/${meetup.capacity} JOINED',
                        ),
                      ],
                    ),
                  ),
                  MeetupStatusBadge(status: meetup.status),
                ],
              ),
              const SizedBox(height: 14),
              if (meetup.isHostedByMe)
                const _StatusPill(
                  label: 'YOU\'RE HOSTING',
                  color: AppPalette.candyBlue,
                )
              else if (meetup.myRequestStatus != null)
                _StatusPill(
                  label: switch (meetup.myRequestStatus!) {
                    MeetupRequestStatus.pending => 'REQUEST PENDING',
                    MeetupRequestStatus.accepted => 'YOU\'RE IN',
                    MeetupRequestStatus.rejected => 'REQUEST DECLINED',
                    MeetupRequestStatus.withdrawn => 'WITHDRAWN',
                  },
                  color: switch (meetup.myRequestStatus!) {
                    MeetupRequestStatus.pending => AppPalette.candyBlue,
                    MeetupRequestStatus.accepted => AppPalette.verified,
                    MeetupRequestStatus.rejected => AppPalette.danger,
                    MeetupRequestStatus.withdrawn => AppPalette.textSecondary,
                  },
                )
              else if (!meetup.intent.isUnlockedFor(trustLevel))
                // Level 0/under-trust (ADR-014's Level 0 read-only audit,
                // Step 6) — disabled with an upsell instead of letting the
                // tap reach the server just to bounce off its 403
                // (services/meetup/internal/service/trustgate.go's floor is
                // the actual enforcement; this is UX only, same discipline
                // as ADR-013).
                Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const GradientButton(
                      label: 'REQUEST TO JOIN',
                      height: 42,
                      onPressed: null,
                    ),
                    const SizedBox(height: 6),
                    Text(
                      'Requires Level ${meetup.intent.requiredTrustLevel} trust',
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        color: AppPalette.textSecondary,
                        fontSize: 11,
                      ),
                    ),
                  ],
                )
              else
                GradientButton(
                  label: full ? 'FULL' : 'REQUEST TO JOIN',
                  height: 42,
                  onPressed: full ? null : onRequestToJoin,
                ),
            ],
          ),
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
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(vertical: 12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        color: color.withValues(alpha: 0.08),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Center(
        child: Text(
          label,
          style: TextStyle(
            color: color,
            fontWeight: FontWeight.w800,
            letterSpacing: 1.2,
            fontSize: 11,
          ),
        ),
      ),
    );
  }
}

class _MeetupsSkeleton extends StatelessWidget {
  const _MeetupsSkeleton();

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
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  const SkeletonBox(width: 48, height: 48, radius: 24),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const SkeletonBox(
                          width: 120,
                          height: 12,
                          opacity: 0.08,
                        ),
                        const SizedBox(height: 6),
                        const SkeletonBox(width: 80, height: 10),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              LayoutBuilder(
                builder: (context, constraints) => SkeletonBox(
                  width: constraints.maxWidth,
                  height: 38,
                  radius: 12,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// One chip per [IntentType], letting the browsed intent be switched
/// without leaving this page — matches this page's own filter
/// (`selectedIntentProvider`), same locked/unlocked visual language as
/// HomePage's IntentTile (dimmed + a lock icon, not hidden entirely).
class _IntentTabsBar extends StatelessWidget {
  const _IntentTabsBar({
    required this.selected,
    required this.trustLevel,
    required this.onSelect,
  });

  final IntentType selected;
  final int trustLevel;
  final void Function(IntentType intent) onSelect;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 56,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.fromLTRB(20, 4, 20, 12),
        itemCount: IntentType.values.length,
        separatorBuilder: (context, index) => const SizedBox(width: 8),
        itemBuilder: (context, index) {
          final intent = IntentType.values[index];
          final isSelected = intent == selected;
          final locked = !intent.isUnlockedFor(trustLevel);
          return GestureDetector(
            onTap: () => onSelect(intent),
            child: Glass(
              radius: 20,
              padding: const EdgeInsets.symmetric(horizontal: 14),
              tint: isSelected
                  ? AppPalette.candyBlue.withValues(alpha: 0.15)
                  : null,
              border: isSelected
                  ? AppPalette.candyBlue.withValues(alpha: 0.6)
                  : null,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    locked ? Icons.lock_outline : intent.icon,
                    size: 15,
                    color: locked
                        ? AppPalette.textSecondary
                        : (isSelected
                              ? AppPalette.candyBlue
                              : AppPalette.textPrimary),
                  ),
                  const SizedBox(width: 6),
                  Text(
                    intent.label,
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 0.6,
                      color: locked
                          ? AppPalette.textSecondary
                          : (isSelected
                                ? AppPalette.candyBlue
                                : AppPalette.textPrimary),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}
