import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/theme/app_palette.dart';
import 'package:professional_connections_platform/features/splash/splash_screen.dart';

/// Replaces Flutter's default error widget (which renders the raw
/// exception message and widget-library stack trace) with a clean,
/// user-safe fallback — never anything from [details] itself. A top-level
/// function, not an inline closure in [main], so it's directly testable.
Widget buildFriendlyErrorWidget(FlutterErrorDetails details) {
  return Material(
    color: AppPalette.onyx,
    child: Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(
              Icons.error_outline_rounded,
              color: AppPalette.danger,
              size: 40,
            ),
            const SizedBox(height: 16),
            const Text(
              'Something went wrong.',
              style: TextStyle(
                color: AppPalette.textPrimary,
                fontSize: 16,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 8),
            const Text(
              'Please try again. If this keeps happening, let us know.',
              textAlign: TextAlign.center,
              style: TextStyle(color: AppPalette.textSecondary, fontSize: 13),
            ),
          ],
        ),
      ),
    ),
  );
}

void main() {
  WidgetsFlutterBinding.ensureInitialized();

  // Without this, any widget-build-time exception (a bad cast, a null from
  // a provider that shouldn't be null, a RenderFlex overflow that throws)
  // falls through to Flutter's *default* error widget — which renders the
  // raw exception message and widget-library stack trace directly on
  // screen, in both debug and release builds. That's developer-facing
  // detail, not something a real user should ever see. This only replaces
  // what's *shown*; FlutterError.onError is left at its default, so the
  // full error still gets dumped to the console exactly as before — the
  // real detail is still there for whoever's looking at logs, just not
  // rendered into the app itself.
  ErrorWidget.builder = buildFriendlyErrorWidget;

  // Configure image cache for better performance
  PaintingBinding.instance.imageCache.maximumSize = 100;
  PaintingBinding.instance.imageCache.maximumSizeBytes = 100 << 20; // 100 MB

  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
      statusBarBrightness: Brightness.dark,
      systemNavigationBarColor: AppPalette.onyx,
      systemNavigationBarIconBrightness: Brightness.light,
    ),
  );
  runApp(const ProviderScope(child: ProfessionalConnectionsApp()));
}

class ProfessionalConnectionsApp extends StatelessWidget {
  const ProfessionalConnectionsApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'Professional Connections',
      theme: ThemeData(
        brightness: Brightness.dark,
        useMaterial3: true,
        scaffoldBackgroundColor: AppPalette.onyx,
        colorScheme: const ColorScheme.dark(
          primary: AppPalette.candyBlue,
          secondary: AppPalette.steelBlue,
          surface: AppPalette.surface,
          onPrimary: AppPalette.onyx,
        ),
        appBarTheme: const AppBarTheme(
          backgroundColor: Colors.transparent,
          elevation: 0,
          centerTitle: true,
          titleTextStyle: TextStyle(
            color: AppPalette.textPrimary,
            fontSize: 15,
            fontWeight: FontWeight.w600,
            letterSpacing: 2.0,
          ),
          iconTheme: IconThemeData(color: AppPalette.candyBlue),
        ),
        snackBarTheme: SnackBarThemeData(
          behavior: SnackBarBehavior.floating,
          backgroundColor: AppPalette.card,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
          ),
          contentTextStyle: const TextStyle(
            color: AppPalette.candyBlue,
            fontSize: 13,
          ),
        ),
        // Add premium page transitions
        pageTransitionsTheme: const PageTransitionsTheme(
          builders: {
            TargetPlatform.android: _ZoomPageTransitionsBuilder(),
            TargetPlatform.iOS: _ZoomPageTransitionsBuilder(),
          },
        ),
      ),
      home: const SplashScreen(),
    );
  }
}

// Custom zoom + fade transition for premium feel
class _ZoomPageTransitionsBuilder extends PageTransitionsBuilder {
  const _ZoomPageTransitionsBuilder();

  @override
  Widget buildTransitions<T>(
    PageRoute<T> route,
    BuildContext context,
    Animation<double> animation,
    Animation<double> secondaryAnimation,
    Widget child,
  ) {
    return _ZoomPageTransition(
      animation: animation,
      secondaryAnimation: secondaryAnimation,
      child: child,
    );
  }
}

class _ZoomPageTransition extends StatelessWidget {
  const _ZoomPageTransition({
    required this.animation,
    required this.secondaryAnimation,
    required this.child,
  });

  final Animation<double> animation;
  final Animation<double> secondaryAnimation;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: CurvedAnimation(parent: animation, curve: Curves.easeInOut),
      child: ScaleTransition(
        scale: Tween<double>(begin: 0.92, end: 1.0).animate(
          CurvedAnimation(parent: animation, curve: Curves.easeOutCubic),
        ),
        child: child,
      ),
    );
  }
}
