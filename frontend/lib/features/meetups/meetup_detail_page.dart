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
import 'package:professional_connections_platform/core/widgets/skeleton_box.dart';
import 'package:professional_connections_platform/features/meetups/widgets/host_meetup_controls.dart';
import 'package:professional_connections_platform/features/meetups/widgets/rating_prompt.dart';

/// Replaces `UpcomingMeetupCard`'s hardcoded "Coffee with Sachini Fernando"
/// text and its two stub taps with a real detail page bound to real
/// [Meetup] data, including the Safety Gate sub-flow (ADR-013 § 3,
/// frontend/meetup-scheduling-PLAN.md Step 9).
class MeetupDetailPage extends ConsumerStatefulWidget {
  const MeetupDetailPage({super.key, required this.meetupId});

  final String meetupId;

  @override
  ConsumerState<MeetupDetailPage> createState() => _MeetupDetailPageState();
}

class _MeetupDetailPageState extends ConsumerState<MeetupDetailPage> {
  Meetup? _meetup;
  SafetyState? _safetyState;
  bool _loading = true;
  String? _loadError;

  /// Set once the viewer confirms (SubmitMeetupFeedback, happened=true)
  /// that this meetup happened — the trigger for showing [RatingPrompt]
  /// (ADR-015).
  bool _feedbackHappened = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final meetup = await ref
          .read(meetupServiceProvider)
          .getMeetup(widget.meetupId);
      SafetyState? safetyState;
      // Safety Gate state only exists once the meetup has at least one
      // accepted request (ADR-013 § 3) — MeetupNotFoundException here just
      // means "not started yet," not a real error.
      if (meetup.acceptedCount > 0) {
        try {
          safetyState = await ref
              .read(meetupServiceProvider)
              .getSafetyState(widget.meetupId);
        } on MeetupNotFoundException {
          // not started yet — safetyState stays null.
        }
      }
      if (!mounted) return;
      setState(() {
        _meetup = meetup;
        _safetyState = safetyState;
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

  Future<void> _requestToJoin() async {
    try {
      await ref.read(meetupServiceProvider).requestToJoin(widget.meetupId);
      if (!mounted) return;
      showSnack(context, 'Request sent.', type: ToastType.success);
      await _load();
    } catch (error) {
      if (mounted) {
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

  /// Disables REQUEST TO JOIN with an upsell instead of letting a Level-0
  /// (or otherwise under-trust) tap reach the server just to bounce off its
  /// 403 — mirrors the same trust-gate pattern `matches_page.dart` already
  /// uses for its intent tabs (ADR-014's Level 0 read-only audit, Step 6).
  /// The server-side check in `services/meetup/internal/service/
  /// trustgate.go` remains the one that's actually enforced; this is UX
  /// only, same discipline as ADR-013.
  Widget _buildJoinAction(BuildContext context, Meetup meetup) {
    final trustLevel =
        ref.watch(authSessionProvider).value?.profile?.trustLevel ?? 0;
    if (meetup.intent.isUnlockedFor(trustLevel)) {
      return GradientButton(
        label: 'REQUEST TO JOIN',
        onPressed: _requestToJoin,
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const GradientButton(label: 'REQUEST TO JOIN', onPressed: null),
        const SizedBox(height: 8),
        Text(
          '${meetup.intent.label} requires Level ${meetup.intent.requiredTrustLevel} trust. Verify your phone, personal email, and details in Profile to unlock it.',
          textAlign: TextAlign.center,
          style: const TextStyle(color: AppPalette.textSecondary, fontSize: 12),
        ),
      ],
    );
  }

  Future<void> _acknowledgeChecklist() async {
    try {
      final state = await ref
          .read(meetupServiceProvider)
          .acknowledgeSafetyChecklist(widget.meetupId);
      if (!mounted) return;
      setState(() => _safetyState = state);
    } catch (error) {
      if (mounted) {
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

  Future<void> _setLiveLocationOptIn(bool optIn) async {
    try {
      final state = await ref
          .read(meetupServiceProvider)
          .setLiveLocationOptIn(widget.meetupId, optIn);
      if (!mounted) return;
      setState(() => _safetyState = state);
    } catch (error) {
      if (mounted) {
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

  Future<void> _checkIn() async {
    try {
      final state = await ref
          .read(meetupServiceProvider)
          .checkIn(widget.meetupId);
      if (!mounted) return;
      setState(() => _safetyState = state);
      showSnack(context, 'Checked in.', type: ToastType.success);
    } catch (error) {
      if (mounted) {
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

  Future<void> _submitFeedback({
    required bool happened,
    bool? feltSafe,
    bool? profileAccurate,
    bool? wouldMeetAgain,
  }) async {
    // Second, optional step (ADR-016) — a free-text note after the
    // happened/didn't-happen choice. Neither Save nor Skip (nor dismissing
    // the sheet) blocks reaching the actual submit call below; only Save
    // with real text carries a non-null value through.
    final notes = await _showFeedbackNoteSheet();
    if (!mounted) return;
    try {
      await ref
          .read(meetupServiceProvider)
          .submitMeetupFeedback(
            widget.meetupId,
            happened: happened,
            feltSafe: feltSafe,
            profileAccurate: profileAccurate,
            wouldMeetAgain: wouldMeetAgain,
            notes: notes,
          );
      if (!mounted) return;
      showSnack(context, 'Thanks for the feedback.', type: ToastType.success);
      if (happened) setState(() => _feedbackHappened = true);
    } catch (error) {
      if (mounted) {
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

  Future<String?> _showFeedbackNoteSheet() {
    final controller = TextEditingController();
    return showModalBottomSheet<String?>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => _FeedbackNoteSheet(controller: controller),
    ).whenComplete(controller.dispose);
  }

  // Every meetup has a real window now, "today" included (ADR-016) — no
  // more isToday-means-always-open special case. Opens 10 minutes before
  // windowStart, same grace period as before.
  bool get _checkInWindowOpen {
    final meetup = _meetup;
    if (meetup == null) return true;
    return DateTime.now().isAfter(
      meetup.windowStart.subtract(const Duration(minutes: 10)),
    );
  }

  @override
  Widget build(BuildContext context) {
    // Pushed as its own route from Home/Matches, not one of AppShell's
    // bottom-nav tabs — see MyMeetupsPage's matching comment for why this
    // needs its own AppBackground wrap rather than inheriting AppShell's.
    return AppBackground(
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: AppBar(title: const Text('MEETUP')),
        body: SafeArea(
          child: _loading
              ? const _MeetupDetailSkeleton()
              : _loadError != null
              ? Center(
                  child: Text(
                    _loadError!,
                    style: const TextStyle(color: AppPalette.textSecondary),
                  ),
                )
              : _buildContent(context, _meetup!),
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, Meetup meetup) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
      children: [
        Glass(
          radius: 20,
          padding: const EdgeInsets.all(18),
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
                        letterSpacing: 1.4,
                      ),
                    ),
                  ),
                  MeetupStatusBadge(status: meetup.status),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                meetup.formattedWindow,
                style: const TextStyle(
                  color: AppPalette.textPrimary,
                  fontSize: 20,
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                meetup.locationLabel,
                style: const TextStyle(
                  color: AppPalette.textSecondary,
                  fontSize: 13,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                'Hosted by ${meetup.hostFullName} • Level ${meetup.hostTrustLevel}',
                style: const TextStyle(
                  color: AppPalette.textSecondary,
                  fontSize: 12,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                '${meetup.acceptedCount}/${meetup.capacity} confirmed',
                style: const TextStyle(
                  color: AppPalette.textSecondary,
                  fontSize: 12,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),
        if (!meetup.isHostedByMe && meetup.myRequestStatus == null)
          _buildJoinAction(context, meetup)
        else if (!meetup.isHostedByMe && meetup.myRequestStatus != null)
          _RequestStatusBanner(status: meetup.myRequestStatus!),
        // Host-only Cancel/Close actions (ADR-016 + its 2026-08-20
        // addendum) — extracted into one shared widget, also used by the
        // "Hosting" tab's request-management screen, so this logic exists
        // once. Independent of the Safety Gate section below (rating
        // eligibility stays gated on each participant's own
        // confirmed-attendance feedback, ADR-015, unaffected by either
        // action).
        HostMeetupControls(
          meetup: meetup,
          onChanged: (updated) => setState(() => _meetup = updated),
        ),
        if (meetup.acceptedCount > 0) ...[
          const SizedBox(height: 24),
          _SafetyGateSection(
            safetyState: _safetyState,
            checkInWindowOpen: _checkInWindowOpen,
            onAcknowledgeChecklist: _acknowledgeChecklist,
            onSetLiveLocationOptIn: _setLiveLocationOptIn,
            onCheckIn: _checkIn,
            onSubmitFeedback: _submitFeedback,
          ),
        ],
        if (_feedbackHappened) ...[
          const SizedBox(height: 24),
          RatingPrompt(meetupId: widget.meetupId),
        ],
      ],
    );
  }
}

/// Shown while [MeetupService.getMeetup] resolves — mirrors the summary
/// card + action button shape `_buildContent` renders once data actually
/// arrives, instead of a bare spinner.
class _MeetupDetailSkeleton extends StatelessWidget {
  const _MeetupDetailSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
      children: [
        Glass(
          radius: 20,
          padding: const EdgeInsets.all(18),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SkeletonBox(width: 70, height: 11, opacity: 0.08),
              const SizedBox(height: 10),
              const SkeletonBox(width: 140, height: 20, opacity: 0.08),
              const SizedBox(height: 8),
              const SkeletonBox(width: 180, height: 13),
              const SizedBox(height: 14),
              const SkeletonBox(width: 160, height: 12),
              const SizedBox(height: 4),
              const SkeletonBox(width: 100, height: 12),
            ],
          ),
        ),
        const SizedBox(height: 20),
        LayoutBuilder(
          builder: (context, constraints) =>
              SkeletonBox(width: constraints.maxWidth, height: 48, radius: 14),
        ),
      ],
    );
  }
}

class _RequestStatusBanner extends StatelessWidget {
  const _RequestStatusBanner({required this.status});

  final MeetupRequestStatus status;

  @override
  Widget build(BuildContext context) {
    final (label, color) = switch (status) {
      MeetupRequestStatus.pending => ('REQUEST PENDING', AppPalette.candyBlue),
      MeetupRequestStatus.accepted => ('YOU\'RE IN', AppPalette.verified),
      MeetupRequestStatus.rejected => ('REQUEST DECLINED', AppPalette.danger),
      MeetupRequestStatus.withdrawn => ('WITHDRAWN', AppPalette.textSecondary),
    };
    return Glass(
      radius: 16,
      padding: const EdgeInsets.symmetric(vertical: 14),
      tint: color.withValues(alpha: 0.08),
      border: color.withValues(alpha: 0.3),
      child: Center(
        child: Text(
          label,
          style: TextStyle(
            color: color,
            fontWeight: FontWeight.w800,
            letterSpacing: 1.4,
            fontSize: 12,
          ),
        ),
      ),
    );
  }
}

/// The Safety Gate sub-flow: checklist → optional live-location → check-in
/// → post-meetup feedback, in that order (Safety UX Flows.md's step order,
/// also enforced server-side — CheckIn rejects if the checklist hasn't
/// been acknowledged yet).
class _SafetyGateSection extends StatelessWidget {
  const _SafetyGateSection({
    required this.safetyState,
    required this.checkInWindowOpen,
    required this.onAcknowledgeChecklist,
    required this.onSetLiveLocationOptIn,
    required this.onCheckIn,
    required this.onSubmitFeedback,
  });

  final SafetyState? safetyState;
  final bool checkInWindowOpen;
  final VoidCallback onAcknowledgeChecklist;
  final void Function(bool optIn) onSetLiveLocationOptIn;
  final VoidCallback onCheckIn;
  final void Function({
    required bool happened,
    bool? feltSafe,
    bool? profileAccurate,
    bool? wouldMeetAgain,
  })
  onSubmitFeedback;

  @override
  Widget build(BuildContext context) {
    final acknowledged = safetyState?.checklistAcknowledged ?? false;
    final checkedIn = safetyState?.checkedIn ?? false;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'SAFETY GATE',
          style: TextStyle(
            color: AppPalette.textSecondary,
            fontSize: 11,
            fontWeight: FontWeight.w800,
            letterSpacing: 1.6,
          ),
        ),
        const SizedBox(height: 12),
        Glass(
          radius: 18,
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Before you go',
                style: TextStyle(
                  color: AppPalette.textPrimary,
                  fontWeight: FontWeight.w700,
                  fontSize: 14,
                ),
              ),
              const SizedBox(height: 8),
              const _ChecklistItem('Meet in a public place'),
              const _ChecklistItem(
                'Tell a trusted contact where you\'re going',
              ),
              const _ChecklistItem('Keep first meetings short'),
              const _ChecklistItem('Never share OTP codes or send money'),
              const SizedBox(height: 12),
              if (!acknowledged)
                GradientButton(
                  label: 'I UNDERSTAND',
                  height: 44,
                  onPressed: onAcknowledgeChecklist,
                )
              else
                const _DoneRow('Checklist acknowledged'),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Glass(
          radius: 18,
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              const Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Share live location',
                      style: TextStyle(
                        color: AppPalette.textPrimary,
                        fontWeight: FontWeight.w700,
                        fontSize: 13,
                      ),
                    ),
                    SizedBox(height: 2),
                    Text(
                      'Optional. Only for the duration of this meetup.',
                      style: TextStyle(
                        color: AppPalette.textSecondary,
                        fontSize: 11,
                      ),
                    ),
                  ],
                ),
              ),
              Switch(
                value: safetyState?.liveLocationOptIn ?? false,
                onChanged: onSetLiveLocationOptIn,
                activeTrackColor: AppPalette.candyBlue,
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Glass(
          radius: 18,
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Check in',
                style: TextStyle(
                  color: AppPalette.textPrimary,
                  fontWeight: FontWeight.w700,
                  fontSize: 14,
                ),
              ),
              const SizedBox(height: 8),
              if (checkedIn)
                const _DoneRow('Checked in')
              else if (!acknowledged)
                const Text(
                  'Acknowledge the checklist above first.',
                  style: TextStyle(
                    color: AppPalette.textSecondary,
                    fontSize: 12,
                  ),
                )
              else if (!checkInWindowOpen)
                const Text(
                  'Check-in opens 10 minutes before the meetup.',
                  style: TextStyle(
                    color: AppPalette.textSecondary,
                    fontSize: 12,
                  ),
                )
              else
                GradientButton(
                  label: 'CHECK IN',
                  height: 44,
                  onPressed: onCheckIn,
                ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Glass(
          radius: 18,
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'How did it go?',
                style: TextStyle(
                  color: AppPalette.textPrimary,
                  fontWeight: FontWeight.w700,
                  fontSize: 14,
                ),
              ),
              const SizedBox(height: 10),
              Row(
                children: [
                  Expanded(
                    child: GradientButton(
                      label: 'IT HAPPENED',
                      height: 42,
                      onPressed: () => onSubmitFeedback(
                        happened: true,
                        feltSafe: true,
                        profileAccurate: true,
                        wouldMeetAgain: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: OutlinedButton(
                      onPressed: () => onSubmitFeedback(happened: false),
                      style: OutlinedButton.styleFrom(
                        minimumSize: const Size.fromHeight(42),
                        side: const BorderSide(color: AppPalette.glassBorder),
                      ),
                      child: const Text(
                        'DIDN\'T HAPPEN',
                        style: TextStyle(
                          color: AppPalette.textSecondary,
                          fontSize: 11,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _ChecklistItem extends StatelessWidget {
  const _ChecklistItem(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          const Icon(
            Icons.check_circle_outline,
            size: 14,
            color: AppPalette.textSecondary,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              text,
              style: const TextStyle(
                color: AppPalette.textSecondary,
                fontSize: 12,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _DoneRow extends StatelessWidget {
  const _DoneRow(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        const Icon(Icons.check_circle, size: 16, color: AppPalette.verified),
        const SizedBox(width: 8),
        Text(
          text,
          style: const TextStyle(
            color: AppPalette.verified,
            fontWeight: FontWeight.w700,
            fontSize: 12,
          ),
        ),
      ],
    );
  }
}

/// The "add a note" step after the happened/didn't-happen choice (ADR-016)
/// — entirely optional, both Save and Skip (and dismissing the sheet
/// itself) proceed to the real feedback submission; only Save with
/// non-empty text carries a note through.
class _FeedbackNoteSheet extends StatelessWidget {
  const _FeedbackNoteSheet({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 20,
        bottom: MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      child: Glass(
        radius: 20,
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const Text(
              'Add a note (optional)',
              style: TextStyle(
                color: AppPalette.textPrimary,
                fontWeight: FontWeight.w700,
                fontSize: 15,
              ),
            ),
            const SizedBox(height: 12),
            Glass(
              radius: 14,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              child: TextField(
                controller: controller,
                maxLines: 4,
                style: const TextStyle(
                  color: AppPalette.textPrimary,
                  fontSize: 14,
                ),
                decoration: const InputDecoration(
                  border: InputBorder.none,
                  hintText: 'Anything worth remembering about this meetup?',
                  hintStyle: TextStyle(color: AppPalette.textSecondary),
                ),
              ),
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(context),
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size.fromHeight(44),
                      side: const BorderSide(color: AppPalette.glassBorder),
                    ),
                    child: const Text(
                      'SKIP',
                      style: TextStyle(color: AppPalette.textSecondary),
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: GradientButton(
                    label: 'SAVE',
                    height: 44,
                    onPressed: () {
                      final text = controller.text.trim();
                      Navigator.pop(context, text.isEmpty ? null : text);
                    },
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
