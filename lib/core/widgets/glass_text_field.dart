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
  });

  final TextEditingController controller;
  final IconData icon;
  final String hint;
  final TextInputType? keyboardType;
  final String? Function(String?)? validator;
  final int? maxLength;
  final TextAlign textAlign;
  final double? letterSpacing;

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
        style: TextStyle(
          color: AppPalette.textPrimary,
          fontSize: 16,
          letterSpacing: letterSpacing,
        ),
        decoration: InputDecoration(
          border: InputBorder.none,
          counterText: '',
          hintText: hint,
          hintStyle: const TextStyle(color: AppPalette.textSecondary, letterSpacing: 0),
          icon: Icon(icon, color: AppPalette.candyBlue, size: 20),
        ),
        validator: validator,
      ),
    );
  }
}