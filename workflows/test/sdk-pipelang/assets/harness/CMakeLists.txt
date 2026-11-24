cmake_minimum_required(VERSION 3.21.1)
project(DockpipeSDKProof LANGUAGES CXX)
foreach(required PIPELANG_SDK_PREFIX PIPELANG_SDK_MANIFEST_SHA256
        DOCKPIPE_SOURCE_SDK_DESCRIPTOR DOCKPIPE_SOURCE_SDK_SHA256)
    if("${${required}}" STREQUAL "")
        message(FATAL_ERROR "${required} must be supplied explicitly")
    endif()
endforeach()
if(NOT IS_ABSOLUTE "${PIPELANG_SDK_PREFIX}" OR NOT IS_ABSOLUTE "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}")
    message(FATAL_ERROR "Select absolute installed SDK paths")
endif()
file(SHA256 "${PIPELANG_SDK_PREFIX}/share/pipelang-model/sdk-manifest.json" actual_sdk)
file(SHA256 "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}" actual_source_sdk)
if(NOT actual_sdk STREQUAL PIPELANG_SDK_MANIFEST_SHA256 OR
        NOT actual_source_sdk STREQUAL DOCKPIPE_SOURCE_SDK_SHA256)
    message(FATAL_ERROR "Selected SDK identities do not match")
endif()
find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED
    PATHS "${PIPELANG_SDK_PREFIX}/lib/cmake/PipeLangModel" NO_DEFAULT_PATH)
get_filename_component(source_sdk_root "${DOCKPIPE_SOURCE_SDK_DESCRIPTOR}" DIRECTORY)

string(REPLACE "\\" "\\\\" source_sdk_json "${source_sdk_root}")
string(REPLACE "\"" "\\\"" source_sdk_json "${source_sdk_json}")
string(REPLACE "\n" "\\n" source_sdk_json "${source_sdk_json}")
string(REPLACE "\r" "\\r" source_sdk_json "${source_sdk_json}")
string(REPLACE "\t" "\\t" source_sdk_json "${source_sdk_json}")
set(source_sdk_references "${CMAKE_CURRENT_BINARY_DIR}/sdk-libraries.json")
file(WRITE "${source_sdk_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[{\"root\":\"${source_sdk_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${DOCKPIPE_SOURCE_SDK_SHA256}\"}]}\n")


pipelang_add_model(sdk_model PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}"
    ENTRY Entry.pipe NAMESPACE sdk_proof COMPILATION COMPILED
    SOURCE_LIBRARIES "${source_sdk_references}"
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")
option(SANITIZERS "Instrument complete producer and consumer" OFF)
if(SANITIZERS)
    target_compile_options(sdk_model INTERFACE -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(sdk_model INTERFACE -fsanitize=address,undefined)
endif()
include("${source_sdk_root}/cmake/DockpipePipeLang.cmake")
dockpipe_add_pipelang_bridge(sdk_bridge MODEL_TARGET sdk_model
    MODEL_NAMESPACE sdk_proof BRIDGE_NAMESPACE sdk_proof_host
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/bridge")
add_executable(sdk_probe Main.cpp)
target_link_libraries(sdk_probe PRIVATE sdk_bridge)
target_compile_options(sdk_probe PRIVATE -Wall -Wextra -Wpedantic -Werror)
