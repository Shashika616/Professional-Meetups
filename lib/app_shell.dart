import 'package:flutter/material.dart';

import 'package:professional_connections_platform/core/widgets/app_background.dart';
import 'package:professional_connections_platform/core/widgets/glass_bottom_bar.dart';
import 'package:professional_connections_platform/features/chats/chats_page.dart';
import 'package:professional_connections_platform/features/home/home_page.dart';
import 'package:professional_connections_platform/features/matches/matches_page.dart';
import 'package:professional_connections_platform/features/profile/profile_page.dart';
import 'package:professional_connections_platform/features/safety/safety_page.dart';

class AppShell extends StatefulWidget {
  const AppShell({super.key});

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
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