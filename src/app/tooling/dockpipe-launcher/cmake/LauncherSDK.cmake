# This is a host integration SDK, not the neutral Spatial-Web browser runtime.
option(DOCKPIPE_INSTALL_LAUNCHER_SDK "Install the matching native host UI SDK" OFF)
if(NOT DOCKPIPE_INSTALL_LAUNCHER_SDK)
    return()
endif()

if(NOT Qt6_VERSION VERSION_EQUAL "6.10.3")
    message(FATAL_ERROR "The launcher SDK requires Qt 6.10.3 exactly")
endif()

include(GNUInstallDirs)
include(CMakePackageConfigHelpers)
set(DOCKPIPE_LAUNCHER_SDK_VERSION "1.1.0")
set(sdk_config_directory "${CMAKE_INSTALL_LIBDIR}/cmake/DockPipeLauncherSDK")
set(sdk_resource_directory "${CMAKE_INSTALL_DATADIR}/DockPipeLauncherSDK")

add_library(dockpipe-launcher-headers INTERFACE)
set_target_properties(dockpipe-launcher-headers PROPERTIES EXPORT_NAME LauncherHeaders)
target_include_directories(dockpipe-launcher-headers INTERFACE
    "$<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/src>"
    "$<INSTALL_INTERFACE:include/dockpipe-launcher>"
    "$<INSTALL_INTERFACE:include>")
target_compile_features(dockpipe-launcher-headers INTERFACE cxx_std_17)
set_target_properties(dockpipe-launcher-ui PROPERTIES EXPORT_NAME LauncherUI)
set_target_properties(dockpipe-browser PROPERTIES EXPORT_NAME HostBrowser)
target_compile_features(dockpipe-launcher-ui PUBLIC cxx_std_17)
target_compile_features(dockpipe-browser PUBLIC cxx_std_17)
install(TARGETS dockpipe-workspace-reader RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR} COMPONENT LauncherSDK)
install(TARGETS dockpipe-launcher-ui dockpipe-browser dockpipe-launcher-headers
    EXPORT DockPipeLauncherSDKTargets
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR} COMPONENT LauncherSDK
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR} COMPONENT LauncherSDK
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR} COMPONENT LauncherSDK)

# Public headers used by Cockpit, followed by their launcher-owned include closure.
set(sdk_headers
    BrowserRuntime.h BrowserWidget.h CanvasIcons.h CanvasView.h ContextStore.h
    DesktopAppWindow.h DesktopApplicationCatalog.h LauncherWorkspacePackage.h LauncherDisplayPackage.h LauncherNativeLibraryScope.h
    ProjectFilesWidget.h ProjectGitClient.h SourceEditor.h SourceHighlighter.h
    Theme.h WorkspaceFiles.h WorkspacePackageManager.h
    WorkspaceGraphPage.h WorkspaceGraphReader.h WorkspaceGraphNode.h WorkspaceGraphView.h WorkspaceGraphWidget.h
    LauncherUiExport.h CockpitSettings.h Context.h RemoteCommand.h
    LauncherSettings.h PackageCommand.h)
foreach(header IN LISTS sdk_headers)
    install(FILES "${CMAKE_CURRENT_SOURCE_DIR}/src/${header}"
        DESTINATION include/dockpipe-launcher COMPONENT LauncherSDK)
endforeach()

# SourceHighlighter exposes the KSyntaxHighlighting base type in its public API.
# Install headers, not its archive: the implementation is already in LauncherUI.
set(syntax_build "${CMAKE_CURRENT_BINARY_DIR}/third_party/ksyntaxhighlighting")
install(DIRECTORY "${syntax_build}/syntax-highlighting-5.116.0/src/lib/"
    DESTINATION include/dockpipe-launcher/syntax COMPONENT LauncherSDK FILES_MATCHING PATTERN "*.h")
