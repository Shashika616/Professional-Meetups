import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod/legacy.dart' show StateProvider;

import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/match_profile.dart';
import 'package:professional_connections_platform/core/models/paged_result.dart';
import 'package:professional_connections_platform/core/services/auth_service.dart';
import 'package:professional_connections_platform/core/services/matching_service.dart';

final authServiceProvider = Provider<AuthService>((ref) => MockAuthService());

final matchingServiceProvider = Provider<MatchingService>(
  (ref) => MockMatchingService(),
);

final selectedIntentProvider = StateProvider<IntentType>(
  (ref) => IntentType.coffee,
);

final matchesProvider = FutureProvider.autoDispose
    .family<PagedResult<MatchProfile>, IntentType>(
      (ref, intent) =>
          ref.watch(matchingServiceProvider).fetchMatches(intent: intent),
    );

final homeStatsProvider = FutureProvider.autoDispose<Map<String, dynamic>>((
  ref,
) async {
  await Future.delayed(const Duration(seconds: 1));
  return {'nearby': 128, 'meetups': 12, 'trustScore': 4.9};
});
