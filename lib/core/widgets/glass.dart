import 'dart:ui';

import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';

class Glass extends StatelessWidget {
  const Glass({
    super.key,
    required this.child,
    this.radius = 20,
    this.padding,
    this.tint,
    this.border,
    this.blur = 10, 
  });

  final Widget child;
  final double radius;
  final EdgeInsetsGeometry? padding;
  final Color? tint;
  final Color? border;
  final double blur;

  @override
  Widget build(BuildContext context) {
    return RepaintBoundary(
      child: ClipRRect(
        borderRadius: BorderRadius.circular(radius),
        child: BackdropFilter(
          filter: ImageFilter.blur(sigmaX: blur, sigmaY: blur),
          child: Container(
            padding: padding,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(radius),
              color: tint ?? AppPalette.glassTint,
              border: Border.all(color: border ?? AppPalette.glassBorder, width: 1),
            ),
            child: child,
          ),
        ),
      ),
    );
  }
}