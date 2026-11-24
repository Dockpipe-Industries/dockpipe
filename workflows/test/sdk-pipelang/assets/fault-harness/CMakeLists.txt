cmake_minimum_required(VERSION 3.21.1)
project(DockpipeSDKWindowsFaultProof LANGUAGES CXX)

if(NOT WIN32)
    message(FATAL_ERROR "The process fault fixture requires an actual Windows host")
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
set(source_references "${CMAKE_CURRENT_BINARY_DIR}/fault-libraries.json")
file(WRITE "${source_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[{\"root\":\"${source_sdk_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${DOCKPIPE_SOURCE_SDK_SHA256}\"},{\"root\":\"${testing_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PIPELANG_TESTING_SHA256}\"}]}\n")

pipelang_add_model(sdk_fault_model PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}"
    ENTRY FaultHarness.pipe NAMESPACE sdk_fault COMPILATION COMPILED
    SOURCE_LIBRARIES "${source_references}"
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")
option(SANITIZERS "Instrument generated source producer, native bridge and consumer" OFF)
if(SANITIZERS)
    if(NOT CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        message(FATAL_ERROR "This sanitizer profile requires a GCC or Clang toolchain")
    endif()
    target_compile_options(sdk_fault_model INTERFACE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(sdk_fault_model INTERFACE -fsanitize=address,undefined)
endif()

include("${source_sdk_root}/cmake/DockpipePipeLang.cmake")
dockpipe_add_pipelang_bridge(sdk_fault_bridge MODEL_TARGET sdk_fault_model
    MODEL_NAMESPACE sdk_fault BRIDGE_NAMESPACE sdk_fault_host
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/bridge")
add_executable(sdk_fault_probe native/Main.cpp)
target_link_libraries(sdk_fault_probe PRIVATE sdk_fault_bridge Threads::Threads)
target_compile_features(sdk_fault_probe PRIVATE cxx_std_20)
set_target_properties(sdk_fault_probe PROPERTIES CXX_EXTENSIONS OFF)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(sdk_fault_probe PRIVATE -Wall -Wextra -Wpedantic -Werror)
endif()

# Test-only native effect artifacts, selected explicitly by source inputs.
# These targets never change the production SDK helper or introduce fault flags.
function(add_broker_fault_helper target source)
    add_executable("${target}" "${source}" "${source_sdk_root}/native/WindowsFiles.cpp")
    target_include_directories("${target}" PRIVATE "${source_sdk_root}/native")
    target_compile_features("${target}" PRIVATE cxx_std_20)
    target_compile_definitions("${target}" PRIVATE _WIN32_WINNT=0x0A00 WINVER=0x0A00 NOMINMAX)
    set_target_properties("${target}" PROPERTIES CXX_EXTENSIONS OFF)
    if(MINGW)
        target_link_options("${target}" PRIVATE -municode)
    endif()
    if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        target_compile_options("${target}" PRIVATE -Wall -Wextra -Wpedantic -Werror)
    endif()
endfunction()

add_broker_fault_helper(sdk_lost_offer_helper native/LostOffer_windows.cpp)
add_broker_fault_helper(sdk_partial_offer_helper native/PartialOffer_windows.cpp)
add_broker_fault_helper(sdk_late_offer_helper native/LateOffer_windows.cpp)
