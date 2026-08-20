import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';

/// Compact "Professional"/"Official" chips shown wherever another user's
/// trust level is displayed (request cards, browse cards) — a friendlier
/// alternative to the raw "L{n}" [TrustLevelBadge] pill for surfaces where
/// what matters to the viewer is "did this person actually verify who they
/// are," not the exact numeric level.
///
/// Deliberately derived from [trustLevel] alone, not any new backend field:
/// `computeTrustLevel` (`backend/services/auth/internal/service/trustlevel.go`)
/// is a strict ladder where Level 1+ requires LinkedIn connected and Level 3
/// requires corporate work-email verification on top of Level 2 — so the
/// trust level integer already sent on every meetup/request response
/// unambiguously implies both badges below. This avoids exposing any new
/// field (e.g. a raw `linkedin_verified`/`work_email_verified` boolean) to
/// other users, keeping to the "name and trust level only" scope
/// (ADR-013, `docs/02-domain/domain-model.md` § Meetup Request).
class VerificationBadges extends StatelessWidget {
  const VerificationBadges({super.key, required this.trustLevel});

  final int trustLevel;

  @override
  Widget build(BuildContext context) {
    final badges = <Widget>[
      if (trustLevel >= 1)
        const _Badge(
          icon: Icons.work_outline_rounded,
          label: 'Professional',
          color: AppPalette.verified,
        ),
      if (trustLevel >= 3)
        const _Badge(
          icon: Icons.verified_rounded,
          label: 'Official',
          color: AppPalette.gold,
        ),
    ];
    if (badges.isEmpty) return const SizedBox.shrink();
    return Wrap(spacing: 6, runSpacing: 6, children: badges);
  }
}

class _Badge extends StatelessWidget {
  const _Badge({required this.icon, required this.label, required this.color});

  final IconData icon;
  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        color: color.withValues(alpha: 0.10),
        border: Border.all(color: color.withValues(alpha: 0.30)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 11, color: color),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.w800,
              letterSpacing: 0.3,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}
