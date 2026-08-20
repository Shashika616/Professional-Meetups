import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/meetup_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/core/widgets/meetup_status_badge.dart';
import 'package:professional_connections_platform/core/widgets/professional_avatar.dart';
import 'package:professional_connections_platform/core/widgets/skeleton_box.dart';
import 'package:professional_connections_platform/core/widgets/star_rating.dart';
import 'package:professional_connections_platform/core/widgets/trust_level_badge.dart';
import 'package:professional_connections_platform/core/widgets/verification_badges.dart';
import 'package:professional_connections_platform/features/meetups/meetup_detail_page.dart';
import 'package:professional_connections_platform/features/meetups/widgets/host_meetup_controls.dart';

/// Meetups the signed-in user hosts or has requested to join, reachable in
/// one tap from Home (frontend/meetup-scheduling-PLAN.md Step 8). Tapping a
/// hosted meetup opens its request-management view; tapping a requested
/// meetup opens the ordinary detail page.
class MyMeetupsPage extends ConsumerWidget {
  const MyMeetupsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final myMeetupsAsync = ref.watch(myMeetupsProvider);

    // Pushed as its own route from Home, not one of AppShell's bottom-nav
    // tabs — AppShell's own AppBackground wrap (app_shell.dart) only
    // covers pages[currentIndex], so a page reached via Navigator.push
    // needs this itself or it renders on a plain black canvas instead of
    // the rest of the app's glassmorphism background.
    return AppBackground(
      child: DefaultTabController(
        length: 2,
        child: Scaffold(
          backgroundColor: Colors.transparent,
          appBar: AppBar(
            title: const Text('MY MEETUPS'),
            bottom: const TabBar(
              tabs: [
                Tab(text: 'HOSTING'),
                Tab(text: 'REQUESTED'),
              ],
            ),
          ),
          body: myMeetupsAsync.when(
            loading: () => const _MyMeetupsSkeleton(),
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
                      'Could not load your meetups.',
                      style: TextStyle(
                        color: AppPalette.textPrimary,
                        fontSize: 13,
                      ),
                    ),
                    const SizedBox(height: 14),
                    GradientButton(
                      label: 'RETRY',
                      height: 40,
                      onPressed: () => ref.invalidate(myMeetupsProvider),
                    ),
                  ],
                ),
              ),
            ),
            data: (result) => TabBarView(
              children: [
                _MeetupList(
                  meetups: result.hosted,
                  emptyMessage: 'You aren\'t hosting any meetups yet.',
                  onTap: (meetup) async {
                    await Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => _RequestManagementPage(meetup: meetup),
                      ),
                    );
                    ref.invalidate(myMeetupsProvider);
                  },
                ),
                _MeetupList(
                  meetups: result.requested,
                  emptyMessage:
                      'You haven\'t requested to join any meetups yet.',
                  onTap: (meetup) async {
                    await Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => MeetupDetailPage(meetupId: meetup.id),
                      ),
                    );
                    ref.invalidate(myMeetupsProvider);
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Shown while [myMeetupsProvider] resolves, instead of a bare spinner —
/// mirrors the card shape [_MeetupList] renders once data actually
/// arrives, so the list doesn't visibly "pop" from blank to content.
class _MyMeetupsSkeleton extends StatelessWidget {
  const _MyMeetupsSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 100),
      itemCount: 3,
      itemBuilder: (context, index) => Padding(
        padding: const EdgeInsets.only(bottom: 14),
        child: Glass(
          radius: 20,
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const SkeletonBox(width: 70, height: 11, opacity: 0.08),
                  const Spacer(),
                  const SkeletonBox(width: 28, height: 16, radius: 8),
                ],
              ),
              const SizedBox(height: 10),
              const SkeletonBox(width: 100, height: 15, opacity: 0.08),
              const SizedBox(height: 6),
              const SkeletonBox(width: 140, height: 12),
              const SizedBox(height: 10),
              const SkeletonBox(width: 90, height: 11),
            ],
          ),
        ),
      ),
    );
  }
}

