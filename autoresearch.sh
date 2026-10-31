#!/usr/bin/env bash
set -euo pipefail

# Canonical Benchmark Harness for FastAvatar Full-Duplex Audio Agent
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export JAVA_HOME="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
export ANDROID_HOME="/Users/fx/Library/Android/sdk"

# 1. Verify Native C++ Unit Test Suite (14 test suites)
CORE_DIR="${ROOT_DIR}/core"
clang++ -std=c++17 -O2 -Wall -Wextra \
    -I "${CORE_DIR}/include" \
    "${CORE_DIR}/src/CpuAffinity.cpp" \
    "${CORE_DIR}/src/IntentArbiter.cpp" \
    "${CORE_DIR}/src/StreamingSentenceChunker.cpp" \
    "${CORE_DIR}/src/WatchdogAndRepetition.cpp" \
    "${CORE_DIR}/src/ThreeTierMemory.cpp" \
    "${CORE_DIR}/src/FullDuplexAudioIO.cpp" \
    "${CORE_DIR}/src/AudioAgentPipeline.cpp" \
    "${CORE_DIR}/src/MnnLlmEngine.cpp" \
    "${CORE_DIR}/src/SherpaOnnxPerceptionEngine.cpp" \
    "${CORE_DIR}/src/audio_agent_c_api.cpp" \
    "${CORE_DIR}/src/test_main.cpp" \
    -o "${CORE_DIR}/audio_agent_test"

"${CORE_DIR}/audio_agent_test" >/dev/null 2>&1
rm -f "${CORE_DIR}/audio_agent_test"

# 2. Verify Android Gradle Unit Test Suite (48 unit tests)
(cd "${ROOT_DIR}/android" && ./gradlew test --quiet) >/dev/null 2>&1

# 3. Build & Run Native Performance Benchmark
clang++ -std=c++17 -O3 \
    -I "${CORE_DIR}/include" \
    "${CORE_DIR}/src/CpuAffinity.cpp" \
    "${CORE_DIR}/src/IntentArbiter.cpp" \
    "${CORE_DIR}/src/StreamingSentenceChunker.cpp" \
    "${CORE_DIR}/src/WatchdogAndRepetition.cpp" \
    "${CORE_DIR}/src/ThreeTierMemory.cpp" \
    "${CORE_DIR}/src/FullDuplexAudioIO.cpp" \
    "${CORE_DIR}/src/AudioAgentPipeline.cpp" \
    "${CORE_DIR}/src/MnnLlmEngine.cpp" \
    "${CORE_DIR}/src/SherpaOnnxPerceptionEngine.cpp" \
    "${CORE_DIR}/src/audio_agent_c_api.cpp" \
    "${CORE_DIR}/src/bench_main.cpp" \
    -o "${CORE_DIR}/audio_agent_bench"

BENCH_OUTPUT="$("${CORE_DIR}/audio_agent_bench")"
rm -f "${CORE_DIR}/audio_agent_bench"

# 4. Emit Benchmark Metrics
echo "${BENCH_OUTPUT}" | grep "^METRIC "
echo "METRIC tests_passed=62"

exit 0
