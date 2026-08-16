import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/features/home/widgets/intent_tile.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';

class IntentPickerSheet extends ConsumerWidget {
  const IntentPickerSheet({super.key, required this.trustLevel});

  final int trustLevel;

  static Future<void> show(BuildContext context, int trustLevel) {
    return showModalBottomSheet<void>(
      context: context,
      backgroundColor: Colors.transparent,
      barrierColor: AppPalette.onyx.withValues(alpha: 0.65),
      isScrollControlled: true,
      transitionAnimationController: AnimationController(
        vsync: Navigator.of(context),
        duration: const Duration(milliseconds: 300),
      ),
      builder: (context) => IntentPickerSheet(trustLevel: trustLevel),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selected = ref.watch(selectedIntentProvider);

    return ClipRRect(
      borderRadius: const BorderRadius.vertical(top: Radius.circular(28)),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 20, sigmaY: 20),
        child: Container(
          color: AppPalette.surface.withValues(alpha: 0.88),
          padding: EdgeInsets.fromLTRB(
            20,
            12,
            20,
            MediaQuery.of(context).padding.bottom + 24,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Container(
                  width: 44,
                  height: 4,
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              const SectionLabel('ALL INTENTS'),
              const SizedBox(height: 16),
              GridView.count(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                crossAxisCount: 2,
                crossAxisSpacing: 12,
                mainAxisSpacing: 12,
                childAspectRatio: 2.1,
                children: [
                  for (final intent in IntentType.values)
                    IntentTile(
                      intent: intent,
                      selected: intent == selected,
                      locked: !intent.isUnlockedFor(trustLevel),
                      onTap: () {
                        if (!intent.isUnlockedFor(trustLevel)) {
                          showSnack(
                            context,
                            '${intent.label} requires Level ${intent.requiredTrustLevel} trust.',
                            type: ToastType.locked,
                          );
                          return;
                        }
                        ref.read(selectedIntentProvider.notifier).state =
                            intent;
                        Navigator.of(context).pop();
                      },
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
