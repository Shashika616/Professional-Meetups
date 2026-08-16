import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';

class UpcomingMeetupCard extends StatelessWidget {
  const UpcomingMeetupCard({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionLabel('YOUR NEXT MEETUP'),
          const SizedBox(height: 16),
          Glass(
            radius: 20,
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                Row(
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
                        children: const [
                          Text(
                            'Coffee with Sachini Fernando',
                            style: TextStyle(
                              color: AppPalette.textPrimary,
                              fontSize: 14,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          SizedBox(height: 4),
                          Text(
                            'Today, 12:30 PM • Colombo Fort',
                            style: TextStyle(
                              color: AppPalette.textSecondary,
                              fontSize: 12,
                            ),
                          ),
                        ],
                      ),
                    ),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 10,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: AppPalette.verified.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(10),
                        border: Border.all(
                          color: AppPalette.verified.withValues(alpha: 0.35),
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
                  ],
                ),
                const SizedBox(height: 14),
                Row(
                  children: [
                    Expanded(
                      child: GestureDetector(
                        onTap: () => showSnack(
                          context,
                          'Check-in opens 10 minutes before the meetup.',
                        ),
                        child: Container(
                          height: 42,
                          decoration: BoxDecoration(
                            borderRadius: BorderRadius.circular(21),
                            border: Border.all(
                              color: AppPalette.candyBlue.withValues(
                                alpha: 0.5,
                              ),
                            ),
                          ),
                          child: const Center(
                            child: Text(
                              'CHECK IN',
                              style: TextStyle(
                                color: AppPalette.candyBlue,
                                fontSize: 11,
                                fontWeight: FontWeight.w800,
                                letterSpacing: 1.6,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(width: 10),
                    GestureDetector(
                      onTap: () => showSnack(
                        context,
                        'Safety checklist and live share will open here.',
                      ),
                      child: Container(
                        height: 42,
                        width: 42,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(color: AppPalette.glassBorder),
                          color: Colors.white.withValues(alpha: 0.05),
                        ),
                        child: const Icon(
                          Icons.shield_outlined,
                          size: 18,
                          color: AppPalette.textSecondary,
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
    );
  }
}
