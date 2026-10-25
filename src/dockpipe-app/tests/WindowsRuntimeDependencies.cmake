# Diagnostic import scan only. It neither deploys DLLs nor changes the test environment.
cmake_minimum_required(VERSION 3.21)

if(NOT DEFINED TEST_EXECUTABLE OR NOT EXISTS "${TEST_EXECUTABLE}")
    message(FATAL_ERROR "TEST_EXECUTABLE must name the built embedding fixture")
endif()
if(NOT DEFINED DUMPBIN OR NOT EXISTS "${DUMPBIN}")
    message(FATAL_ERROR "DUMPBIN must name the active Visual Studio import inspector")
endif()

set(CMAKE_GET_RUNTIME_DEPENDENCIES_PLATFORM "windows+pe")
set(CMAKE_GET_RUNTIME_DEPENDENCIES_TOOL "dumpbin")
set(CMAKE_GET_RUNTIME_DEPENDENCIES_COMMAND "${DUMPBIN}")
get_filename_component(test_directory "${TEST_EXECUTABLE}" DIRECTORY)
# CTest runs in the build directory. CMake's PE resolver searches the executable
# directory and Windows system directories, but requires PATH entries explicitly.
file(TO_CMAKE_PATH "$ENV{PATH}" runtime_directories)
list(PREPEND runtime_directories "${test_directory}")
message(STATUS "Windows loader scan: executable=${TEST_EXECUTABLE}")
message(STATUS "Windows loader scan: expected CTest working directory=${test_directory}")
message(STATUS "Windows loader scan: effective PATH=$ENV{PATH}")
message(STATUS "Windows loader scan: dumpbin=${DUMPBIN}")

file(GET_RUNTIME_DEPENDENCIES
    EXECUTABLES "${TEST_EXECUTABLE}"
    DIRECTORIES ${runtime_directories}
    RESOLVED_DEPENDENCIES_VAR resolved_dependencies
    UNRESOLVED_DEPENDENCIES_VAR unresolved_dependencies
    CONFLICTING_DEPENDENCIES_PREFIX conflicting_dependencies
    # Windows resolves API-set contracts; they are not ordinary files on PATH.
    PRE_EXCLUDE_REGEXES "^api-ms-win-" "^ext-ms-win-")

# Bind no-load access probes in the verified restricted child to this exact graph.
string(JOIN "\n" manifest "${TEST_EXECUTABLE}" ${resolved_dependencies})
file(WRITE "${test_directory}/windows-runtime-dependencies.txt" "${manifest}\n")

foreach(dependency IN LISTS resolved_dependencies)
    message(STATUS "Windows loader scan: resolved=${dependency}")
endforeach()
foreach(dependency IN LISTS unresolved_dependencies)
    message(STATUS "Windows loader scan: unresolved=${dependency}")
endforeach()
foreach(dependency IN LISTS conflicting_dependencies_FILENAMES)
    message(STATUS "Windows loader scan: conflict=${dependency}: ${conflicting_dependencies_${dependency}}")
endforeach()
list(LENGTH resolved_dependencies resolved_count)
list(LENGTH unresolved_dependencies unresolved_count)
list(LENGTH conflicting_dependencies_FILENAMES conflict_count)
message(STATUS "Windows loader scan: resolved=${resolved_count}, unresolved=${unresolved_count}, conflicts=${conflict_count}")
# Static resolution is diagnostic evidence, not restricted-token loader proof.
# The original native CTest remains the admission gate even if this scan is clean.
