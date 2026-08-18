import 'dart:async';

import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass_text_field.dart';
import 'package:professional_connections_platform/core/widgets/gradient_button.dart';

/// Shared 6-digit code entry — used identically by all three OTP-based
/// verification screens (phone/personal-email/corporate-email) so the
/// input-plus-timer UI isn't built four times (`frontend/PLAN.md`'s Level
/// 2/3 addendum, Step 4).
///
/// The countdown is seeded from [initialResendAfterSeconds] — the server's
/// actual value from the `Start*Verification` call that preceded this
/// widget, never a hardcoded client-side guess (self-review checklist).
/// [onResend] returns the server's fresh `resend_after_seconds` for the
/// *next* cooldown, which reseeds the timer — if the server ever changes
/// that value, this widget picks it up rather than assuming it's always 60.
class OtpEntry extends StatefulWidget {
  const OtpEntry({
    super.key,
    required this.initialResendAfterSeconds,
    required this.onSubmit,
    required this.onResend,
  });

  final int initialResendAfterSeconds;
  final Future<void> Function(String code) onSubmit;
  final Future<int> Function() onResend;

  @override
  State<OtpEntry> createState() => _OtpEntryState();
}

class _OtpEntryState extends State<OtpEntry> {
  final _codeController = TextEditingController();
  Timer? _timer;
  late int _secondsRemaining = widget.initialResendAfterSeconds;
  bool _submitting = false;
  bool _resending = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _startTimer();
    // GlassTextField doesn't expose onChanged — listening on the
    // controller directly is what makes the VERIFY button react as digits
    // are typed (enabled only once all 6 are entered).
    _codeController.addListener(_onCodeChanged);
  }

  void _onCodeChanged() => setState(() {});

  @override
  void dispose() {
    _timer?.cancel();
    _codeController.removeListener(_onCodeChanged);
    _codeController.dispose();
    super.dispose();
  }

  void _startTimer() {
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() {
        if (_secondsRemaining > 0) _secondsRemaining--;
      });
      if (_secondsRemaining <= 0) _timer?.cancel();
    });
  }

  Future<void> _submit() async {
    if (_submitting || _codeController.text.length != 6) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await widget.onSubmit(_codeController.text);
      // On success the caller navigates away — nothing left to update here.
    } catch (error) {
      if (!mounted) return;
      setState(() {
        // A wrong/expired code shows an error but deliberately does NOT
        // reset the countdown — a wrong guess shouldn't give a free timer
        // reset (frontend/PLAN.md's addendum, Step 4).
        _error = error is AuthException
            ? error.message
            : 'Something went wrong. Please try again.';
      });
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _resend() async {
    if (_resending || _secondsRemaining > 0) return;
    setState(() => _resending = true);
    try {
      final resendAfterSeconds = await widget.onResend();
      if (!mounted) return;
      setState(() {
        _secondsRemaining = resendAfterSeconds;
        _error = null;
        _codeController.clear();
      });
      _startTimer();
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error is AuthException
            ? error.message
            : 'Something went wrong. Please try again.';
      });
    } finally {
      if (mounted) setState(() => _resending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final canResend = _secondsRemaining <= 0 && !_resending;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        GlassTextField(
          controller: _codeController,
          icon: Icons.pin_outlined,
          hint: '——————',
          keyboardType: TextInputType.number,
          maxLength: 6,
          textAlign: TextAlign.center,
          letterSpacing: 8,
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
          label: 'VERIFY',
          isLoading: _submitting,
          onPressed: _codeController.text.length == 6 ? _submit : null,
        ),
        const SizedBox(height: 14),
        Center(
          child: GestureDetector(
            onTap: canResend ? _resend : null,
            child: Text(
              canResend
                  ? 'Resend code'
                  : 'Resend code in ${_secondsRemaining}s',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: canResend
                    ? AppPalette.candyBlue
                    : AppPalette.textSecondary.withValues(alpha: 0.6),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
