cmake_minimum_required(VERSION 3.18.1)

project("audio_agent" CXX C)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Optimization & Architecture Flags
add_compile_options(
    -O3
    -fPIC
    -Wall
    -Wextra
    -Wno-unused-parameter
)

if(APPLE)
    set(CMAKE_MACOSX_RPATH 1)
elseif(ANDROID AND ANDROID_ABI STREQUAL "arm64-v8a")
    add_compile_options(-march=armv8.2-a+dotprod+fp16)
endif()

# Header include search directories
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/include
)

# Core source files
set(AUDIO_AGENT_CORE_SOURCES
    src/CpuAffinity.cpp
    src/IntentArbiter.cpp
    src/StreamingSentenceChunker.cpp
    src/WatchdogAndRepetition.cpp
    src/ThreeTierMemory.cpp
    src/FullDuplexAudioIO.cpp
    src/AudioAgentPipeline.cpp
    src/MnnLlmEngine.cpp
    src/SherpaOnnxPerceptionEngine.cpp
    src/audio_agent_c_api.cpp
)

# Optional Oboe support for Android
if(ANDROID)
    find_package(oboe CONFIG QUIET)
    if(oboe_FOUND)
        message(STATUS "Found Oboe via Android Prefab.")
        add_definitions(-DAUDIO_AGENT_USE_OBOE)
    endif()
endif()

# 1. Static library (ideal for Go cgo static linking)
add_library(audio_agent_static STATIC ${AUDIO_AGENT_CORE_SOURCES})
set_target_properties(audio_agent_static PROPERTIES OUTPUT_NAME "audio_agent")

# 2. Shared library (.dylib / .so)
add_library(audio_agent SHARED ${AUDIO_AGENT_CORE_SOURCES})

if(ANDROID)
    target_link_libraries(audio_agent android log OpenSLES)
    if(oboe_FOUND)
        target_link_libraries(audio_agent oboe::oboe)
    endif()
elseif(APPLE)
    # macOS CoreAudio / AudioToolbox support
    target_link_libraries(audio_agent "-framework CoreAudio" "-framework AudioToolbox")
endif()

# 3. Unit test executable
add_executable(audio_agent_test
    ${AUDIO_AGENT_CORE_SOURCES}
    src/test_main.cpp
)

# 4. Benchmark executable
add_executable(audio_agent_bench
    ${AUDIO_AGENT_CORE_SOURCES}
    src/bench_main.cpp
)
