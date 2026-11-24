cmake_minimum_required(VERSION 3.21.1)
project(DockpipeWorkflowControl LANGUAGES CXX)

foreach(required SDK_PREFIX SDK_SHA256)
    if("${${required}}" STREQUAL "")
        message(FATAL_ERROR "${required} must be supplied explicitly")
    endif()
endforeach()
if(NOT IS_ABSOLUTE "${SDK_PREFIX}" OR SDK_PREFIX MATCHES "[;\r\n]")
    message(FATAL_ERROR "SDK_PREFIX must select an absolute installed SDK")
endif()
string(LENGTH "${SDK_SHA256}" sdk_digest_length)
if(NOT sdk_digest_length EQUAL 64 OR NOT SDK_SHA256 MATCHES "^[0-9a-f]+$")
    message(FATAL_ERROR "SDK_SHA256 must be a lowercase SHA-256 digest")
endif()
file(REAL_PATH "${SDK_PREFIX}" selected_sdk)
set(selected_manifest "${selected_sdk}/share/pipelang-model/sdk-manifest.json")
file(SHA256 "${selected_manifest}" actual_sdk_digest)
if(NOT actual_sdk_digest STREQUAL SDK_SHA256)
    message(FATAL_ERROR "Selected SDK manifest differs from SDK_SHA256")
endif()
execute_process(
    COMMAND "${selected_sdk}/bin/pipelang-sdk-tool${CMAKE_EXECUTABLE_SUFFIX}" verify
        --prefix "${selected_sdk}" --manifest "${selected_manifest}" --sha256 "${SDK_SHA256}"
    RESULT_VARIABLE sdk_verification_result
    OUTPUT_QUIET ERROR_VARIABLE sdk_verification_error)
if(NOT sdk_verification_result EQUAL 0)
    message(FATAL_ERROR "Selected installed SDK verification failed: ${sdk_verification_error}")
endif()

# An inherited package cache must not substitute a different SDK installation.
set(PipeLangModel_DIR "${selected_sdk}/lib/cmake/PipeLangModel")
find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED
    PATHS "${PipeLangModel_DIR}" NO_DEFAULT_PATH)
file(REAL_PATH "${PipeLangModel_CONFIG}" loaded_config)
file(REAL_PATH "${PipeLangModel_SDK_PREFIX}" loaded_sdk)
file(REAL_PATH "${selected_sdk}/lib/cmake/PipeLangModel/PipeLangModelConfig.cmake" selected_config)
if(NOT loaded_config STREQUAL selected_config OR NOT loaded_sdk STREQUAL selected_sdk OR
        NOT PipeLangModel_MANIFEST_SHA256 STREQUAL SDK_SHA256)
    message(FATAL_ERROR "Loaded PipeLang package does not match the selected installed SDK")
endif()

pipelang_add_model(control_model PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}"
    SOURCE_ROOT src ENTRY Entry.pipe NAMESPACE dockpipe_control COMPILATION COMPILED
    OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")
configure_file(Main.cpp.in "${CMAKE_CURRENT_BINARY_DIR}/Main.cpp" COPYONLY)
add_executable(control "${CMAKE_CURRENT_BINARY_DIR}/Main.cpp")
target_include_directories(control PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}")
target_link_libraries(control PRIVATE control_model)
target_compile_features(control PRIVATE cxx_std_20)

# The admitted controller is one executable, including after package relocation.
# MinGW toolchains otherwise leave their C++ and unwind DLLs on the build host.
if(MINGW)
    target_link_options(control PRIVATE -static)
endif()

# Keep artifact admission independent of the generator's configuration layout.
set_target_properties(control PROPERTIES
    RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/bin")
foreach(configuration IN LISTS CMAKE_CONFIGURATION_TYPES)
    string(TOUPPER "${configuration}" configuration_upper)
    set_target_properties(control PROPERTIES
        "RUNTIME_OUTPUT_DIRECTORY_${configuration_upper}" "${CMAKE_CURRENT_BINARY_DIR}/bin")
endforeach()

# Preserve the bounded Release optimization level with each compiler's spelling.
if(MSVC)
    target_compile_options(control_model INTERFACE "$<$<CONFIG:Release>:/O2>")
elseif(CMAKE_CXX_COMPILER_ID MATCHES "^(GNU|Clang|AppleClang)$")
    target_compile_options(control_model INTERFACE "$<$<CONFIG:Release>:-O2>")
endif()

option(SANITIZERS "Instrument the complete control model and consumer" OFF)
if(SANITIZERS)
    if(MSVC OR NOT CMAKE_CXX_COMPILER_ID MATCHES "^(GNU|Clang|AppleClang)$")
        message(FATAL_ERROR "The complete ASan/UBSan control profile requires a GNU-compatible compiler")
    endif()
    target_compile_options(control_model INTERFACE -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(control_model INTERFACE -fsanitize=address,undefined)
endif()
