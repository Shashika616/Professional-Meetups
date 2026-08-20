import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/glass.dart';

class GlassTextField extends StatelessWidget {
  const GlassTextField({
    super.key,
    required this.controller,
    required this.icon,
    required this.hint,
    this.keyboardType,
    this.validator,
    this.maxLength,
    this.textAlign = TextAlign.left,
    this.letterSpacing,
    this.textInputAction,
    this.onFieldSubmitted,
    this.obscureText = false,
  });

  final TextEditingController controller;
  final IconData icon;
  final String hint;
  final TextInputType? keyboardType;
  final String? Function(String?)? validator;
  final int? maxLength;
  final TextAlign textAlign;
  final double? letterSpacing;
  final TextInputAction? textInputAction;
  // A location search field's "type a full query and press search/return"
  // path (frontend/meetup-scheduling-PLAN.md's 2026-08-18 platform-split
  // addendum, Step 2) — optional, every other caller of this field leaves
  // it null and gets the previous behavior unchanged.
  final void Function(String)? onFieldSubmitted;
  // Email+password signup/login (ADR-014 decision #2) — optional, defaults
  // to false so every existing caller is unaffected.
  final bool obscureText;

  @override
  Widget build(BuildContext context) {
    return Glass(
      radius: 16,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: TextFormField(
        controller: controller,
        keyboardType: keyboardType,
        textAlign: textAlign,
        autocorrect: false,
        maxLength: maxLength,
        textInputAction: textInputAction,
        onFieldSubmitted: onFieldSubmitted,
        obscureText: obscureText,
        style: TextStyle(
          color: AppPalette.textPrimary,
          fontSize: 16,
          letterSpacing: letterSpacing,
        ),
        decoration: InputDecoration(
          border: InputBorder.none,
          counterText: '',
          hintText: hint,
          hintStyle: const TextStyle(
            color: AppPalette.textSecondary,
            letterSpacing: 0,
          ),
          icon: Icon(icon, color: AppPalette.candyBlue, size: 20),
        ),
        validator: validator,
      ),
    );
  }
}
