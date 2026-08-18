import 'package:flutter_test/flutter_test.dart';

import 'package:professional_connections_platform/core/models/user_profile.dart';

void main() {
  group('UserProfile.fromJson', () {
    test('parses a full GET /v1/users/me response', () {
      final profile = UserProfile.fromJson({
        'user_id': 'user-1',
        'full_name': 'Ada Lovelace',
        'profile_photo_url': 'https://example.com/ada.jpg',
        'trust_level': 3,
        'phone_verified': true,
        'personal_email_verified': true,
        'personal_details_complete': true,
        'company_domain': 'example.com',
        'work_email_verified': true,
      });

      expect(profile.id, 'user-1');
      expect(profile.fullName, 'Ada Lovelace');
      expect(profile.profilePhotoUrl, 'https://example.com/ada.jpg');
      expect(profile.trustLevel, 3);
      expect(profile.phoneVerified, isTrue);
      expect(profile.personalEmailVerified, isTrue);
      expect(profile.personalDetailsComplete, isTrue);
      expect(profile.companyDomain, 'example.com');
      expect(profile.workEmailVerified, isTrue);
    });

    test('defaults the four verification booleans and trust level to '
        'level-1 falsy values when fields are absent', () {
      final profile = UserProfile.fromJson({'user_id': 'user-1'});

      expect(profile.fullName, '');
      expect(profile.profilePhotoUrl, '');
      expect(profile.trustLevel, 1);
      expect(profile.phoneVerified, isFalse);
      expect(profile.personalEmailVerified, isFalse);
      expect(profile.personalDetailsComplete, isFalse);
      expect(profile.companyDomain, '');
      expect(profile.workEmailVerified, isFalse);
    });
  });
}
