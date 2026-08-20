import 'package:flutter/foundation.dart';

/// A signed-in user's profile, sourced from two places: the one-time
/// LinkedIn callback response (`id`/`fullName`/`profilePhotoUrl`/
/// `trustLevel` — see `AuthSession`) and `GET /v1/users/me` (everything
/// else here — the Level 2/3 verification addendum, `backend/PLAN.md`'s
/// matching addendum Step E). `getProfile()` is the only source for the
/// fields below; **never** the raw phone number or email address — the
/// backend deliberately doesn't send them over the wire (Verification
/// Model § 1's "never reveal a user's full phone number" rule), so this
/// model has no field capable of holding one.
@immutable
class UserProfile {
  const UserProfile({
    required this.id,
    required this.fullName,
    this.profilePhotoUrl = '',
    // Always empty in this slice — LinkedIn's OIDC userinfo call (`scope:
    // openid profile email`) doesn't return a headline, and the backend
    // doesn't populate one. Kept as a field so it can be wired up later
    // without another model change.
    this.headline = '',
    // Level 0 (ADR-014) — a federated (Apple/Google) or email+password
    // account with no LinkedIn linked is a real, reachable state now, not
    // just "mid-onboarding," so an unset trustLevel must default to the
    // least-trusted value, never assume LinkedIn is already connected.
    this.trustLevel = 0,
    this.phoneVerified = false,
    this.personalEmailVerified = false,
    this.personalDetailsComplete = false,
    this.companyDomain = '',
    this.workEmailVerified = false,
    this.ratingAverage = 0,
    this.ratingCount = 0,
  });

  final String id;
  final String fullName;
  final String profilePhotoUrl;
  final String headline;
  final int trustLevel;
  final bool phoneVerified;
  final bool personalEmailVerified;
  final bool personalDetailsComplete;
  final String companyDomain;
  final bool workEmailVerified;

  /// Post-meetup star rating aggregate (ADR-015,
  /// docs/02-domain/domain-model.md § Rating) — 0/0 until this user has
  /// been rated at least once.
  final double ratingAverage;
  final int ratingCount;

  /// LinkedIn is the sole path to Level 1+ (ADR-014 §1: Apple/Google/email
  /// alone never grant trust, no matter how many are linked) — a first-
  /// class derived signal so call sites (ProfilePage's banner/badge) don't
  /// each re-derive `trustLevel >= 1` themselves.
  bool get linkedInConnected => trustLevel >= 1;

  /// Parses `GET /v1/users/me`'s response body. Distinct from
  /// `AuthSession.fromJson` — this is a different endpoint/response shape,
  /// not a re-parse of the same payload.
  factory UserProfile.fromJson(Map<String, dynamic> json) {
    return UserProfile(
      id: json['user_id'] as String,
      fullName: json['full_name'] as String? ?? '',
      profilePhotoUrl: json['profile_photo_url'] as String? ?? '',
      trustLevel: json['trust_level'] as int? ?? 0,
      phoneVerified: json['phone_verified'] as bool? ?? false,
      personalEmailVerified: json['personal_email_verified'] as bool? ?? false,
      personalDetailsComplete:
          json['personal_details_complete'] as bool? ?? false,
      companyDomain: json['company_domain'] as String? ?? '',
      workEmailVerified: json['work_email_verified'] as bool? ?? false,
      ratingAverage: (json['rating_average'] as num?)?.toDouble() ?? 0,
      ratingCount: json['rating_count'] as int? ?? 0,
    );
  }

  UserProfile copyWith({
    String? fullName,
    String? profilePhotoUrl,
    String? headline,
    int? trustLevel,
    bool? phoneVerified,
    bool? personalEmailVerified,
    bool? personalDetailsComplete,
    String? companyDomain,
    bool? workEmailVerified,
    double? ratingAverage,
    int? ratingCount,
  }) {
    return UserProfile(
      id: id,
      fullName: fullName ?? this.fullName,
      profilePhotoUrl: profilePhotoUrl ?? this.profilePhotoUrl,
      headline: headline ?? this.headline,
      trustLevel: trustLevel ?? this.trustLevel,
      phoneVerified: phoneVerified ?? this.phoneVerified,
      personalEmailVerified:
          personalEmailVerified ?? this.personalEmailVerified,
      personalDetailsComplete:
          personalDetailsComplete ?? this.personalDetailsComplete,
      companyDomain: companyDomain ?? this.companyDomain,
      workEmailVerified: workEmailVerified ?? this.workEmailVerified,
      ratingAverage: ratingAverage ?? this.ratingAverage,
      ratingCount: ratingCount ?? this.ratingCount,
    );
  }
}
