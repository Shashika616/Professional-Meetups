import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';

/// The user's profile picture.
/// Shows initials on the brand gradient until a real photo URL arrives
/// from the backend. The brand mark (watch dial) is a separate widget: [AppIcon].
class ProfessionalAvatar extends StatelessWidget {
  const ProfessionalAvatar({
    super.key,
    this.name,
    this.imageUrl,
    this.size = 44,
  });

  final String? name;
  final String? imageUrl;
  final double size;

  String get _initials {
    final trimmed = name?.trim() ?? '';
    if (trimmed.isEmpty) return 'PC';
    final parts = trimmed.split(RegExp(r'\s+'));
    final first = parts.first.isNotEmpty ? parts.first[0].toUpperCase() : '';
    final last = parts.length > 1 ? parts.last[0].toUpperCase() : '';
    return '$first$last';
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: const LinearGradient(
          colors: [AppPalette.deepBlue, AppPalette.steelBlue],
        ),
        border: Border.all(
          color: AppPalette.candyBlue.withValues(alpha: 0.4),
          width: 1.5,
        ),
      ),
      child: ClipOval(
        child: imageUrl != null
            ? Image.network(
                imageUrl!,
                fit: BoxFit.cover,
                errorBuilder: (context, error, stackTrace) => _initialsWidget(),
                loadingBuilder: (context, child, progress) =>
                    progress == null ? child : _initialsWidget(),
              )
            : _initialsWidget(),
      ),
    );
  }

  Widget _initialsWidget() {
    return Center(
      child: Text(
        _initials,
        style: TextStyle(
          color: AppPalette.candyBlue,
          fontWeight: FontWeight.w800,
          fontSize: size * 0.36,
          letterSpacing: 0.5,
        ),
      ),
    );
  }
}
