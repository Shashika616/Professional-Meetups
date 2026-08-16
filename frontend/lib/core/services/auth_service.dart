import 'package:professional_connections_platform/core/validation/validators.dart';

/// Contract for authentication and verification.
/// The client never decides trust. It only displays what the server returns.
abstract interface class AuthService {
  Future<void> requestPhoneOtp(String phoneNumber);
  Future<void> verifyPhoneOtp({
    required String phoneNumber,
    required String code,
  });
  Future<void> connectLinkedIn(String profileUrl);
  Future<void> verifyCorporateEmail(String email);
}

/// Simulates server latency and server-side rejection for the MVP.
class MockAuthService implements AuthService {
  static const Duration latency = Duration(milliseconds: 600);

  @override
  Future<void> requestPhoneOtp(String phoneNumber) async {
    await Future<void>.delayed(latency);
  }

  @override
  Future<void> verifyPhoneOtp({
    required String phoneNumber,
    required String code,
  }) async {
    await Future<void>.delayed(latency);
    if (Validators.otp(code) != null) {
      throw const FormatException('Invalid verification code.');
    }
  }

  @override
  Future<void> connectLinkedIn(String profileUrl) async {
    await Future<void>.delayed(latency);
    if (Validators.linkedin(profileUrl) != null) {
      throw const FormatException('Invalid LinkedIn profile.');
    }
  }

  @override
  Future<void> verifyCorporateEmail(String email) async {
    await Future<void>.delayed(latency);
    if (Validators.corporateEmail(email) != null) {
      throw const FormatException('Invalid corporate email.');
    }
  }
}