/// Open vs. History is a second, orthogonal axis on top of the page's own
/// Hosting/Requested tabs (ADR-016 revives `completed`, which needs
/// somewhere to show up) — filtered client-side from the same
/// already-fetched list, not a second round trip, and not a second level
/// of [TabController] nesting for what's really just a toggle.
class _MeetupList extends StatefulWidget {
  const _MeetupList({
    required this.meetups,
    required this.emptyMessage,
    required this.onTap,
  });

  final List<Meetup> meetups;
  final String emptyMessage;
  final void Function(Meetup meetup) onTap;

  @override
  State<_MeetupList> createState() => _MeetupListState();
}

class _MeetupListState extends State<_MeetupList> {
  bool _showHistory = false;

  static bool _isOpen(Meetup m) =>
      m.status == MeetupStatus.open || m.status == MeetupStatus.full;
  static bool _isHistory(Meetup m) =>
      m.status == MeetupStatus.completed || m.status == MeetupStatus.cancelled;

  @override
  Widget build(BuildContext context) {
    final filtered = widget.meetups
        .where(_showHistory ? _isHistory : _isOpen)
        .toList();

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
          child: Row(
            children: [
              Expanded(
                child: _OpenHistoryToggle(
                  label: 'OPEN',
                  selected: !_showHistory,
                  onTap: () => setState(() => _showHistory = false),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: _OpenHistoryToggle(
                  label: 'HISTORY',
                  selected: _showHistory,
                  onTap: () => setState(() => _showHistory = true),
                ),
              ),
            ],
          ),
        ),
        Expanded(
          child: filtered.isEmpty
              ? Center(
                  child: Text(
                    _showHistory ? 'Nothing here yet.' : widget.emptyMessage,
                    style: const TextStyle(
                      color: AppPalette.textSecondary,
                      fontSize: 13,
                    ),
                  ),
                )
              : _MeetupListView(meetups: filtered, onTap: widget.onTap),
        ),
      ],
    );
  }
}

class _OpenHistoryToggle extends StatelessWidget {
  const _OpenHistoryToggle({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 10),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          color: selected
              ? AppPalette.candyBlue.withValues(alpha: 0.15)
              : Colors.transparent,
          border: Border.all(
            color: selected ? AppPalette.candyBlue : AppPalette.glassBorder,
          ),
        ),
        child: Text(
          label,
          textAlign: TextAlign.center,
          style: TextStyle(
            color: selected ? AppPalette.candyBlue : AppPalette.textSecondary,
            fontWeight: FontWeight.w800,
            fontSize: 12,
            letterSpacing: 0.8,
          ),
        ),
      ),
    );
  }
}

class _MeetupListView extends StatelessWidget {
  const _MeetupListView({required this.meetups, required this.onTap});

  final List<Meetup> meetups;
  final void Function(Meetup meetup) onTap;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 100),
      itemCount: meetups.length,
      itemBuilder: (context, index) {
        final meetup = meetups[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 14),
          child: GestureDetector(
            onTap: () => onTap(meetup),
            child: Glass(
              radius: 20,
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          meetup.intent.label,
                          style: const TextStyle(
                            color: AppPalette.candyBlue,
                            fontSize: 11,
                            fontWeight: FontWeight.w800,
                            letterSpacing: 1.2,
                          ),
                        ),
                      ),
                      MeetupStatusBadge(status: meetup.status),
                      const SizedBox(width: 6),
                      TrustLevelBadge(trustLevel: meetup.hostTrustLevel),
                      const SizedBox(width: 6),
                      StarRating(
                        average: meetup.hostRatingAverage,
                        count: meetup.hostRatingCount,
                      ),
                    ],
                  ),
                  const SizedBox(height: 6),
                  VerificationBadges(trustLevel: meetup.hostTrustLevel),
                  const SizedBox(height: 6),
                  Text(
                    meetup.formattedWindow,
                    style: const TextStyle(
                      color: AppPalette.textPrimary,
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    meetup.locationLabel,
                    style: const TextStyle(
                      color: AppPalette.textSecondary,
                      fontSize: 12,
                    ),
                  ),
                  const SizedBox(height: 10),
                  _statusRow(meetup),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _statusRow(Meetup meetup) {
    if (meetup.isHostedByMe) {
      return Text(
        '${meetup.acceptedCount}/${meetup.capacity} confirmed',
        style: const TextStyle(color: AppPalette.textSecondary, fontSize: 12),
      );
    }
    final status = meetup.myRequestStatus;
    if (status == null) {
      return const SizedBox.shrink();
    }
    final (label, color) = switch (status) {
      MeetupRequestStatus.pending => ('REQUEST PENDING', AppPalette.candyBlue),
      MeetupRequestStatus.accepted => ('YOU\'RE IN', AppPalette.verified),
      MeetupRequestStatus.rejected =>
        meetup.myRequestAutoRejected
            ? ('NOT SELECTED — MEETUP FILLED UP', AppPalette.textSecondary)
            : ('DECLINED BY HOST', AppPalette.danger),
      MeetupRequestStatus.withdrawn => ('WITHDRAWN', AppPalette.textSecondary),
    };
    return Text(
      label,
      style: TextStyle(
        color: color,
        fontWeight: FontWeight.w800,
        letterSpacing: 0.6,
        fontSize: 11,
      ),
    );
  }
}

