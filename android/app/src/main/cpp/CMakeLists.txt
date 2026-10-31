cmake_minimum_required(VERSION 3.18.1)

project("audio_agent" CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Optimization & Architecture Flags (Section 3.2)
add_compile_options(
    -O3
    -fPIC
    -Wall
    -Wextra
    -Wno-unused-parameter
)

if(ANDROID_ABI STREQUAL "arm64-v8a")
    add_compile_options(-march=armv8.2-a+dotprod+fp16)
endif()

set(CORE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../../../../core")

# Header include search directories
include_directories(
    ${CORE_DIR}/include
    ${CMAKE_CURRENT_SOURCE_DIR}/include
)

# Source files
set(AUDIO_AGENT_SOURCES
    ${CORE_DIR}/src/CpuAffinity.cpp
    ${CORE_DIR}/src/IntentArbiter.cpp
    ${CORE_DIR}/src/StreamingSentenceChunker.cpp
    ${CORE_DIR}/src/WatchdogAndRepetition.cpp
    ${CORE_DIR}/src/ThreeTierMemory.cpp
    ${CORE_DIR}/src/FullDuplexAudioIO.cpp
    ${CORE_DIR}/src/AudioAgentPipeline.cpp
    ${CORE_DIR}/src/MnnLlmEngine.cpp
    ${CORE_DIR}/src/SherpaOnnxPerceptionEngine.cpp
    ${CORE_DIR}/src/audio_agent_c_api.cpp
    src/NativeJniBridge.cpp
)

# Check for Oboe package via Android Prefab
find_package(oboe CONFIG QUIET)

if(oboe_FOUND)
    message(STATUS "Found Oboe via Android Prefab.")
    add_definitions(-DAUDIO_AGENT_USE_OBOE)
else()
    message(STATUS "Oboe not found via Prefab, attempting FetchContent or fallback.")
endif()

# Build shared library libaudio_agent.so
add_library(audio_agent SHARED ${AUDIO_AGENT_SOURCES})

# Target link dependencies
target_link_libraries(
    audio_agent
    android
    log
    OpenSLES
)

if(oboe_FOUND)
    target_link_libraries(audio_agent oboe::oboe)
endif()
