import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/core/widgets/professional_avatar.dart';

class ProfilePage extends StatelessWidget {
  const ProfilePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SafeArea(
        child: ListView(
          padding: EdgeInsets.zero,
          children: [
            const SizedBox(height: 28),
            _avatarBlock(),
            const SizedBox(height: 22),
            _statsRow(),
            const SizedBox(height: 28),
            _section('VERIFICATION', children: [
              _Row(
                icon: Icons.phone_android,
                title: 'Phone',
                subtitle: 'Not verified',
                trailing: _verifyChip(context, 'Phone'),
              ),
              const _Divider(),
              _Row(icon: Icons.work_outline, title: 'LinkedIn', subtitle: 'Not connected', trailing: _verifyChip(context, 'LinkedIn')),
              const _Divider(),
              _Row(icon: Icons.email_outlined, title: 'Work Email', subtitle: 'Not verified', trailing: _verifyChip(context, 'Work email')),
            ]),
            const SizedBox(height: 24),
            _section('PREFERENCES', children: const [
              _Row(
                icon: Icons.lock_outline,
                title: 'Privacy Controls',
                subtitle: 'Visibility and location',
                trailing: Icon(Icons.chevron_right_rounded, size: 18, color: AppPalette.textSecondary),
              ),
              _Divider(),
              _Row(
                icon: Icons.notifications_none_rounded,
                title: 'Notifications',
                subtitle: 'Push and email alerts',
                trailing: Icon(Icons.chevron_right_rounded, size: 18, color: AppPalette.textSecondary),
              ),
              _Divider(),
              _Row(
                icon: Icons.shield_outlined,
                title: 'Safety Center',
                subtitle: 'Trusted contacts and SOS',
                trailing: Icon(Icons.chevron_right_rounded, size: 18, color: AppPalette.textSecondary),
              ),
            ]),
            const SizedBox(height: 24),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: GestureDetector(
                onTap: () => showSnack(context, 'Sign out will be wired to the auth service.'),
                child: Glass(
                  radius: 16,
                  tint: AppPalette.danger.withValues(alpha: 0.08),
                  border: AppPalette.danger.withValues(alpha: 0.3),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  child: const Center(
                    child: Text(
                      'SIGN OUT',
                      style: TextStyle(fontSize: 11, letterSpacing: 1.8, fontWeight: FontWeight.w800, color: AppPalette.danger),
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 96),
          ],
        ),
      ),
    );
  }

    Widget _avatarBlock() {
    return Center(
      child: Column(
        children: [
          // The user's own avatar (initials now, real photo later)
          const ProfessionalAvatar(size: 92, name: 'Shashika Fernando'),
          const SizedBox(height: 14),
          const Text(
            'Shashika Fernando',
            style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: AppPalette.textPrimary, letterSpacing: -0.3),
          ),
          const SizedBox(height: 4),
          Text(
            'SWE • Colombo',
            style: TextStyle(fontSize: 12, color: AppPalette.textSecondary.withValues(alpha: 0.9)),
          ),
        ],
      ),
    );
  }

  Widget _statsRow() {
    return const Padding(
      padding: EdgeInsets.symmetric(horizontal: 20),
      child: Row(
        children: [
          Expanded(child: _StatChip(value: '12', label: 'MEETUPS')),
          SizedBox(width: 12),
          Expanded(child: _StatChip(value: '4.9', label: 'RATING')),
          SizedBox(width: 12),
          Expanded(child: _StatChip(value: 'L1', label: 'TRUST')),
        ],
      ),
    );
  }

  Widget _section(String title, {required List<Widget> children}) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SectionLabel(title),
          const SizedBox(height: 12),
          Glass(
            radius: 20,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
            child: Column(children: children),
          ),
        ],
      ),
    );
  }

  Widget _verifyChip(BuildContext context, String name) {
    return GestureDetector(
      onTap: () => showSnack(context, '$name verification flow will be built next.'),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: AppPalette.candyBlue.withValues(alpha: 0.5)),
        ),
        child: const Text(
          'VERIFY',
          style: TextStyle(fontSize: 8, letterSpacing: 1.4, fontWeight: FontWeight.w800, color: AppPalette.candyBlue),
        ),
      ),
    );
  }
}

class _StatChip extends StatelessWidget {
  const _StatChip({required this.value, required this.label});

  final String value;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 16,
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Column(
        children: [
          Text(value, style: const TextStyle(color: AppPalette.candyBlue, fontWeight: FontWeight.w800, fontSize: 16)),
          const SizedBox(height: 2),
          Text(
            label,
            style: const TextStyle(fontSize: 7, letterSpacing: 1.4, color: AppPalette.textSecondary, fontWeight: FontWeight.w700),
          ),
        ],
      ),
    );
  }
}

class _Row extends StatelessWidget {
  const _Row({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.trailing,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final Widget trailing;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: Colors.white.withValues(alpha: 0.05),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(icon, size: 18, color: AppPalette.candyBlue),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: const TextStyle(color: AppPalette.textPrimary, fontSize: 13, fontWeight: FontWeight.w600)),
                const SizedBox(height: 2),
                Text(subtitle, style: const TextStyle(color: AppPalette.textSecondary, fontSize: 11)),
              ],
            ),
          ),
          trailing,
        ],
      ),
    );
  }
}

class _Divider extends StatelessWidget {
  const _Divider();

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 1,
      color: AppPalette.glassBorder,
      margin: const EdgeInsets.symmetric(vertical: 2),
    );
  }
}