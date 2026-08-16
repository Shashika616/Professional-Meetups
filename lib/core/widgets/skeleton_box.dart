import 'package:flutter/material.dart';

class SkeletonBox extends StatelessWidget {
  const SkeletonBox({
    super.key,
    required this.width,
    required this.height,
    this.radius = 6,
    this.opacity = 0.06,
  });

  final double width;
  final double height;
  final double radius;
  final double opacity;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: opacity),
        borderRadius: BorderRadius.circular(radius),
      ),
    );
  }
}