import 'package:professional_connections_platform/core/models/intent_type.dart';
import 'package:professional_connections_platform/core/models/match_profile.dart';
import 'package:professional_connections_platform/core/models/paged_result.dart';

/// Contract for match discovery. Cursor-based, server-owned pagination.
abstract interface class MatchingService {
  Future<PagedResult<MatchProfile>> fetchMatches({
    required IntentType intent,
    String? cursor,
    int limit = 10,
  });
}

class MockMatchingService implements MatchingService {
  static const List<MatchProfile> _catalog = [
    MatchProfile(id: 'u1', name: 'Nimal Perera', role: 'Software Engineer', intent: IntentType.coffee, distanceKm: 1.2),
    MatchProfile(id: 'u2', name: 'Sachini Fernando', role: 'Product Manager', intent: IntentType.networking, distanceKm: 2.5),
    MatchProfile(id: 'u3', name: 'Kasun Silva', role: 'Founder', intent: IntentType.mentorship, distanceKm: 3.1),
    MatchProfile(id: 'u4', name: 'Amaya Wickrama', role: 'Data Scientist', intent: IntentType.lunch, distanceKm: 1.8),
    MatchProfile(id: 'u5', name: 'Ruwan Jayasuriya', role: 'UX Designer', intent: IntentType.coffee, distanceKm: 4.2),
  ];

  @override
  Future<PagedResult<MatchProfile>> fetchMatches({
    required IntentType intent,
    String? cursor,
    int limit = 10,
  }) async {
    await Future<void>.delayed(const Duration(milliseconds: 500));
    final start = cursor == null ? 0 : int.tryParse(cursor) ?? 0;
    final pool = _catalog.where((match) => match.intent == intent).toList();
    final page = pool.skip(start).take(limit).toList();
    final next = start + page.length;
    return PagedResult(
      items: List.unmodifiable(page),
      nextCursor: next < pool.length ? next.toString() : null,
      hasMore: next < pool.length,
    );
  }
}