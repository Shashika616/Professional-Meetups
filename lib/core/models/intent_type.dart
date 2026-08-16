import 'package:flutter/material.dart';

enum IntentType {
  coffee,
  lunch,
  networking,
  mentorship,
  rideShare,
  dating;

  String get label => switch (this) {
    IntentType.coffee => 'COFFEE',
    IntentType.lunch => 'LUNCH',
    IntentType.networking => 'NETWORKING',
    IntentType.mentorship => 'MENTORSHIP',
    IntentType.rideShare => 'RIDE SHARE',
    IntentType.dating => 'DATING',
  };

  IconData get icon => switch (this) {
    IntentType.coffee => Icons.local_cafe_outlined,
    IntentType.lunch => Icons.restaurant_outlined,
    IntentType.networking => Icons.work_outline,
    IntentType.mentorship => Icons.school_outlined,
    IntentType.rideShare => Icons.directions_car_outlined,
    IntentType.dating => Icons.favorite_border,
  };

  int get requiredTrustLevel => switch (this) {
    IntentType.rideShare || IntentType.dating => 4,
    _ => 1,
  };

  bool isUnlockedFor(int trustLevel) => trustLevel >= requiredTrustLevel;
}
