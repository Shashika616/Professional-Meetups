import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/meetup_status_badge.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/features/meetups/meetup_detail_page.dart';

/// A real preview of the soonest upcoming confirmed meetup — a meetup the
/// user hosts, or has an accepted request on (frontend/meetup-scheduling-
/// PLAN.md Step 9). Hides itself entirely when there isn't one, rather than
/// showing stale/fake content.
class UpcomingMeetupCard extends ConsumerWidget {
  const UpcomingMeetupCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final myMeetups = ref.watch(myMeetupsProvider).value;
    final meetup = myMeetups == null ? null : _soonestConfirmed(myMeetups);
    if (meetup == null) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionLabel('YOUR NEXT MEETUP'),
          const SizedBox(height: 16),
          GestureDetector(
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => MeetupDetailPage(meetupId: meetup.id),
              ),
            ),
            child: Glass(
              radius: 20,
              padding: const EdgeInsets.all(16),
              // crossAxisAlignment.start (not the Row default, center) so
              // the icon stays pinned to the top when the location line
              // below wraps to two lines instead of getting cut off — see
              // that Text's own comment for why it wraps rather than
              // ellipsizes.
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: AppPalette.candyBlue.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: const Icon(
                      Icons.coffee_rounded,
                      color: AppPalette.candyBlue,
                      size: 20,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Wrap(
                          spacing: 8,
                          runSpacing: 6,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 3,
                              ),
                              decoration: BoxDecoration(
                                color: AppPalette.candyBlue.withValues(
                                  alpha: 0.12,
                                ),
                                borderRadius: BorderRadius.circular(8),
                              ),
                              child: Text(
                                meetup.intent.label,
                                style: const TextStyle(
                                  fontSize: 9,
                                  letterSpacing: 1.0,
                                  fontWeight: FontWeight.w800,
                                  color: AppPalette.candyBlue,
                                ),
                              ),
                            ),
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 3,
                              ),
                              decoration: BoxDecoration(
                                color: AppPalette.verified.withValues(
                                  alpha: 0.12,
                                ),
                                borderRadius: BorderRadius.circular(8),
                                border: Border.all(
                                  color: AppPalette.verified.withValues(
                                    alpha: 0.35,
                                  ),
                                ),
                              ),
                              child: const Text(
                                'CONFIRMED',
                                style: TextStyle(
                                  fontSize: 8,
                                  letterSpacing: 1.2,
                                  fontWeight: FontWeight.w800,
                                  color: AppPalette.verified,
                                ),
                              ),
                            ),
                            MeetupStatusBadge(status: meetup.status),
                          ],
                        ),
                        const SizedBox(height: 8),
                        // The host's full name on its own line — cramming
                        // it onto one ellipsized line together with the
                        // intent label (the old layout) is exactly what
                        // truncated real names down to "Shashik...".
                        Text(
                          meetup.hostFullName,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: AppPalette.textPrimary,
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          meetup.formattedWindow,
                          style: const TextStyle(
                            color: AppPalette.textSecondary,
                            fontSize: 12,
                          ),
                        ),
                        const SizedBox(height: 2),
                        // Location wraps instead of ellipsizing — where to
                        // actually go is crucial info, not something to
                        // hide behind a "...".
                        Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Padding(
                              padding: EdgeInsets.only(top: 2),
                              child: Icon(
                                Icons.place_outlined,
                                size: 12,
                                color: AppPalette.textSecondary,
                              ),
                            ),
                            const SizedBox(width: 4),
                            Expanded(
                              child: Text(
                                meetup.locationLabel,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: AppPalette.textSecondary,
                                  fontSize: 12,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Meetup? _soonestConfirmed(
    ({List<Meetup> hosted, List<Meetup> requested}) myMeetups,
  ) {
    bool isLive(Meetup m) =>
        m.status == MeetupStatus.open || m.status == MeetupStatus.full;
    // A window is "upcoming" while it hasn't ended yet — windowEnd, not
    // windowStart, since a meetup that's already started but still running
    // is still the one worth showing (ADR-016: every meetup has a real
    // window now, no more nullable scheduledFor to fall back on).
    bool isUpcoming(Meetup m) => m.windowEnd.isAfter(DateTime.now());

    final candidates = [
      ...myMeetups.hosted.where((m) => isLive(m) && isUpcoming(m)),
      ...myMeetups.requested.where(
        (m) =>
            m.myRequestStatus == MeetupRequestStatus.accepted &&
            isLive(m) &&
            isUpcoming(m),
      ),
    ];
    if (candidates.isEmpty) return null;
    candidates.sort((a, b) => a.windowStart.compareTo(b.windowStart));
    return candidates.first;
  }
}
