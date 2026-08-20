import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/app_shell.dart';
import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/utils/snacks.dart';
import 'package:professional_connections_platform/core/utils/toast.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';

/// Email + password sign-in for a returning user (ADR-014) — the one path
/// that genuinely can't collapse "resolve or create" into a single tap the
/// way Apple/Google/LinkedIn do, so it gets its own form, reached from
/// [LandingPage]'s "Sign in" link — distinct from [OnboardingFlow]'s entry
/// screen, which is for new sign-ups.
class EmailLoginPage extends ConsumerStatefulWidget {
  const EmailLoginPage({super.key});

  @override
  ConsumerState<EmailLoginPage> createState() => _EmailLoginPageState();
}

class _EmailLoginPageState extends ConsumerState<EmailLoginPage> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _emailController.addListener(_onFieldChanged);
    _passwordController.addListener(_onFieldChanged);
  }

  void _onFieldChanged() => setState(() {});

  @override
  void dispose() {
    _emailController.removeListener(_onFieldChanged);
    _passwordController.removeListener(_onFieldChanged);
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  bool get _canSubmit =>
      _emailController.text.trim().contains('@') &&
      _passwordController.text.isNotEmpty;

  Future<void> _submit() async {
    if (_busy || !_canSubmit) return;
    setState(() => _busy = true);
    try {
      await ref
          .read(authSessionProvider.notifier)
          .loginWithEmail(
            email: _emailController.text.trim(),
            password: _passwordController.text,
          );
      if (!mounted) return;
      // Same pushAndRemoveUntil reasoning as OnboardingFlow's own
      // post-sign-in navigation — this page was pushed on top of
      // LandingPage, so a plain pushReplacement would leave LandingPage
      // stranded under AppShell in the stack.
      Navigator.of(context).pushAndRemoveUntil(
        MaterialPageRoute(builder: (context) => const AppShell()),
        (route) => false,
      );
    } catch (error) {
      if (mounted) {
        showSnack(
          context,
          error is AuthException
              ? error.message
              : 'Something went wrong. Please try again.',
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
      appBar: AppBar(title: const Text('SIGN IN')),
      body: AppBackground(
        imageOpacity: 0.35,
        child: SafeArea(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const SizedBox(height: 8),
                const Text(
                  'Welcome back',
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w800,
                    color: AppPalette.textPrimary,
                  ),
                ),
                const SizedBox(height: 8),
                const Text(
                  'Sign in with the email and password you set during '
                  'signup. Used LinkedIn, Apple, or Google instead? Use '
                  'that same button on the sign-up screen it signs you '
                  'in too.',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppPalette.textSecondary,
                    height: 1.4,
                  ),
                ),
                const SizedBox(height: 24),
                GlassTextField(
                  controller: _emailController,
                  icon: Icons.alternate_email_rounded,
                  hint: 'you@example.com',
                  keyboardType: TextInputType.emailAddress,
                ),
                const SizedBox(height: 12),
                GlassTextField(
                  controller: _passwordController,
                  icon: Icons.lock_outline_rounded,
                  hint: 'Password',
                  obscureText: true,
                  textInputAction: TextInputAction.done,
                  onFieldSubmitted: (_) => _submit(),
                ),
                const SizedBox(height: 20),
                GradientButton(
                  label: 'SIGN IN',
                  isLoading: _busy,
                  onPressed: _canSubmit ? _submit : null,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
