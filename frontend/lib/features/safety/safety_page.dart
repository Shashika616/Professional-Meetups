import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';

class SafetyPage extends StatelessWidget {
  const SafetyPage({super.key});

  @override
  Widget build(BuildContext context) {
    final checklist = [
      'Meet in a public place',
      'Tell a trusted contact where you are going',
      'Keep first meetings short',
      'Never share OTP codes',
      'Never send money',
    ];

    return Scaffold(
      backgroundColor: Colors.transparent,
      appBar: AppBar(title: const Text('SAFETY CENTER')),
      body: Padding(
        padding: const EdgeInsets.fromLTRB(20, 4, 20, 100),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const SectionLabel('PRE MEETUP CHECKLIST'),
            const SizedBox(height: 12),
            Expanded(
              child: ListView.builder(
                itemCount: checklist.length,
                itemBuilder: (context, index) {
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 10),
                    child: Glass(
                      radius: 16,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 12,
                      ),
                      child: Row(
                        children: [
                          const Icon(
                            Icons.check_circle_outline,
                            size: 18,
                            color: AppPalette.verified,
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              checklist[index],
                              style: const TextStyle(
                                color: AppPalette.textPrimary,
                                fontSize: 13,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  );
                },
              ),
            ),
            const SizedBox(height: 12),
            GestureDetector(
              onTap: () => showDialog(
                context: context,
                builder: (context) => AlertDialog(
                  backgroundColor: AppPalette.card,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(20),
                  ),
                  title: const Text(
                    'EMERGENCY SOS',
                    style: TextStyle(
                      color: AppPalette.danger,
                      letterSpacing: 1.6,
                      fontSize: 15,
                    ),
                  ),
                  content: const Text(
                    'This will share your live location with trusted contacts and alert the safety team.',
                    style: TextStyle(
                      color: AppPalette.textSecondary,
                      fontSize: 13,
                    ),
                  ),
                  actions: [
                    TextButton(
                      onPressed: () => Navigator.pop(context),
                      child: const Text(
                        'CANCEL',
                        style: TextStyle(color: AppPalette.textSecondary),
                      ),
                    ),
                    TextButton(
                      onPressed: () {
                        Navigator.pop(context);
                        showSnack(
                          context,
                          'SOS activated. Trusted contacts notified.',
                          type: ToastType.error,
                        );
                      },
                      child: const Text(
                        'CONFIRM',
                        style: TextStyle(
                          color: AppPalette.danger,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              child: Glass(
                radius: 26,
                tint: AppPalette.danger.withValues(alpha: 0.12),
                border: AppPalette.danger.withValues(alpha: 0.5),
                padding: const EdgeInsets.symmetric(vertical: 16),
                child: const Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      Icons.emergency_share,
                      color: AppPalette.danger,
                      size: 20,
                    ),
                    SizedBox(width: 10),
                    Text(
                      'TRIGGER SOS',
                      style: TextStyle(
                        color: AppPalette.danger,
                        fontWeight: FontWeight.w800,
                        letterSpacing: 2.4,
                        fontSize: 13,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
