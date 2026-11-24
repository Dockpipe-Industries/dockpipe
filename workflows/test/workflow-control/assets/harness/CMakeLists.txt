cmake_minimum_required(VERSION 3.21.1)
project(DockpipeWorkflowControlProof LANGUAGES CXX)

if(NOT WIN32 AND NOT CMAKE_SYSTEM_NAME STREQUAL "Linux")
    message(FATAL_ERROR "The workflow-control consumer supports actual Windows and Linux hosts")
endif()

foreach(required PIPELANG_SDK_PREFIX PIPELANG_SDK_MANIFEST_SHA256 PIPELANG_TESTING_SHA256
        DOCKPIPE_SOURCE_SDK_DESCRIPTOR DOCKPIPE_SOURCE_SDK_SHA256)
    if("${${required}}" STREQUAL "")
        message(FATAL_ERROR "${required} must be supplied explicitly")
    endif()
endforeach()
if(NOT IS_ABSOLUTE "${PIPELANG_SDK_PREFIX}" OR NOT IS_ABSOLUTE "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}")
    message(FATAL_ERROR "Select absolute installed and materialized SDK paths")
endif()

set(sdk_manifest "${PIPELANG_SDK_PREFIX}/share/pipelang-model/sdk-manifest.json")
file(SHA256 "${sdk_manifest}" actual_sdk_sha256)
file(SHA256 "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}" actual_source_sha256)
if(NOT actual_sdk_sha256 STREQUAL PIPELANG_SDK_MANIFEST_SHA256 OR
        NOT actual_source_sha256 STREQUAL DOCKPIPE_SOURCE_SDK_SHA256)
    message(FATAL_ERROR "Selected SDK identities do not match the supplied hashes")
endif()
execute_process(
    COMMAND "${PIPELANG_SDK_PREFIX}/bin/pipelang-sdk-tool${CMAKE_EXECUTABLE_SUFFIX}" verify
        --prefix "${PIPELANG_SDK_PREFIX}" --manifest "${sdk_manifest}"
        --sha256 "${PIPELANG_SDK_MANIFEST_SHA256}"
    RESULT_VARIABLE sdk_verify_status)
if(NOT sdk_verify_status EQUAL 0)
    message(FATAL_ERROR "Selected installed PipeLang SDK failed manifest verification")
endif()

set(PipeLangModel_DIR "${PIPELANG_SDK_PREFIX}/lib/cmake/PipeLangModel"
    CACHE PATH "Explicit installed PipeLang package directory" FORCE)
find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED
    PATHS "${PipeLangModel_DIR}" NO_DEFAULT_PATH)
file(REAL_PATH "${PipeLangModel_CONFIG}" loaded_package)
file(REAL_PATH "${PIPELANG_SDK_PREFIX}/lib/cmake/PipeLangModel/PipeLangModelConfig.cmake" selected_package)
if(NOT loaded_package STREQUAL selected_package)
    message(FATAL_ERROR "CMake resolved a package outside the selected installed SDK")
endif()
find_package(Threads REQUIRED)

get_filename_component(source_sdk_root "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}" DIRECTORY)
file(REAL_PATH "${PipeLangModel_SOURCE_LIBRARY_DIRECTORY}/testing" testing_root)
file(SHA256 "${testing_root}/library.json" actual_testing_sha256)
if(NOT actual_testing_sha256 STREQUAL PIPELANG_TESTING_SHA256)
    message(FATAL_ERROR "Selected testing library descriptor identity does not match")
endif()

function(escape_json_path output value)
    string(REPLACE "\\" "\\\\" escaped "${value}")
    string(REPLACE "\"" "\\\"" escaped "${escaped}")
    string(REPLACE "\n" "\\n" escaped "${escaped}")
    string(REPLACE "\r" "\\r" escaped "${escaped}")
    string(REPLACE "\t" "\\t" escaped "${escaped}")
    set("${output}" "${escaped}" PARENT_SCOPE)
endfunction()
escape_json_path(source_sdk_json "${source_sdk_root}")
escape_json_path(testing_json "${testing_root}")
set(source_references "${CMAKE_CURRENT_BINARY_DIR}/control-libraries.json")
file(WRITE "${source_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[{\"root\":\"${source_sdk_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${DOCKPIPE_SOURCE_SDK_SHA256}\"},{\"root\":\"${testing_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PIPELANG_TESTING_SHA256}\"}]}\n")

pipelang_add_model(workflow_control_model PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}"
    ENTRY ControlHarness.pipe NAMESPACE workflow_control_proof COMPILATION COMPILED
    SOURCE_LIBRARIES "${source_references}"
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")
option(SANITIZERS "Instrument generated source producer, native bridge and consumer" OFF)
if(SANITIZERS)
    if(NOT CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        message(FATAL_ERROR "This sanitizer profile requires a GCC or Clang toolchain")
    endif()
    target_compile_options(workflow_control_model INTERFACE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(workflow_control_model INTERFACE -fsanitize=address,undefined)
endif()

include("${source_sdk_root}/cmake/DockpipePipeLang.cmake")
dockpipe_add_pipelang_bridge(workflow_control_bridge MODEL_TARGET workflow_control_model
    MODEL_NAMESPACE workflow_control_proof BRIDGE_NAMESPACE workflow_control_host
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/bridge")
set(control_native_sources native/Main.cpp)
if(NOT WIN32)
    set(product_native_root "${CMAKE_CURRENT_SOURCE_DIR}/../../../product/assets/harness/native")
    list(APPEND control_native_sources "${product_native_root}/ConfinedFiles.cpp")
endif()

add_executable(workflow_control_probe ${control_native_sources})
if(NOT WIN32)
    target_include_directories(workflow_control_probe PRIVATE "${product_native_root}")
endif()
target_link_libraries(workflow_control_probe PRIVATE workflow_control_bridge Threads::Threads)
target_compile_features(workflow_control_probe PRIVATE cxx_std_20)
set_target_properties(workflow_control_probe PROPERTIES CXX_EXTENSIONS OFF)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(workflow_control_probe PRIVATE -Wall -Wextra -Wpedantic -Werror)
endif()
