import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/app_shell.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/validation/validators.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';
import 'package:professional_connections_platform/core/widgets/section_label.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';

class OnboardingFlow extends ConsumerStatefulWidget {
  const OnboardingFlow({super.key});

  @override
  ConsumerState<OnboardingFlow> createState() => _OnboardingFlowState();
}

class _OnboardingFlowState extends ConsumerState<OnboardingFlow> {
  final _formKey = GlobalKey<FormState>();
  int _step = 0;
  bool _busy = false;

  final _phoneController = TextEditingController();
  final _otpController = TextEditingController();
  final _linkedinController = TextEditingController();
  final _emailController = TextEditingController();

  @override
  void dispose() {
    _phoneController.dispose();
    _otpController.dispose();
    _linkedinController.dispose();
    _emailController.dispose();
    super.dispose();
  }

  Future<void> _continue() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;

    final auth = ref.read(authServiceProvider);
    setState(() => _busy = true);

    try {
      switch (_step) {
        case 1:
          await auth.verifyPhoneOtp(
            phoneNumber: _phoneController.text.trim(),
            code: _otpController.text.trim(),
          );
          if (mounted) showSnack(context, 'Phone verified successfully.', type: ToastType.success);
        case 2:
          await auth.connectLinkedIn(_linkedinController.text.trim());
          if (mounted) showSnack(context, 'LinkedIn connected.', type: ToastType.success);
        case 3:
          await auth.verifyCorporateEmail(_emailController.text.trim());
          if (mounted) showSnack(context, 'Work email verified. Welcome aboard.', type: ToastType.success);
      }

      if (!mounted) return;

      if (_step == 3) {
        Navigator.pushReplacement(
          context,
          MaterialPageRoute(builder: (context) => const AppShell()),
        );
      } else {
        setState(() => _step += 1);
      }
    } catch (error) {
      if (mounted) {
        showSnack(
           context,
          error is FormatException ? error.message : 'Something went wrong. Please try again.',
          type: ToastType.error,
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: AppBackground(
        imageOpacity: 0.35,
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
            child: Form(
              key: _formKey,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildProgress(),
                  const SizedBox(height: 40),
                  Expanded(child: _buildCurrentStep()),
                  const SizedBox(height: 24),
                  GradientButton(
                    label: _step == 3 ? 'ENTER PLATFORM' : 'CONTINUE',
                    isLoading: _busy,
                    onPressed: _continue,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildProgress() {
    return Row(
      children: List.generate(4, (index) {
        return Expanded(
          child: Container(
            height: 4,
            margin: const EdgeInsets.only(right: 8),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(2),
              color: index <= _step ? AppPalette.candyBlue : AppPalette.card,
            ),
          ),
        );
      }),
    );
  }

  Widget _buildCurrentStep() {
    switch (_step) {
      case 0:
        return _welcomeStep();
      case 1:
        return _phoneStep();
      case 2:
        return _linkedinStep();
      default:
        return _emailStep();
    }
  }

  Widget _welcomeStep() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              border: Border.all(color: AppPalette.candyBlue.withValues(alpha: 0.3), width: 2),
            ),
            child: const Icon(Icons.handshake_outlined, size: 64, color: AppPalette.candyBlue),
          ),
          const SizedBox(height: 32),
          const Text(
            'Connect Beyond\nThe Office.',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 32,
              fontWeight: FontWeight.w800,
              color: AppPalette.textPrimary,
              height: 1.2,
              letterSpacing: -0.5,
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Meet verified professionals in real life.\nYour next coffee, mentor, or co-founder is nearby.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 14, color: AppPalette.textSecondary, height: 1.5),
          ),
        ],
      ),
    );
  }

  Widget _phoneStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionLabel('STEP 1 OF 3'),
        const SizedBox(height: 12),
        const Text('Verify your phone', style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: AppPalette.textPrimary)),
        const SizedBox(height: 8),
        const Text('We use this to ensure our community remains safe and free of bots.', style: TextStyle(fontSize: 14, color: AppPalette.textSecondary)),
        const SizedBox(height: 32),
        GlassTextField(
          controller: _phoneController,
          icon: Icons.phone_android,
          hint: '+94 7X XXX XXXX',
          keyboardType: TextInputType.phone,
          maxLength: 16,
          validator: (value) => Validators.phone(value ?? ''),
        ),
        const SizedBox(height: 16),
        GlassTextField(
          controller: _otpController,
          icon: Icons.sms_outlined,
          hint: '• • • • • •',
          keyboardType: TextInputType.number,
          maxLength: 6,
          textAlign: TextAlign.center,
          letterSpacing: 8,
          validator: (value) => Validators.otp(value ?? ''),
        ),
      ],
    );
  }

  Widget _linkedinStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionLabel('STEP 2 OF 3'),
        const SizedBox(height: 12),
        const Text('Connect LinkedIn', style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: AppPalette.textPrimary)),
        const SizedBox(height: 8),
        const Text('This proves your professional identity and helps us match you with relevant peers.', style: TextStyle(fontSize: 14, color: AppPalette.textSecondary)),
        const SizedBox(height: 32),
        GlassTextField(
          controller: _linkedinController,
          icon: Icons.work_outline,
          hint: 'linkedin.com/in/your-profile',
          keyboardType: TextInputType.url,
          maxLength: 100,
          validator: (value) => Validators.linkedin(value ?? ''),
        ),
      ],
    );
  }

  Widget _emailStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionLabel('STEP 3 OF 3'),
        const SizedBox(height: 12),
        const Text('Professional Email', style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: AppPalette.textPrimary)),
        const SizedBox(height: 8),
        const Text('Enter your corporate email to unlock Level 2 Trust. (No Gmail or Yahoo).', style: TextStyle(fontSize: 14, color: AppPalette.textSecondary)),
        const SizedBox(height: 32),
        GlassTextField(
          controller: _emailController,
          icon: Icons.email_outlined,
          hint: 'name@company.com',
          keyboardType: TextInputType.emailAddress,
          maxLength: 254,
          validator: (value) => Validators.corporateEmail(value ?? ''),
        ),
      ],
    );
  }
}