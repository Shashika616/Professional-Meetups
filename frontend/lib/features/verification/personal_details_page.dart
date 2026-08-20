import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/features/verification/widgets/verification_scaffold.dart';

/// Legal name + address — the one Level 2 step with no OTP, since it's
/// self-reported (Verification Model § 4), not verified against anything.
/// Reachable both from the post-LinkedIn onboarding sequence and
/// independently from `ProfilePage` (`frontend/PLAN.md`'s Level 2/3
/// addendum, Step 6).
class PersonalDetailsPage extends ConsumerStatefulWidget {
  const PersonalDetailsPage({super.key});

  @override
  ConsumerState<PersonalDetailsPage> createState() =>
      _PersonalDetailsPageState();
}

class _PersonalDetailsPageState extends ConsumerState<PersonalDetailsPage> {
  final _legalNameController = TextEditingController();
  final _addressController = TextEditingController();
  bool _submitting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    // GlassTextField doesn't expose onChanged — listening on the
    // controllers directly is what makes CONTINUE react as the user types.
    _legalNameController.addListener(_onFieldChanged);
    _addressController.addListener(_onFieldChanged);
  }

  void _onFieldChanged() => setState(() {});

  @override
  void dispose() {
    _legalNameController.removeListener(_onFieldChanged);
    _addressController.removeListener(_onFieldChanged);
    _legalNameController.dispose();
    _addressController.dispose();
    super.dispose();
  }

  bool get _canSubmit =>
      _legalNameController.text.trim().isNotEmpty &&
      _addressController.text.trim().isNotEmpty;

  Future<void> _submit() async {
    if (_submitting || !_canSubmit) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      final session = await ref
          .read(authServiceProvider)
          .submitPersonalDetails(
            _legalNameController.text.trim(),
            _addressController.text.trim(),
          );
      await ref
          .read(authSessionProvider.notifier)
          .completeVerification(session);
      if (!mounted) return;
      showSnack(context, 'Personal details saved.', type: ToastType.success);
      Navigator.pop(context);
    } on SessionExpiredException {
      // No local error shown — AppShell's listener navigates to LandingPage
      // and shows the "session expired" message itself.
      if (mounted) ref.read(authSessionProvider.notifier).forceSignOut();
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error is AuthException
            ? error.message
            : 'Something went wrong. Please try again.';
      });
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return VerificationScaffold(
      icon: Icons.badge_outlined,
      headline: 'Personal Details',
      trustBenefit:
          'Your legal name and address are never shown to other members '
          '— they help confirm you\'re a real professional and support '
          'incident response if it\'s ever needed.',
      onSkip: () => Navigator.pop(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          GlassTextField(
            controller: _legalNameController,
            icon: Icons.badge_outlined,
            hint: 'Legal name',
          ),
          const SizedBox(height: 12),
          GlassTextField(
            controller: _addressController,
            icon: Icons.home_outlined,
            hint: 'Address',
          ),
          if (_error != null) ...[
            const SizedBox(height: 10),
            Text(
              _error!,
              textAlign: TextAlign.center,
              style: const TextStyle(color: AppPalette.danger, fontSize: 12),
            ),
          ],
          const SizedBox(height: 16),
          GradientButton(
            label: 'CONTINUE',
            isLoading: _submitting,
            onPressed: _canSubmit ? _submit : null,
          ),
        ],
      ),
    );
  }
}
