import 'package:flutter/foundation.dart';

@immutable
class UserProfile {
  const UserProfile({
    required this.id,
    required this.fullName,
    this.role = '',
    this.company = '',
    this.trustLevel = 0,
    this.isPhoneVerified = false,
    this.isLinkedInConnected = false,
    this.isWorkEmailVerified = false,
  });

  final String id;
  final String fullName;
  final String role;
  final String company;
  final int trustLevel;
  final bool isPhoneVerified;
  final bool isLinkedInConnected;
  final bool isWorkEmailVerified;

  UserProfile copyWith({
    String? fullName,
    String? role,
    String? company,
    int? trustLevel,
    bool? isPhoneVerified,
    bool? isLinkedInConnected,
    bool? isWorkEmailVerified,
  }) {
    return UserProfile(
      id: id,
      fullName: fullName ?? this.fullName,
      role: role ?? this.role,
      company: company ?? this.company,
      trustLevel: trustLevel ?? this.trustLevel,
      isPhoneVerified: isPhoneVerified ?? this.isPhoneVerified,
      isLinkedInConnected: isLinkedInConnected ?? this.isLinkedInConnected,
      isWorkEmailVerified: isWorkEmailVerified ?? this.isWorkEmailVerified,
    );
  }
}