cmake_minimum_required(VERSION 3.21.1)
project(DockpipeQualificationInputCustodyContracts LANGUAGES CXX)

find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED)
find_package(PipeLangFileStreams 0.1.0 EXACT CONFIG REQUIRED)
foreach(required IN ITEMS PIPELANG_JSON_SHA256 PIPELANG_TESTING_SHA256 PIPELANG_TESTING_LIBRARY_ROOT)
    if("${${required}}" STREQUAL "")
        message(FATAL_ERROR "${required} must be supplied explicitly")
    endif()
endforeach()

function(json_path output input)
    string(REPLACE "\\" "\\\\" escaped "${input}")
    string(REPLACE "\"" "\\\"" escaped "${escaped}")
    string(REPLACE "\n" "\\n" escaped "${escaped}")
    string(REPLACE "\r" "\\r" escaped "${escaped}")
    string(REPLACE "\t" "\\t" escaped "${escaped}")
    set("${output}" "${escaped}" PARENT_SCOPE)
endfunction()
file(REAL_PATH "${PipeLangModel_SOURCE_LIBRARY_DIRECTORY}/json" json_root)
if(NOT IS_ABSOLUTE "${PIPELANG_TESTING_LIBRARY_ROOT}")
    message(FATAL_ERROR "Select an absolute immutable Testing source artifact")
endif()
file(REAL_PATH "${PIPELANG_TESTING_LIBRARY_ROOT}" testing_root)
file(SHA256 "${json_root}/library.json" json_sha256)
file(SHA256 "${testing_root}/library.json" testing_sha256)
if(NOT json_sha256 STREQUAL PIPELANG_JSON_SHA256 OR
        NOT testing_sha256 STREQUAL PIPELANG_TESTING_SHA256)
    message(FATAL_ERROR "Selected source library descriptors differ from the supplied identities")
endif()
json_path(json_root_json "${json_root}")
json_path(testing_root_json "${testing_root}")
json_path(crypto_root_json "${PipeLangCryptoStream_SOURCE_LIBRARY_DIRECTORY}")
json_path(file_root_json "${PipeLangFileStreams_SOURCE_LIBRARY_DIRECTORY}")
set(source_references "${CMAKE_CURRENT_BINARY_DIR}/evidence-libraries.json")
file(WRITE "${source_references}"
    "{\"protocol\":\"pipelang.source-library-references.v1\",\"libraries\":[")
file(APPEND "${source_references}"
    "{\"root\":\"${json_root_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PIPELANG_JSON_SHA256}\"},"
    "{\"root\":\"${testing_root_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PIPELANG_TESTING_SHA256}\"},"
    "{\"root\":\"${crypto_root_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PipeLangCryptoStream_SOURCE_LIBRARY_SHA256}\"},"
    "{\"root\":\"${file_root_json}\",\"descriptor\":\"library.json\",\"sha256\":\"${PipeLangFileStreams_SOURCE_LIBRARY_SHA256}\"}]}\n")

pipelang_add_model(qualification_input_custody_model
    PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../harness"
    SOURCE_ROOT src ENTRY qualification-output/InputCustodyEntry.pipe
    NAMESPACE qualification_input_custody_proof SOURCE_LIBRARIES "${source_references}"
    COMPILATION COMPILED OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/generated")

option(SANITIZERS "Instrument generated Source units and the actual JSON reader" OFF)
set(evidence_reader PipeLang::JsonReader)
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
    add_library(evidence_instrumented_reader STATIC "${json_sources}/Reader.cpp")
    get_target_property(json_includes PipeLang::JsonReader INTERFACE_INCLUDE_DIRECTORIES)
    target_include_directories(evidence_instrumented_reader PUBLIC "${json_includes}"
        PRIVATE "${json_sources}/third_party")
    target_compile_features(evidence_instrumented_reader PUBLIC cxx_std_20)
    set_target_properties(evidence_instrumented_reader PROPERTIES POSITION_INDEPENDENT_CODE ON)
    target_compile_options(evidence_instrumented_reader PRIVATE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(evidence_instrumented_reader INTERFACE -fsanitize=address,undefined)
    _pipelang_model_resolve_cxx_resource_profile(resource_profile resource_response resource_digest resource_dependency)
    _pipelang_model_apply_cxx_resource_profile(evidence_instrumented_reader
        "${resource_profile}" "${resource_response}" "${resource_digest}")
    if(resource_response)
        target_compile_options(evidence_instrumented_reader PRIVATE "$<$<COMPILE_LANGUAGE:CXX>:@${resource_response}>")
    endif()
    target_compile_options(qualification_input_custody_model INTERFACE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(qualification_input_custody_model INTERFACE -fsanitize=address,undefined)
    set(evidence_reader evidence_instrumented_reader)
endif()

add_executable(qualification_input_custody_probe Main.cpp)
target_link_libraries(qualification_input_custody_probe PRIVATE qualification_input_custody_model "${evidence_reader}" PipeLang::FileStreams)
target_compile_features(qualification_input_custody_probe PRIVATE cxx_std_20)
set_target_properties(qualification_input_custody_probe PROPERTIES CXX_EXTENSIONS OFF)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(qualification_input_custody_probe PRIVATE -Wall -Wextra -Wpedantic -Werror)
endif()
