#!/usr/bin/env bash
# One-time repo reorg: moves the Flutter app into frontend/, alongside
# backend/. Run once from anywhere inside the repo, review `git status`,
# then commit. Safe to delete after it's run successfully.
#
# CLAUDE.md and .github/workflows/flutter-ci.yml already assume this layout
# (Flutter root at frontend/, CI working-directory: frontend) — this script
# is what makes reality match those docs.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if [ -f frontend/pubspec.yaml ]; then
  echo "frontend/pubspec.yaml already exists — this looks like it already ran. Aborting."
  exit 1
fi

if [ ! -f pubspec.yaml ]; then
  echo "No pubspec.yaml at repo root — nothing to migrate, or this already ran. Aborting."
  exit 1
fi

mkdir -p frontend

# Stale build artifacts at the repo root (gitignored, so `git mv` can't move
# them anyway) — Flutter regenerates these fresh under frontend/ on the next
# `flutter pub get` / `flutter run`, so just drop the old copies.
rm -rf .dart_tool build
rm -f professional_connections_platform.iml

# --- Move the Flutter project into frontend/, preserving git history ---
git mv lib frontend/lib
git mv test frontend/test
git mv android frontend/android
git mv ios frontend/ios
git mv macos frontend/macos
git mv web frontend/web
git mv assets frontend/assets
git mv pubspec.yaml frontend/pubspec.yaml
git mv pubspec.lock frontend/pubspec.lock
git mv analysis_options.yaml frontend/analysis_options.yaml
git mv .metadata frontend/.metadata

# --- Flutter/Dart-specific ignores move into their own file, scoped to frontend/ ---
cat > frontend/.gitignore <<'EOF'
# Flutter/Dart/Pub related
**/doc/api/
**/ios/Flutter/.last_build_id
.dart_tool/
.flutter-plugins-dependencies
.pub-cache/
.pub/
/build/
/coverage/

# Symbolication related
app.*.symbols

# Obfuscation related
app.*.map.json

# Android Studio will place build artifacts here
/android/app/debug
/android/app/profile
/android/app/release

# Android related
**/android/**/gradle-wrapper.jar
**/android/.gradle
**/android/captures/
**/android/gradlew
**/android/gradlew.bat
**/android/local.properties
**/android/**/GeneratedPluginRegistrant.java
**/android/key.properties
*.jks
*.keystore

# iOS/Xcode related
**/ios/**/*.mode1v3
**/ios/**/*.mode2v3
**/ios/**/*.moved-aside
**/ios/**/*.pbxuser
**/ios/**/*.perspectivev3
**/ios/**/*sync/
**/ios/**/.sconsign.dblite
**/ios/**/.tags*
**/ios/**/.vagrant/
**/ios/**/DerivedData/
**/ios/**/Icon?
**/ios/**/Pods/
**/ios/**/.symlinks/
**/ios/**/profile
**/ios/**/xcuserdata
**/ios/.generated/
**/ios/Flutter/App.framework
**/ios/Flutter/Flutter.framework
**/ios/Flutter/Flutter.podspec
**/ios/Flutter/Generated.xcconfig
**/ios/Flutter/ephemeral/
**/ios/Flutter/app.flx
**/ios/Flutter/app.zip
**/ios/Flutter/flutter_assets/
**/ios/Flutter/flutter_export_environment.sh
**/ios/ServiceDefinitions.json
**/ios/Runner/GeneratedPluginRegistrant.*

# Exceptions to the Xcode-project-file rules above
!**/ios/**/default.mode1v3
!**/ios/**/default.mode2v3
!**/ios/**/default.pbxuser
!**/ios/**/default.perspectivev3

# macOS related
**/macos/Flutter/GeneratedPluginRegistrant.swift
**/macos/Flutter/ephemeral/
**/macos/**/xcuserdata
**/macos/**/DerivedData/
**/macos/**/Pods/

# Windows related (kept in case the platform is added later)
**/windows/flutter/generated_plugin_registrant.cc
**/windows/flutter/generated_plugin_registrant.h
**/windows/flutter/generated_plugins.cmake

# Linux related (kept in case the platform is added later)
**/linux/flutter/generated_plugin_registrant.cc
**/linux/flutter/generated_plugin_registrant.h
**/linux/flutter/generated_plugins.cmake
EOF

# --- Root .gitignore trimmed to repo-wide/editor/OS concerns only ---
# (Flutter-specific rules now live in frontend/.gitignore; backend/ already
# has its own .gitignore for Go build artifacts and secrets.)
cat > .gitignore <<'EOF'
# Miscellaneous
*.class
*.log
run_log.txt
*.pyc
*.swp
.DS_Store
.atom/
.build/
.buildlog/
.history
.svn/
.swiftpm/
migrate_working_dir/

# IntelliJ related
*.iml
*.ipr
*.iws
.idea/

# The .vscode folder contains launch configuration and tasks you configure in
# VS Code which you may wish to be included in version control, so this line
# is commented out by default.
#.vscode/

# Environment / secrets — never commit real credentials
.env
.env.*
*.env
!.env.example
google-services.json
GoogleService-Info.plist
EOF

git add -A

echo
echo "Done. frontend/ now holds the Flutter app; backend/ is untouched."
echo
echo "Review before committing:"
echo "  git status"
echo "  git diff --cached --stat"
echo
echo "Sanity-check the app still builds from its new location:"
echo "  cd frontend && flutter pub get && flutter analyze && flutter test"
echo
echo "Suggested commit message:"
echo "  git commit -m 'Reorganize repo: move Flutter app into frontend/, alongside backend/'"
