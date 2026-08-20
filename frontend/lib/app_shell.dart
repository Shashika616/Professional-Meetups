import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:professional_connections_platform/core/providers/app_providers.dart';
import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/glass_bottom_bar.dart';
import 'package:professional_connections_platform/features/chats/chats_page.dart';
import 'package:professional_connections_platform/features/home/home_page.dart';
import 'package:professional_connections_platform/features/landing/landing_page.dart';
import 'package:professional_connections_platform/features/matches/matches_page.dart';
import 'package:professional_connections_platform/features/profile/profile_page.dart';
import 'package:professional_connections_platform/features/safety/safety_page.dart';

class AppShell extends ConsumerStatefulWidget {
  const AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  int currentIndex = 0;

  final List<Widget> pages = const [
    HomePage(),
    MatchesPage(),
    SafetyPage(),
    ChatsPage(),
    ProfilePage(),
  ];

  @override
  Widget build(BuildContext context) {
    // Safety net for any authenticated call site that forgot its own
    // SessionExpiredException catch (`frontend/PLAN.md`'s "Session refresh
    // wiring fix" addendum, Step 5.2): whenever authSessionProvider
    // transitions from logged-in to logged-out while AppShell is mounted —
    // whether via an explicit forceSignOut() elsewhere or a gap this missed
    // — land on LandingPage with a "session expired" explanation, rather
    // than stranding the user on a dead session where every action silently
    // fails. Distinct from ProfilePage's voluntary sign-out: no confirmation
    // dialog here, since nothing was confirmed — this already happened.
    ref.listen<AsyncValue<AuthSessionState>>(authSessionProvider, (
      previous,
      next,
    ) {
      final wasLoggedIn = previous?.value?.isLoggedIn ?? false;
      final isLoggedIn = next.value?.isLoggedIn ?? false;
      if (wasLoggedIn && !isLoggedIn) {
        Navigator.of(context).pushAndRemoveUntil(
          MaterialPageRoute(
            builder: (context) => const LandingPage(sessionExpired: true),
          ),
          (route) => false,
        );
      }
    });

    return Scaffold(
      extendBody: true,
      backgroundColor: Colors.transparent,
      body: AppBackground(child: pages[currentIndex]),
      bottomNavigationBar: GlassBottomBar(
        index: currentIndex,
        onTap: (index) => setState(() => currentIndex = index),
      ),
    );
  }
}