/// A host's view of every request on one of their meetups — Accept/Reject
/// per pending request (frontend/meetup-scheduling-PLAN.md Step 8).
class _RequestManagementPage extends ConsumerStatefulWidget {
  const _RequestManagementPage({required this.meetup});

  final Meetup meetup;

  @override
  ConsumerState<_RequestManagementPage> createState() =>
      _RequestManagementPageState();
}

class _RequestManagementPageState
    extends ConsumerState<_RequestManagementPage> {
  late Meetup _meetup;
  List<MeetupRequestModel>? _requests;
  bool _loading = true;
  String? _loadError;

  @override
  void initState() {
    super.initState();
    _meetup = widget.meetup;
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final service = ref.read(meetupServiceProvider);
      final requests = await service.listMeetupRequests(_meetup.id);
      // Also re-fetches the meetup itself, not just its requests — an
      // Accept here bumps acceptedCount server-side, which HostMeetupControls
      // needs to know about immediately: it hides CANCEL once a request has
      // been accepted (the backend rejects cancelling in that state), and a
      // stale local _meetup would otherwise keep showing an action that's
      // now guaranteed to 409.
      final meetup = await service.getMeetup(_meetup.id);
      if (!mounted) return;
      setState(() {
        _requests = requests;
        _meetup = meetup;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loadError = error is MeetupException
            ? error.message
            : 'Something went wrong. Please try again.';
        _loading = false;
      });
    }
  }

  Future<void> _respond(MeetupRequestModel request, bool accept) async {
    try {
      await ref
          .read(meetupServiceProvider)
          .respondToRequest(request.id, accept: accept);
      if (!mounted) return;
      showSnack(
        context,
        accept ? 'Request accepted.' : 'Request rejected.',
        type: ToastType.success,
      );
      await _load();
    } catch (error) {
      if (!mounted) return;
      showSnack(
        context,
        error is MeetupException
            ? error.message
            : 'Something went wrong. Please try again.',
        type: ToastType.error,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return AppBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(title: const Text('REQUESTS')),
        body: SafeArea(
          child: Column(
            children: [
              // Was previously missing entirely — this screen showed
              // requester cards with no context about the meetup itself,
              // and (the bug this addendum fixes) no way to close or
              // cancel it, even though tapping the calendar icon →
              // Hosting → a meetup is the normal way a host lands here
              // (ADR-016 addendum, 2026-08-20).
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
                child: Glass(
                  radius: 18,
                  padding: const EdgeInsets.all(14),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              _meetup.intent.label,
                              style: const TextStyle(
                                color: AppPalette.candyBlue,
                                fontSize: 11,
                                fontWeight: FontWeight.w800,
                                letterSpacing: 1.2,
                              ),
                            ),
                          ),
                          MeetupStatusBadge(status: _meetup.status),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Text(
                        _meetup.formattedWindow,
                        style: const TextStyle(
                          color: AppPalette.textPrimary,
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      HostMeetupControls(
                        meetup: _meetup,
                        onChanged: (updated) =>
                            setState(() => _meetup = updated),
                      ),
                    ],
                  ),
                ),
              ),
              Expanded(
                child: _loading
                    ? const _RequestsSkeleton()
                    : _loadError != null
                    ? Center(
                        child: Text(
                          _loadError!,
                          style: const TextStyle(
                            color: AppPalette.textSecondary,
                          ),
                        ),
                      )
                    : _buildRequests(),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildRequests() {
    final requests = _requests!;
    if (requests.isEmpty) {
      return const Center(
        child: Text(
          'No one has requested to join yet.',
          style: TextStyle(color: AppPalette.textSecondary, fontSize: 13),
        ),
      );
    }
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      itemCount: requests.length,
      itemBuilder: (context, index) {
        final request = requests[index];
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: Glass(
            radius: 18,
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    ProfessionalAvatar(
                      name: request.requesterFullName,
                      imageUrl: request.requesterProfilePhotoUrl.isEmpty
                          ? null
                          : request.requesterProfilePhotoUrl,
                      size: 40,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        request.requesterFullName,
                        style: const TextStyle(
                          color: AppPalette.textPrimary,
                          fontWeight: FontWeight.w600,
                          fontSize: 14,
                        ),
                      ),
                    ),
                    TrustLevelBadge(trustLevel: request.requesterTrustLevel),
                    const SizedBox(width: 6),
                    StarRating(
                      average: request.requesterRatingAverage,
                      count: request.requesterRatingCount,
                    ),
                  ],
                ),
                const SizedBox(height: 10),
                VerificationBadges(trustLevel: request.requesterTrustLevel),
                const SizedBox(height: 12),
                if (request.status == MeetupRequestStatus.pending)
                  Row(
                    children: [
                      Expanded(
                        child: GradientButton(
                          label: 'ACCEPT',
                          height: 40,
                          onPressed: () => _respond(request, true),
                        ),
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: OutlinedButton(
                          onPressed: () => _respond(request, false),
                          style: OutlinedButton.styleFrom(
                            minimumSize: const Size.fromHeight(40),
                            side: const BorderSide(
                              color: AppPalette.glassBorder,
                            ),
                          ),
                          child: const Text(
                            'REJECT',
                            style: TextStyle(
                              color: AppPalette.textSecondary,
                              fontSize: 12,
                            ),
                          ),
                        ),
                      ),
                    ],
                  )
                else
                  Text(
                    switch (request.status) {
                      MeetupRequestStatus.accepted => 'ACCEPTED',
                      MeetupRequestStatus.rejected =>
                        request.autoRejected
                            ? 'AUTO-REJECTED (CAPACITY FULL)'
                            : 'REJECTED',
                      MeetupRequestStatus.withdrawn => 'WITHDRAWN',
                      MeetupRequestStatus.pending => 'PENDING',
                    },
                    style: const TextStyle(
                      color: AppPalette.textSecondary,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 0.6,
                      fontSize: 11,
                    ),
                  ),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Shown while [MeetupService.listMeetupRequests] resolves — mirrors the
/// card shape `_buildRequests()` renders once data actually arrives.
class _RequestsSkeleton extends StatelessWidget {
  const _RequestsSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      itemCount: 3,
      itemBuilder: (context, index) => Padding(
        padding: const EdgeInsets.only(bottom: 12),
        child: Glass(
          radius: 18,
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  const SkeletonBox(width: 40, height: 40, radius: 20),
                  const SizedBox(width: 12),
                  const Expanded(
                    child: SkeletonBox(
                      width: double.infinity,
                      height: 14,
                      opacity: 0.08,
                    ),
                  ),
                  const SizedBox(width: 10),
                  const SkeletonBox(width: 28, height: 16, radius: 8),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: SkeletonBox(
                      width: double.infinity,
                      height: 40,
                      radius: 10,
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: SkeletonBox(
                      width: double.infinity,
                      height: 40,
                      radius: 10,
                      opacity: 0.04,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
