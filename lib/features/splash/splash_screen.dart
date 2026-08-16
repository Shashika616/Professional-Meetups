import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/core/widgets/app_icon.dart';
import 'package:professional_connections_platform/features/landing/landing_page.dart';

class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen> {
  @override
  void initState() {
    super.initState();
    Future.delayed(const Duration(seconds: 2), () {
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (context) => const LandingPage()),
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Container(
        width: double.infinity,
        height: double.infinity,
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [
              AppPalette.onyx,
              AppPalette.surface,
              AppPalette.deepBlue,
            ],
            stops: [0.0, 0.55, 1.0],
          ),
        ),
        child: SafeArea(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              // Big, professional, glowing app icon (no forced circle)
              const AppIcon(size: 170, radius: 40, glow: true),
              const SizedBox(height: 28),
              const Text(
                'PROFESSIONAL\nCONNECTIONS',
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: AppPalette.textPrimary,
                  fontSize: 18,
                  fontWeight: FontWeight.w800,
                  letterSpacing: 3.0,
                  height: 1.4,
                ),
              ),
              const SizedBox(height: 8),
              const Text(
                'Connect Beyond the Office.',
                style: TextStyle(fontSize: 12, color: AppPalette.textSecondary),
              ),
              const SizedBox(height: 44),
              SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: AppPalette.candyBlue.withValues(alpha: 0.7),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}