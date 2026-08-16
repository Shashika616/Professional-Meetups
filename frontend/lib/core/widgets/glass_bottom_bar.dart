import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';

class GlassBottomBar extends StatelessWidget {
  const GlassBottomBar({super.key, required this.index, required this.onTap});

  final int index;
  final ValueChanged<int> onTap;

  static const List<_NavItem> _items = [
    _NavItem(Icons.home_outlined, Icons.home, 'HOME'),
    _NavItem(Icons.people_outline, Icons.people, 'MATCHES'),
    _NavItem(Icons.shield_outlined, Icons.shield, 'SAFETY'),
    _NavItem(Icons.chat_bubble_outline, Icons.chat_bubble, 'CHATS'),
    _NavItem(Icons.person_outline, Icons.person, 'PROFILE'),
  ];

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(14, 0, 14, 14),
        child: Glass(
          radius: 26,
          blur: 18,
          padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 6),
          child: Row(
            children: List.generate(_items.length, (i) {
              final item = _items[i];
              final bool selected = i == index;
              return Expanded(
                child: GestureDetector(
                  behavior: HitTestBehavior.opaque,
                  onTap: () => onTap(i),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        selected ? item.activeIcon : item.icon,
                        size: 21,
                        color: selected
                            ? AppPalette.candyBlue
                            : AppPalette.textSecondary,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        item.label,
                        style: TextStyle(
                          fontSize: 8,
                          letterSpacing: 1.4,
                          fontWeight: FontWeight.w600,
                          color: selected
                              ? AppPalette.candyBlue
                              : AppPalette.textSecondary,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            }),
          ),
        ),
      ),
    );
  }
}

class _NavItem {
  const _NavItem(this.icon, this.activeIcon, this.label);

  final IconData icon;
  final IconData activeIcon;
  final String label;
}