install(FILES "${syntax_build}/ksyntaxhighlighting_export.h"
    "${syntax_build}/ksyntaxhighlighting_version.h"
    DESTINATION include/dockpipe-launcher/syntax COMPONENT LauncherSDK)
file(WRITE "${CMAKE_CURRENT_BINARY_DIR}/generated/SDKTheme" "#include \"../theme.h\"\n")
install(FILES "${CMAKE_CURRENT_BINARY_DIR}/generated/SDKTheme"
    DESTINATION include/dockpipe-launcher/syntax/KSyntaxHighlighting RENAME Theme COMPONENT LauncherSDK)

install(FILES "${CMAKE_CURRENT_SOURCE_DIR}/dockpipe-launcher.qrc"
    DESTINATION ${sdk_resource_directory} COMPONENT LauncherSDK)
install(DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}/resources/images"
    "${CMAKE_CURRENT_SOURCE_DIR}/resources/icons"
    "${CMAKE_CURRENT_SOURCE_DIR}/resources/theme"
    DESTINATION ${sdk_resource_directory}/resources COMPONENT LauncherSDK)
install(FILES "${DOCKPIPE_SYNTAX_RESOURCE}" DESTINATION ${sdk_resource_directory} COMPONENT LauncherSDK)
install(DIRECTORY "${CMAKE_BINARY_DIR}/syntax-licenses"
    DESTINATION ${sdk_resource_directory} COMPONENT LauncherSDK)

# External native package builders use the SDK's audited tooling and notices.
get_filename_component(dockpipe_repository_root "${CMAKE_CURRENT_SOURCE_DIR}/../../../.." ABSOLUTE)
install(FILES "${dockpipe_repository_root}/release/packaging/lib/elf_artifacts.py"
    DESTINATION ${sdk_resource_directory}/tools COMPONENT LauncherSDK)
install(FILES "${dockpipe_repository_root}/LICENSE"
    DESTINATION ${sdk_resource_directory}/licenses RENAME Dockpipe-LICENSE COMPONENT LauncherSDK)
install(FILES "${dockpipe_repository_root}/release/packaging/desktop/licenses/Qt-LGPL-3.txt"
    "${dockpipe_repository_root}/release/packaging/desktop/licenses/Qt-GPL-3.txt"
    DESTINATION ${sdk_resource_directory}/licenses COMPONENT LauncherSDK)
configure_file("${CMAKE_CURRENT_SOURCE_DIR}/cmake/LauncherSDKLayout.json.in"
    "${CMAKE_CURRENT_BINARY_DIR}/sdk-layout.json" @ONLY)
install(FILES "${CMAKE_CURRENT_BINARY_DIR}/sdk-layout.json" DESTINATION ${sdk_resource_directory} COMPONENT LauncherSDK)

configure_package_config_file(
    "${CMAKE_CURRENT_SOURCE_DIR}/cmake/DockPipeLauncherSDKConfig.cmake.in"
    "${CMAKE_CURRENT_BINARY_DIR}/DockPipeLauncherSDKConfig.cmake"
    INSTALL_DESTINATION ${sdk_config_directory}
    PATH_VARS sdk_resource_directory CMAKE_INSTALL_LIBDIR)
write_basic_package_version_file(
    "${CMAKE_CURRENT_BINARY_DIR}/DockPipeLauncherSDKConfigVersion.cmake"
    VERSION ${DOCKPIPE_LAUNCHER_SDK_VERSION} COMPATIBILITY ExactVersion)
install(FILES "${CMAKE_CURRENT_BINARY_DIR}/DockPipeLauncherSDKConfig.cmake"
    "${CMAKE_CURRENT_BINARY_DIR}/DockPipeLauncherSDKConfigVersion.cmake"
    DESTINATION ${sdk_config_directory} COMPONENT LauncherSDK)
install(EXPORT DockPipeLauncherSDKTargets NAMESPACE DockPipe::
    DESTINATION ${sdk_config_directory} COMPONENT LauncherSDK)
