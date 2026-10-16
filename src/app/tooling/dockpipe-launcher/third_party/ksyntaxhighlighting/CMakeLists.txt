# Standalone, offline Qt 6 build of the MIT-licensed highlighting engine.
# Upstream source remains untouched. Syntax definitions ship as a separate data
# resource with their own notices/licenses, not linked into the executable.
set(archive "${CMAKE_CURRENT_SOURCE_DIR}/source-5.116.0.tar.xz")
file(SHA256 "${archive}" archive_hash)
if(NOT archive_hash STREQUAL "6f8ccacebd0dc9ce050b2b23b715750c97938b3336fbbb80967920cb8c1dfaa7")
    message(FATAL_ERROR "KSyntaxHighlighting source archive checksum mismatch")
endif()
# Upstream syntax fixtures contain deliberately invalid Go examples. Keep this
# generated C++ dependency outside the parent Go module scan.
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/go.mod" "module dockpipe.local/generated-syntax\n")
set(upstream "${CMAKE_CURRENT_BINARY_DIR}/syntax-highlighting-5.116.0")
if(NOT EXISTS "${upstream}/src/lib/repository.cpp")
    file(ARCHIVE_EXTRACT INPUT "${archive}" DESTINATION "${CMAKE_CURRENT_BINARY_DIR}")
endif()
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/ksyntaxhighlighting_export.h" "#pragma once\n#define KSYNTAXHIGHLIGHTING_EXPORT\n#define KSYNTAXHIGHLIGHTING_NO_EXPORT\n#define KSYNTAXHIGHLIGHTING_ENABLE_DEPRECATED_SINCE(a,b) 0\n#define KSYNTAXHIGHLIGHTING_BUILD_DEPRECATED_SINCE(a,b) 0\n")
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/ksyntaxhighlighting_version.h" "#pragma once\n#define SyntaxHighlighting_VERSION_MAJOR 5\n#define SyntaxHighlighting_VERSION_MINOR 116\n")
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/ksyntaxhighlighting_logging.h" "#pragma once\n#include <QLoggingCategory>\nnamespace KSyntaxHighlighting { Q_DECLARE_LOGGING_CATEGORY(Log) }\n")
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/logging.cpp" "#include \"ksyntaxhighlighting_logging.h\"\nnamespace KSyntaxHighlighting { Q_LOGGING_CATEGORY(Log, \"dockpipe.syntax\") }\n")
set(engine_sources
    abstracthighlighter context contextswitch highlightingdata foldingregion
    format keywordlist rule definition repository state syntaxhighlighter theme
    wildcardmatcher themedata worddelimiters)
set(engine_files)
foreach(source IN LISTS engine_sources)
    list(APPEND engine_files "${upstream}/src/lib/${source}.cpp")
endforeach()
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/theme_data.qrc"
    "<RCC><qresource prefix=\"/org.kde.syntax-highlighting/themes\"><file alias=\"breeze-dark.theme\">${upstream}/data/themes/breeze-dark.theme</file><file alias=\"breeze-light.theme\">${upstream}/data/themes/breeze-light.theme</file><file alias=\"dockpipe-dark.theme\">${CMAKE_CURRENT_SOURCE_DIR}/themes/dockpipe-dark.theme</file><file alias=\"dockpipe-light.theme\">${CMAKE_CURRENT_SOURCE_DIR}/themes/dockpipe-light.theme</file></qresource></RCC>")
add_library(dockpipe-syntax STATIC ${engine_files} "${CMAKE_CURRENT_BINARY_DIR}/logging.cpp" "${CMAKE_CURRENT_BINARY_DIR}/theme_data.qrc")
target_include_directories(dockpipe-syntax PUBLIC "${upstream}/src/lib" "${CMAKE_CURRENT_BINARY_DIR}")
target_compile_definitions(dockpipe-syntax PRIVATE NO_STANDARD_PATHS QT_USE_QSTRINGBUILDER)
target_link_libraries(dockpipe-syntax PUBLIC Qt6::Gui)

file(GLOB definitions "${upstream}/data/syntax/*.xml" "${CMAKE_CURRENT_SOURCE_DIR}/syntax/*.xml")
set(resource "<RCC><qresource prefix=\"/org.kde.syntax-highlighting/syntax-addons\">\n")
foreach(definition IN LISTS definitions)
    get_filename_component(name "${definition}" NAME)
    string(APPEND resource "<file alias=\"${name}\">${definition}</file>\n")
endforeach()
string(APPEND resource "</qresource></RCC>")
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/syntax.qrc" "${resource}")
set(DOCKPIPE_SYNTAX_RESOURCE "${CMAKE_BINARY_DIR}/dockpipe-syntax.rcc" CACHE INTERNAL "Bundled syntax data")
add_custom_command(OUTPUT "${DOCKPIPE_SYNTAX_RESOURCE}"
    COMMAND Qt6::rcc --binary --compress 9 "${CMAKE_CURRENT_BINARY_DIR}/syntax.qrc" -o "${DOCKPIPE_SYNTAX_RESOURCE}"
    DEPENDS ${definitions} "${CMAKE_CURRENT_BINARY_DIR}/syntax.qrc" VERBATIM)
add_custom_target(dockpipe-syntax-data ALL DEPENDS "${DOCKPIPE_SYNTAX_RESOURCE}")
add_dependencies(dockpipe-syntax dockpipe-syntax-data)
# Every desktop distribution copies this directory next to the resource file.
file(COPY "${upstream}/LICENSES" DESTINATION "${CMAKE_BINARY_DIR}/syntax-licenses")
configure_file("${CMAKE_CURRENT_SOURCE_DIR}/NOTICE.md" "${CMAKE_BINARY_DIR}/syntax-licenses/NOTICE.md" COPYONLY)
# Retain engine/theme copyright notices alongside the applicable license texts.
file(GLOB notice_sources "${upstream}/src/lib/*.cpp" "${upstream}/src/lib/*.h" "${upstream}/data/themes/breeze-*.theme" "${CMAKE_CURRENT_SOURCE_DIR}/themes/*.theme")
set(copyrights "KSyntaxHighlighting engine and Breeze theme notices\n\n")
foreach(source IN LISTS notice_sources)
    file(STRINGS "${source}" notices REGEX "SPDX-(FileCopyrightText|License-Identifier)|\\\"(copyright|license)\\\"")
    get_filename_component(name "${source}" NAME)
    string(APPEND copyrights "${name}\n")
    foreach(notice IN LISTS notices)
        string(APPEND copyrights "${notice}\n")
    endforeach()
    string(APPEND copyrights "\n")
endforeach()
file(WRITE "${CMAKE_BINARY_DIR}/syntax-licenses/COPYRIGHTS.txt" "${copyrights}")
configure_file("${CMAKE_CURRENT_SOURCE_DIR}/Apache-2.0.txt" "${CMAKE_BINARY_DIR}/syntax-licenses/LICENSES/Apache-2.0.txt" COPYONLY)
