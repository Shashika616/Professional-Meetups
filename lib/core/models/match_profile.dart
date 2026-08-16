import 'package:flutter/foundation.dart';

import 'package:professional_connections_platform/core/models/intent_type.dart';

@immutable
class MatchProfile {
  const MatchProfile({
    required this.id,
    required this.name,
    required this.role,
    required this.intent,
    required this.distanceKm,
    this.isVerified = true,
  });

  final String id;
  final String name;
  final String role;
  final IntentType intent;
  final double distanceKm;
  final bool isVerified;

  String get initials => name
      .split(' ')
      .where((part) => part.isNotEmpty)
      .map((part) => part[0].toUpperCase())
      .take(2)
      .join();

  String get formattedDistance => '${distanceKm.toStringAsFixed(1)} KM';
}
