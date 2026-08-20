import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/meetup.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/meetup_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';

/// Host-only Cancel/Close actions, extracted so this logic exists once
/// instead of duplicated between [MeetupDetailPage] and the "Hosting" tab's
/// request-management screen (ADR-016 addendum, 2026-08-20) — that
/// duplication is exactly what stranded hosts on a screen that couldn't
/// close or cancel anything before this fix. Renders whichever of
/// CANCEL/CLOSE actually apply given [meetup]'s current status/window/
/// accepted count; renders nothing if neither applies.
class HostMeetupControls extends ConsumerWidget {
  const HostMeetupControls({
    super.key,
    required this.meetup,
    required this.onChanged,
  });

  final Meetup meetup;

  /// Called with the updated [Meetup] after a successful Cancel or Close.
  final ValueChanged<Meetup> onChanged;

  bool get _isOpenOrFull =>
      meetup.status == MeetupStatus.open || meetup.status == MeetupStatus.full;

  /// Only once the window has actually started; not forced to wait for
  /// windowEnd, since real meetups run long or short (ADR-016).
  bool get _canClose =>
      meetup.isHostedByMe &&
      _isOpenOrFull &&
      DateTime.now().isAfter(meetup.windowStart);

  /// The backend rejects (409) cancelling a meetup that already has an
  /// accepted participant — no notify-affected-participants path exists yet
  /// (`services/meetup/internal/service/service.go`'s `CancelMeetup`, whose
  /// own comment says as much). Gating this client-side too, the same way
  /// `matches_page.dart`'s intent tabs pre-empt a guaranteed-403 trust-gate
  /// tap, avoids offering a button that's guaranteed to fail rather than
  /// relying on the raw "meetup: cannot cancel a meetup with accepted
  /// participants" server string to explain why.
  bool get _canCancel =>
      meetup.isHostedByMe && _isOpenOrFull && meetup.acceptedCount == 0;

  Future<void> _confirmClose(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: AppPalette.card,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
        title: const Text(
          'CLOSE MEETUP',
          style: TextStyle(
            color: AppPalette.textPrimary,
            letterSpacing: 1.6,
            fontSize: 15,
          ),
        ),
        content: const Text(
          'Mark this meetup as done?',
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
              'CONFIRM',
              style: TextStyle(
                color: AppPalette.candyBlue,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    if (!context.mounted) return;
    await _close(context, ref);
  }

  Future<void> _close(BuildContext context, WidgetRef ref) async {
    try {
      final closed = await ref
          .read(meetupServiceProvider)
          .closeMeetup(meetup.id);
      if (!context.mounted) return;
      onChanged(closed);
      showSnack(context, 'Meetup closed.', type: ToastType.success);
    } catch (error) {
      if (context.mounted) {
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

  Future<void> _confirmCancel(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: AppPalette.card,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
        title: const Text(
          'CANCEL MEETUP',
          style: TextStyle(
            color: AppPalette.danger,
            letterSpacing: 1.6,
            fontSize: 15,
          ),
        ),
        content: const Text(
          'Cancel this meetup? This can\'t be undone.',
          style: TextStyle(color: AppPalette.textSecondary, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text(
              'BACK',
              style: TextStyle(color: AppPalette.textSecondary),
            ),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text(
              'CANCEL MEETUP',
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
    await _cancel(context, ref);
  }

  Future<void> _cancel(BuildContext context, WidgetRef ref) async {
    try {
      await ref.read(meetupServiceProvider).cancelMeetup(meetup.id);
      if (!context.mounted) return;
      // cancelMeetup only returns {success: true} server-side — no updated
      // Meetup to re-render from, unlike closeMeetup. Constructing the
      // post-cancel state locally here is the exception to this app's
      // usual "client never decides, only displays" rule; it's applying a
      // known, deterministic transition after a request the server has
      // already confirmed succeeded, not guessing at server-side state.
      onChanged(
        meetup.copyWith(
          status: MeetupStatus.cancelled,
          cancelledAt: DateTime.now(),
        ),
      );
      showSnack(context, 'Meetup cancelled.', type: ToastType.success);
    } catch (error) {
      if (context.mounted) {
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

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final canClose = _canClose;
    final canCancel = _canCancel;
    if (!canClose && !canCancel) return const SizedBox.shrink();

    final closeButton = OutlinedButton(
      onPressed: () => _confirmClose(context, ref),
      style: OutlinedButton.styleFrom(
        minimumSize: const Size.fromHeight(44),
        side: const BorderSide(color: AppPalette.glassBorder),
      ),
      child: const _ButtonLabel('CLOSE MEETUP', color: AppPalette.textPrimary),
    );

    final cancelButton = OutlinedButton(
      onPressed: () => _confirmCancel(context, ref),
      style: OutlinedButton.styleFrom(
        minimumSize: const Size.fromHeight(44),
        side: const BorderSide(color: AppPalette.danger),
      ),
      child: const _ButtonLabel('CANCEL MEETUP', color: AppPalette.danger),
    );

    // Top padding lives here, not as a sibling SizedBox at each call site,
    // so callers don't end up with a stray gap when neither action applies
    // and this widget renders nothing.
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: canClose && canCancel
          // Same narrow-device/long-label RenderFlex overflow class the
          // GradientButton fix addressed — `_ButtonLabel` wraps its Text in
          // a Flexible/ellipsis for the same reason, since two side-by-side
          // buttons is exactly the layout that originally surfaced that bug.
          ? Row(
              children: [
                Expanded(child: cancelButton),
                const SizedBox(width: 10),
                Expanded(child: closeButton),
              ],
            )
          : (canClose ? closeButton : cancelButton),
    );
  }
}

class _ButtonLabel extends StatelessWidget {
  const _ButtonLabel(this.text, {required this.color});

  final String text;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(
            text,
            overflow: TextOverflow.ellipsis,
            maxLines: 1,
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.w700,
              fontSize: 13,
              letterSpacing: 0.6,
            ),
          ),
        ),
      ],
    );
  }
}
