cmake_minimum_required(VERSION 3.21.1)
project(DockpipeQualificationCoverageContracts LANGUAGES CXX)

find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED)
if(NOT DEFINED PIPELANG_CORE_MANIFEST_SHA256 OR
        NOT PipeLangModel_MANIFEST_SHA256 STREQUAL PIPELANG_CORE_MANIFEST_SHA256)
    message(FATAL_ERROR "Select an installed core SDK with its exact manifest pin")
endif()
if(NOT IS_ABSOLUTE "${PIPELANG_TESTING_LIBRARY_ROOT}")
    message(FATAL_ERROR "Select an absolute immutable Testing source artifact")
endif()
file(REAL_PATH "${PIPELANG_TESTING_LIBRARY_ROOT}" testing_root)
file(SHA256 "${testing_root}/library.json" testing_sha256)
if(NOT testing_sha256 STREQUAL PIPELANG_TESTING_SHA256)
    message(FATAL_ERROR "Selected Testing descriptor differs from its pin")
endif()

string(REPLACE "\\" "\\\\" testing_json "${testing_root}")
string(REPLACE "\"" "\\\"" testing_json "${testing_json}")
string(REPLACE "\n" "\\n" testing_json "${testing_json}")
string(REPLACE "\r" "\\r" testing_json "${testing_json}")
string(REPLACE "\t" "\\t" testing_json "${testing_json}")
set(source_references "${CMAKE_CURRENT_BINARY_DIR}/coverage-libraries.json")
file(WRITE "${source_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[{\"root\":\"${testing_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${testing_sha256}\"}]}\n")

pipelang_add_model(qualification_coverage_model
    PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../harness"
    SOURCE_ROOT src/coverage ENTRY CoverageEntry.pipe
    NAMESPACE qualification_coverage_proof SOURCE_LIBRARIES "${source_references}"
    COMPILATION COMPILED OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")

option(SANITIZERS "Instrument generated coverage Source and native composition" OFF)
if(SANITIZERS)
    if(NOT CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        message(FATAL_ERROR "The coverage sanitizer lane requires GNU or Clang")
    endif()

    target_compile_options(qualification_coverage_model INTERFACE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(qualification_coverage_model INTERFACE -fsanitize=address,undefined)
endif()
add_executable(qualification_coverage_probe Main.cpp)
target_link_libraries(qualification_coverage_probe PRIVATE qualification_coverage_model)
target_compile_features(qualification_coverage_probe PRIVATE cxx_std_20)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(qualification_coverage_probe PRIVATE -Wall -Wextra -Wpedantic -Werror)
endif()
