import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';

class GradientButton extends StatefulWidget {
  const GradientButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.height = 56,
    this.isLoading = false,
    this.icon,
  });

  final String label;
  final VoidCallback? onPressed;
  final double height;
  final bool isLoading;
  final IconData? icon;

  @override
  State<GradientButton> createState() => _GradientButtonState();
}

class _GradientButtonState extends State<GradientButton> {
  bool _isPressed = false;

  void _onTapDown(TapDownDetails details) {
    if (widget.onPressed != null && !widget.isLoading) {
      setState(() => _isPressed = true);
    }
  }

  void _onTapUp(TapUpDetails details) {
    if (mounted) setState(() => _isPressed = false);
  }

  void _onTapCancel() {
    if (mounted) setState(() => _isPressed = false);
  }

  @override
  Widget build(BuildContext context) {
    final bool isEnabled = widget.onPressed != null && !widget.isLoading;
    final double borderRadius = widget.height / 2;

    return GestureDetector(
      onTapDown: _onTapDown,
      onTapUp: _onTapUp,
      onTapCancel: _onTapCancel,
      onTap: isEnabled ? widget.onPressed : null,
      child: AnimatedScale(
        scale: _isPressed ? 1.03 : 1.0, // Gets a little bigger when pressed
        duration: const Duration(milliseconds: 120),
        curve: Curves.easeOut,
        child: Material(
          color: Colors.transparent,
          borderRadius: BorderRadius.circular(borderRadius),
          child: Ink(
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(borderRadius),
              // Solid gradient that fills the entire block
              gradient: isEnabled
                  ? const LinearGradient(
                      colors: [AppPalette.candyBlue, AppPalette.steelBlue],
                    )
                  : LinearGradient(
                      colors: [
                        AppPalette.candyBlue.withValues(alpha: 0.4),
                        AppPalette.steelBlue.withValues(alpha: 0.4),
                      ],
                    ),
              boxShadow: [
                BoxShadow(
                  // Shadow expands and drops further when pressed to simulate lifting
                  color: AppPalette.candyBlue.withValues(alpha: _isPressed ? 0.55 : 0.25),
                  blurRadius: _isPressed ? 36 : 20,
                  offset: Offset(0, _isPressed ? 16 : 8),
                ),
              ],
            ),
            child: InkWell(
              borderRadius: BorderRadius.circular(borderRadius),
              onTap: isEnabled ? widget.onPressed : null,
              child: Container(
                width: double.infinity, // Forces the gradient to fill the whole width
                height: widget.height,
                alignment: Alignment.center,
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: widget.isLoading
                    ? const SizedBox(
                        width: 24,
                        height: 24,
                        child: CircularProgressIndicator(
                          strokeWidth: 2.5,
                          color: AppPalette.onyx,
                        ),
                      )
                    : Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (widget.icon != null) ...[
                            Icon(widget.icon, size: 20, color: AppPalette.onyx),
                            const SizedBox(width: 12),
                          ],
                          Text(
                            widget.label,
                            style: const TextStyle(
                              color: AppPalette.onyx,
                              fontWeight: FontWeight.w800,
                              letterSpacing: 1.5,
                              fontSize: 15,
                            ),
                          ),
                        ],
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}