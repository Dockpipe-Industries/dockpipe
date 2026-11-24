cmake_minimum_required(VERSION 3.21.1)
project(DockpipeQualificationNativeTool LANGUAGES CXX)

if(NOT CMAKE_SYSTEM_NAME STREQUAL "Linux" OR NOT IS_ABSOLUTE "${DOCKPIPE_PIPELANG_BUNDLE}")
    message(FATAL_ERROR "Select an absolute materialized Dockpipe SDK for the Linux native tool proof")
endif()
find_package(PipeLangModel 0.1.0 EXACT CONFIG REQUIRED)
find_package(Threads REQUIRED)

set(native "${DOCKPIPE_PIPELANG_BUNDLE}/native")
add_executable(dockpipe_native_tool_contracts Main.cpp
    "${native}/Client_posix.cpp" "${native}/ConfinedFiles_posix.cpp" "${native}/ScopeCodec.cpp")
target_include_directories(dockpipe_native_tool_contracts PRIVATE "${native}")
target_link_libraries(dockpipe_native_tool_contracts PRIVATE PipeLang::ModelRuntime Threads::Threads)
target_compile_features(dockpipe_native_tool_contracts PRIVATE cxx_std_20)
target_compile_options(dockpipe_native_tool_contracts PRIVATE -Wall -Wextra -Wpedantic -Werror)

option(SANITIZERS "Instrument the native tool caller and transport" OFF)
if(SANITIZERS)
    target_compile_options(dockpipe_native_tool_contracts PRIVATE
        -O1 -g -fsanitize=address,undefined -fno-omit-frame-pointer)
    target_link_options(dockpipe_native_tool_contracts PRIVATE -fsanitize=address,undefined)
endif()
