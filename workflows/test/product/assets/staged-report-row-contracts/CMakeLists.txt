cmake_minimum_required(VERSION 3.21.1)
project(DockpipeStagedReportRowContracts LANGUAGES CXX)

foreach(required PIPELANG_SDK_PREFIX PIPELANG_SDK_MANIFEST_SHA256
        PIPELANG_JSON_SHA256)
    if("${${required}}" STREQUAL "")
        message(FATAL_ERROR "${required} must be supplied explicitly")
    endif()
endforeach()
if(NOT IS_ABSOLUTE "${PIPELANG_SDK_PREFIX}")
    message(FATAL_ERROR "Select an absolute installed PipeLang SDK")
endif()
set(sdk_manifest "${PIPELANG_SDK_PREFIX}/share/pipelang-model/sdk-manifest.json")
file(SHA256 "${sdk_manifest}" actual_sdk_sha256)
if(NOT actual_sdk_sha256 STREQUAL PIPELANG_SDK_MANIFEST_SHA256)
    message(FATAL_ERROR "Selected SDK manifest differs from the supplied identity")
endif()
execute_process(
    COMMAND "${PIPELANG_SDK_PREFIX}/bin/pipelang-sdk-tool${CMAKE_EXECUTABLE_SUFFIX}" verify
        --prefix "${PIPELANG_SDK_PREFIX}" --manifest "${sdk_manifest}"
        --sha256 "${PIPELANG_SDK_MANIFEST_SHA256}"
    RESULT_VARIABLE sdk_verify_status)
if(NOT sdk_verify_status EQUAL 0)
    message(FATAL_ERROR "Selected installed SDK failed manifest verification")
endif()

set(PipeLangModel_DIR "${PIPELANG_SDK_PREFIX}/lib/cmake/PipeLangModel"
    CACHE PATH "Selected installed model package" FORCE)
find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED
    PATHS "${PipeLangModel_DIR}" NO_DEFAULT_PATH)
file(REAL_PATH "${PipeLangModel_CONFIG}" loaded_package)
file(REAL_PATH "${PIPELANG_SDK_PREFIX}/lib/cmake/PipeLangModel/PipeLangModelConfig.cmake" selected_package)
if(NOT loaded_package STREQUAL selected_package)
    message(FATAL_ERROR "CMake resolved a package outside the selected SDK")
endif()

function(json_path output input)
    string(REPLACE "\\" "\\\\" escaped "${input}")
    string(REPLACE "\"" "\\\"" escaped "${escaped}")
    string(REPLACE "\n" "\\n" escaped "${escaped}")
    string(REPLACE "\r" "\\r" escaped "${escaped}")
    string(REPLACE "\t" "\\t" escaped "${escaped}")
    set("${output}" "${escaped}" PARENT_SCOPE)
endfunction()
file(REAL_PATH "${PipeLangModel_SOURCE_LIBRARY_DIRECTORY}/json" json_root)
file(SHA256 "${json_root}/library.json" json_sha256)
if(NOT json_sha256 STREQUAL PIPELANG_JSON_SHA256)
    message(FATAL_ERROR "Selected JSON source descriptor differs from the supplied identity")
endif()
json_path(json_root_json "${json_root}")
set(source_references "${CMAKE_CURRENT_BINARY_DIR}/report-row-libraries.json")
file(WRITE "${source_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[{\"root\":\"${json_root_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PIPELANG_JSON_SHA256}\"}]}\n")

# Compile the owning Source directory directly. No implementation is copied into
# this sibling consumer, and the active host-journey entry remains unchanged.
pipelang_add_model(qualification_staged_report_rows_model
    PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../harness"
    SOURCE_ROOT src ENTRY report-projection/StagedRowsEntry.pipe
    NAMESPACE qualification_staged_report_rows SOURCE_LIBRARIES "${source_references}"
    COMPILATION COMPILED OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")

option(SANITIZERS "Instrument generated Source units and the actual JSON reader" OFF)
set(report_reader PipeLang::JsonReader)
if(SANITIZERS)
    if(NOT CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang" OR
            NOT DEFINED PipeLangModel_JSON_READER_SOURCE_DIRECTORY)
        message(FATAL_ERROR "Selected SDK/toolchain cannot instrument the owned JSON reader")
    endif()
    set(json_sources "${PipeLangModel_JSON_READER_SOURCE_DIRECTORY}")
    file(SHA256 "${json_sources}/third_party/nlohmann/json.hpp" json_header_sha256)
    if(NOT json_header_sha256 STREQUAL "aaf127c04cb31c406e5b04a63f1ae89369fccde6d8fa7cdda1ed4f32dfc5de63")
        message(FATAL_ERROR "Installed pinned JSON parser identity differs")
    endif()
    add_library(report_instrumented_reader STATIC
        "${json_sources}/Reader.cpp" "${json_sources}/DocumentWriter.cpp")
    get_target_property(json_includes PipeLang::JsonReader INTERFACE_INCLUDE_DIRECTORIES)
    target_include_directories(report_instrumented_reader PUBLIC "${json_includes}"
        PRIVATE "${json_sources}/third_party")
    target_compile_features(report_instrumented_reader PUBLIC cxx_std_20)
    set_target_properties(report_instrumented_reader PROPERTIES POSITION_INDEPENDENT_CODE ON)
    target_compile_options(report_instrumented_reader PRIVATE
        -O1 -g1 -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(report_instrumented_reader INTERFACE -fsanitize=address,undefined)
    _pipelang_model_resolve_cxx_resource_profile(resource_profile resource_response resource_digest resource_dependency)
    _pipelang_model_apply_cxx_resource_profile(report_instrumented_reader
        "${resource_profile}" "${resource_response}" "${resource_digest}")
    if(resource_response)
        target_compile_options(report_instrumented_reader PRIVATE "$<$<COMPILE_LANGUAGE:CXX>:@${resource_response}>")
    endif()
    target_compile_options(qualification_staged_report_rows_model INTERFACE
        -O1 -g1 -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(qualification_staged_report_rows_model INTERFACE -fsanitize=address,undefined)
    set(report_reader report_instrumented_reader)
endif()

add_executable(qualification_staged_report_rows_probe Main.cpp)
target_include_directories(qualification_staged_report_rows_probe PRIVATE ../harness/native)
target_link_libraries(qualification_staged_report_rows_probe PRIVATE qualification_staged_report_rows_model "${report_reader}")
target_compile_features(qualification_staged_report_rows_probe PRIVATE cxx_std_20)
set_target_properties(qualification_staged_report_rows_probe PROPERTIES CXX_EXTENSIONS OFF)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(qualification_staged_report_rows_probe PRIVATE -Wall -Wextra -Wpedantic -Werror)
endif()
enable_testing()
add_test(NAME qualification_staged_report_rows_source_contracts COMMAND qualification_staged_report_rows_probe
    "${CMAKE_CURRENT_BINARY_DIR}/report.json")
