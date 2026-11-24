# Copyright Dockpipe contributors. SPDX-License-Identifier: Apache-2.0
include_guard(GLOBAL)

function(dockpipe_add_pipelang_bridge target)
    cmake_parse_arguments(PARSE_ARGV 1 bridge ""
        "MODEL_TARGET;MODEL_NAMESPACE;BRIDGE_NAMESPACE;OUTPUT_DIRECTORY" "")
    if(bridge_UNPARSED_ARGUMENTS OR bridge_KEYWORDS_MISSING_VALUES)
        message(FATAL_ERROR "Unexpected or incomplete Dockpipe SDK bridge arguments")
    endif()
    foreach(required MODEL_TARGET MODEL_NAMESPACE BRIDGE_NAMESPACE OUTPUT_DIRECTORY)
        if(NOT DEFINED bridge_${required} OR "${bridge_${required}}" STREQUAL "")
            message(FATAL_ERROR "Dockpipe SDK bridge requires ${required}")
        endif()
    endforeach()
    if(NOT TARGET "${bridge_MODEL_TARGET}" OR TARGET "${target}")
        message(FATAL_ERROR "Dockpipe SDK bridge requires an existing model and a new target")
    endif()
    foreach(namespace MODEL_NAMESPACE BRIDGE_NAMESPACE)
        if(NOT bridge_${namespace} MATCHES "^[A-Za-z_][A-Za-z0-9_]*(::[A-Za-z_][A-Za-z0-9_]*)*$")
            message(FATAL_ERROR "Dockpipe SDK bridge requires valid C++ namespaces")
        endif()
    endforeach()
    if(NOT IS_ABSOLUTE "${bridge_OUTPUT_DIRECTORY}")
        message(FATAL_ERROR "Dockpipe SDK bridge output must be absolute")
    endif()
    if(NOT CMAKE_SYSTEM_NAME STREQUAL "Linux" AND NOT WIN32)
        message(FATAL_ERROR "The Dockpipe SDK native transport requires a Linux or Windows headless host")
    endif()
    set(native "${CMAKE_CURRENT_FUNCTION_LIST_DIR}/../native")
    set(DOCKPIPE_MODEL_NAMESPACE "${bridge_MODEL_NAMESPACE}")
    set(DOCKPIPE_BRIDGE_NAMESPACE "${bridge_BRIDGE_NAMESPACE}")
    file(MAKE_DIRECTORY "${bridge_OUTPUT_DIRECTORY}")
    configure_file("${native}/Bridge.hpp.in"
        "${bridge_OUTPUT_DIRECTORY}/DockpipeSDKBridge.hpp" @ONLY)
    if(WIN32)
        add_library("${target}" STATIC "${native}/Client_windows.cpp"
            "${native}/WindowsFiles.cpp" "${native}/ScopeCodec.cpp")
        target_compile_definitions("${target}" PUBLIC _WIN32_WINNT=0x0A00 WINVER=0x0A00 NOMINMAX)
        set(signal_target "${target}_console_signal")
        if(TARGET "${signal_target}")
            message(FATAL_ERROR "Dockpipe SDK console signal target already exists")
        endif()
        add_executable("${signal_target}" "${native}/ConsoleSignal_windows.cpp"
            "${native}/ConsoleLaunch_windows.cpp")
        target_compile_features("${signal_target}" PRIVATE cxx_std_20)
        target_compile_definitions("${signal_target}" PRIVATE _WIN32_WINNT=0x0A00 WINVER=0x0A00 NOMINMAX)
        if(MINGW)
            target_link_options("${signal_target}" PRIVATE -municode)
        endif()
        add_dependencies("${target}" "${signal_target}")
        # Runtime selection remains explicit. Consumers stage this target's
        # artifact and pass its absolute path in ClientConfiguration; no build
        # directory or compiler is silently selected at runtime.
        set_property(TARGET "${target}" PROPERTY DOCKPIPE_CONSOLE_SIGNAL_TARGET "${signal_target}")
    else()
        add_library("${target}" STATIC "${native}/Client_posix.cpp"
            "${native}/ConfinedFiles_posix.cpp" "${native}/ScopeCodec.cpp")
    endif()
    target_compile_features("${target}" PUBLIC cxx_std_20)
    target_include_directories("${target}" PUBLIC "${native}" "${bridge_OUTPUT_DIRECTORY}")
    # Follow the selected model's exact compiler/resource/instrumentation/runtime
    # mode policy; do not create an independent SDK runtime ownership domain.
    target_link_libraries("${target}" PUBLIC "${bridge_MODEL_TARGET}")
endfunction()
