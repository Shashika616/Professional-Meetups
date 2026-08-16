import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';

class AvailabilitySearchBar extends StatelessWidget {
  const AvailabilitySearchBar({
    super.key,
    required this.controller,
    this.onFilterTap,
  });

  final TextEditingController controller;
  final VoidCallback? onFilterTap;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 24),
      child: Glass(
        radius: 18,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        child: Row(
          children: [
            const Icon(Icons.search_rounded, color: AppPalette.candyBlue, size: 22),
            const SizedBox(width: 12),
            Expanded(
              child: TextField(
                controller: controller,
                style: const TextStyle(color: AppPalette.textPrimary, fontSize: 14, fontWeight: FontWeight.w500),
                decoration: const InputDecoration(
                  border: InputBorder.none,
                  hintText: 'Set availability or search location...',
                  hintStyle: TextStyle(color: AppPalette.textSecondary, fontSize: 14, fontWeight: FontWeight.w400),
                ),
              ),
            ),
            Container(width: 1, height: 24, color: AppPalette.glassBorder),
            const SizedBox(width: 4),
            IconButton(
              onPressed: onFilterTap,
              icon: const Icon(Icons.tune_rounded, color: AppPalette.textSecondary, size: 20),
              splashRadius: 18,
            ),
          ],
        ),
      ),
    );
  }
}