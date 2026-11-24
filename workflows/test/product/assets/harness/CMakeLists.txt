cmake_minimum_required(VERSION 3.21.1)
project(DockpipeProductHostJourney LANGUAGES CXX)

if(NOT CMAKE_SYSTEM_NAME STREQUAL "Linux" AND NOT WIN32)
    message(FATAL_ERROR "This product journey requires a Linux or Windows host adapter")
endif()

set(PIPELANG_PRODUCT_SDK_PREFIX "" CACHE PATH "Explicit installed and versioned PipeLang SDK")
set(PIPELANG_PRODUCT_SDK_SHA256 "" CACHE STRING "Expected SDK manifest SHA256")
set(PIPELANG_PRODUCT_TESTING_SHA256 "" CACHE STRING "Expected installed testing library descriptor SHA256")
foreach(required PIPELANG_PRODUCT_SDK_PREFIX PIPELANG_PRODUCT_SDK_SHA256 PIPELANG_PRODUCT_TESTING_SHA256
        DOCKPIPE_SOURCE_SDK_DESCRIPTOR DOCKPIPE_SOURCE_SDK_SHA256)
    if("${${required}}" STREQUAL "")
        message(FATAL_ERROR "${required} must be explicitly supplied")
    endif()
endforeach()
if(NOT IS_ABSOLUTE "${PIPELANG_PRODUCT_SDK_PREFIX}")
    message(FATAL_ERROR "Select an absolute installed SDK prefix")
endif()
set(sdk_manifest "${PIPELANG_PRODUCT_SDK_PREFIX}/share/pipelang-model/sdk-manifest.json")
file(SHA256 "${sdk_manifest}" actual_sdk_sha256)
if(NOT actual_sdk_sha256 STREQUAL PIPELANG_PRODUCT_SDK_SHA256)
    message(FATAL_ERROR "Selected SDK manifest does not match the operator-supplied SHA256")
endif()
execute_process(
    COMMAND "${PIPELANG_PRODUCT_SDK_PREFIX}/bin/pipelang-sdk-tool${CMAKE_EXECUTABLE_SUFFIX}" verify
        --prefix "${PIPELANG_PRODUCT_SDK_PREFIX}" --manifest "${sdk_manifest}"
        --sha256 "${PIPELANG_PRODUCT_SDK_SHA256}"
    RESULT_VARIABLE sdk_verify_status)
if(NOT sdk_verify_status EQUAL 0)
    message(FATAL_ERROR "Selected SDK failed installed manifest verification")
endif()

set(PipeLangModel_DIR "${PIPELANG_PRODUCT_SDK_PREFIX}/lib/cmake/PipeLangModel"
    CACHE PATH "Selected SDK CMake package directory" FORCE)
find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED
    PATHS "${PipeLangModel_DIR}" NO_DEFAULT_PATH)
file(REAL_PATH "${PipeLangModel_CONFIG}" loaded_package)
file(REAL_PATH "${PIPELANG_PRODUCT_SDK_PREFIX}/lib/cmake/PipeLangModel/PipeLangModelConfig.cmake"
    selected_package)
if(NOT loaded_package STREQUAL selected_package)
    message(FATAL_ERROR "CMake resolved a PipeLang package outside the selected SDK")
endif()
find_package(Threads REQUIRED)
file(REAL_PATH "${PipeLangModel_SOURCE_LIBRARY_DIRECTORY}/testing" testing_root)
file(SHA256 "${testing_root}/library.json" testing_sha256)
if(NOT testing_sha256 STREQUAL PIPELANG_PRODUCT_TESTING_SHA256)
    message(FATAL_ERROR "Installed testing library descriptor does not match the selected immutable identity")
endif()
string(REPLACE "\\" "\\\\" testing_json "${testing_root}")
string(REPLACE "\"" "\\\"" testing_json "${testing_json}")
string(REPLACE "\n" "\\n" testing_json "${testing_json}")
string(REPLACE "\r" "\\r" testing_json "${testing_json}")
string(REPLACE "\t" "\\t" testing_json "${testing_json}")
if(NOT IS_ABSOLUTE "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}")
    message(FATAL_ERROR "Select an absolute materialized Dockpipe SDK descriptor")
endif()
file(SHA256 "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}" actual_dockpipe_sdk_sha256)
if(NOT actual_dockpipe_sdk_sha256 STREQUAL DOCKPIPE_SOURCE_SDK_SHA256)
    message(FATAL_ERROR "Dockpipe SDK descriptor identity does not match")
endif()
get_filename_component(dockpipe_sdk_root "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}" DIRECTORY)
string(REPLACE "\\" "\\\\" dockpipe_sdk_json "${dockpipe_sdk_root}")
string(REPLACE "\"" "\\\"" dockpipe_sdk_json "${dockpipe_sdk_json}")
string(REPLACE "\n" "\\n" dockpipe_sdk_json "${dockpipe_sdk_json}")
string(REPLACE "\r" "\\r" dockpipe_sdk_json "${dockpipe_sdk_json}")
string(REPLACE "\t" "\\t" dockpipe_sdk_json "${dockpipe_sdk_json}")
set(testing_references "${CMAKE_CURRENT_BINARY_DIR}/testing-libraries.json")
file(WRITE "${testing_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[{\"root\":\"${testing_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${testing_sha256}\"},{\"root\":\"${dockpipe_sdk_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${DOCKPIPE_SOURCE_SDK_SHA256}\"}]}\n")

pipelang_add_model(product_host_model
    PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}"
    ENTRY Entry.pipe
    NAMESPACE dockpipe_product
    SOURCE_LIBRARIES "${testing_references}"
    COMPILATION COMPILED
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")

option(PIPELANG_PRODUCT_SANITIZERS "Instrument source producer, generated units and all native consumer adapters" OFF)
if(PIPELANG_PRODUCT_SANITIZERS)
    target_compile_options(product_host_model INTERFACE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(product_host_model INTERFACE -fsanitize=address,undefined)
endif()

include("${dockpipe_sdk_root}/cmake/DockpipePipeLang.cmake")
dockpipe_add_pipelang_bridge(product_sdk_bridge
    MODEL_TARGET product_host_model MODEL_NAMESPACE dockpipe_product
    BRIDGE_NAMESPACE product_sdk OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/sdk-bridge")
set(product_native_sources "")
if(NOT WIN32)
    list(APPEND product_native_sources native/ConfinedFiles.cpp)
endif()

add_executable(dockpipe_product_host Main.cpp ${product_native_sources})
target_link_libraries(dockpipe_product_host PRIVATE product_sdk_bridge Threads::Threads)
target_compile_features(dockpipe_product_host PRIVATE cxx_std_20)
set_target_properties(dockpipe_product_host PROPERTIES CXX_EXTENSIONS OFF)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(dockpipe_product_host PRIVATE -Wall -Wextra -Wpedantic -Werror)
endif()
