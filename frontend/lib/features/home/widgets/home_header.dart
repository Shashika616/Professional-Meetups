import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/app_icon.dart';
import 'package:professional_connections_platform/core/widgets/professional_avatar.dart';

class HomeHeader extends StatelessWidget {
  const HomeHeader({super.key, required this.userName, this.imageUrl});

  final String userName;
  final String? imageUrl;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Brand Logo placed cleanly above the greeting
                const AppIcon(size: 32),
                const SizedBox(height: 12),
                Text(
                  _greeting(),
                  style: TextStyle(
                    color: AppPalette.textSecondary.withValues(alpha: 0.8),
                    fontSize: 13,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  userName,
                  style: const TextStyle(
                    color: AppPalette.textPrimary,
                    fontSize: 22,
                    fontWeight: FontWeight.w700,
                    letterSpacing: -0.5,
                  ),
                ),
              ],
            ),
          ),
          _NotificationBell(
            onTap: () => showSnack(
              context,
              'Notifications will arrive once the backend is live.',
              type: ToastType.info,
            ),
          ),
          const SizedBox(width: 12),
          ProfessionalAvatar(size: 44, name: userName, imageUrl: imageUrl),
        ],
      ),
    );
  }

  String _greeting() {
    final hour = DateTime.now().hour;
    if (hour < 12) return 'Good morning,';
    if (hour < 17) return 'Good afternoon,';
    return 'Good evening,';
  }
}

class _NotificationBell extends StatelessWidget {
  const _NotificationBell({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.all(8.0),
        child: const Icon(
          Icons.notifications_none_rounded,
          size: 24,
          color: AppPalette.textPrimary,
        ),
      ),
    );
  }
}
