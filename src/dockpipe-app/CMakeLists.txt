cmake_minimum_required(VERSION 3.21.1)
project(dockpipe-app VERSION 0.1.0 LANGUAGES CXX)
# Interactive rendering needs optimization even in a plain local CMake build.
# Preserve explicitly selected Debug builds and multi-configuration generators.
if(NOT CMAKE_CONFIGURATION_TYPES AND NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release CACHE STRING "Build type" FORCE)
endif()
option(DOCKPIPE_ENABLE_QUALIFICATION "Build explicit installed desktop qualification entrypoint" OFF)
set(DOCKPIPE_RELEASE_VERSION "" CACHE STRING "Explicit release version; empty stamps a development build")

# Generate at build time so an incremental build sees commit and dirty-state changes.
set(DOCKPIPE_VERSION_HEADER "${CMAKE_CURRENT_BINARY_DIR}/generated/BuildVersion.h")
add_custom_target(dockpipe-build-version
    COMMAND "${CMAKE_COMMAND}"
        "-DSOURCE_DIR=${CMAKE_CURRENT_SOURCE_DIR}"
        "-DOUTPUT_FILE=${DOCKPIPE_VERSION_HEADER}"
        "-DRELEASE_VERSION=${DOCKPIPE_RELEASE_VERSION}"
        -P "${CMAKE_CURRENT_SOURCE_DIR}/cmake/BuildVersion.cmake"
    BYPRODUCTS "${DOCKPIPE_VERSION_HEADER}"
    VERBATIM)

set(DOCKPIPE_BUNDLE_VERSION "${DOCKPIPE_RELEASE_VERSION}")
if(NOT DOCKPIPE_BUNDLE_VERSION)
    set(DOCKPIPE_BUNDLE_VERSION "${PROJECT_VERSION}")
endif()

set(CMAKE_POSITION_INDEPENDENT_CODE ON)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

include(cmake/AppSourceLayout.cmake)
include_directories(${DOCKPIPE_APP_INCLUDE_DIRECTORIES})

# Qt6Gui depends on WrapOpenGL; without Mesa dev packages, find_package(Qt6) fails obscurely on Linux.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(OpenGL REQUIRED)
endif()

find_package(Qt6 REQUIRED COMPONENTS Widgets Network Concurrent WebEngineWidgets PrintSupport WebChannel OpenGLWidgets)
add_subdirectory(third_party/ksyntaxhighlighting)

if(APPLE)
    enable_language(OBJCXX)
    set_source_files_properties(src/native-desktop/infrastructure/DesktopPermissionAccessMac.mm PROPERTIES COMPILE_OPTIONS "-fobjc-arc")
endif()

# Keep every permission widget consumer paired with its platform factory.
function(dockpipe_target_desktop_permissions target)
    target_sources(${target} PRIVATE src/native-desktop/presentation/DesktopPermissionsWidget.cpp src/native-desktop/infrastructure/DesktopPermissionAccess.cpp)
    if(APPLE)
        target_sources(${target} PRIVATE src/native-desktop/infrastructure/DesktopPermissionAccessMac.mm)
        set_target_properties(${target} PROPERTIES OBJCXX_STANDARD 17 OBJCXX_STANDARD_REQUIRED ON)
        target_link_libraries(${target} PRIVATE "-framework ApplicationServices" "-framework CoreGraphics")
    endif()
endfunction()

set(DOCKPIPE_APP_SOURCES
    src/bootstrap/composition/main.cpp
    src/shell/infrastructure/LauncherEnvironment.cpp
    src/shell/infrastructure/LauncherDiagnostics.cpp
    src/shell/infrastructure/FlatpakTerminal.cpp
    src/shell/infrastructure/SingleInstanceGuard.cpp
    src/shared-ui/presentation/Theme.cpp
    src/shared-ui/presentation/WindowChrome.cpp
    src/execution/domain/Context.cpp
    src/execution/infrastructure/ContextStore.cpp
    src/git/infrastructure/GitHelper.cpp
    src/execution/infrastructure/SessionManager.cpp
    src/catalog/infrastructure/DockpipeChoices.cpp src/catalog/infrastructure/OperatorDockpipeBinary.cpp
    src/execution/application/ContextDiscovery.cpp
    src/catalog/infrastructure/WorkflowCatalog.cpp src/catalog/domain/WorkflowCatalogData.cpp
    src/shell/infrastructure/LauncherSettings.cpp
    src/catalog/presentation/AppCardWidget.cpp
    src/catalog/presentation/BasicModeWidget.cpp
    src/packages/infrastructure/PackageCommand.cpp
    src/packages/presentation/PackageManagerDialog.cpp
    src/shell/presentation/SettingsDialog.cpp
    src/shell/presentation/LogViewerDialog.cpp
    src/shell/presentation/PromptDialog.cpp
    src/catalog/presentation/WorkflowLaunchDialog.cpp
    src/shared-ui/presentation/CanvasIcons.cpp
    src/shared-ui/presentation/CanvasPanel.cpp
    src/workflows/presentation/WorkflowCanvas.cpp
    src/workflows/presentation/WorkflowEditorWidget.cpp
    src/workflows/application/WorkflowScriptPath.cpp
    src/files/presentation/ProjectFilesWidget.cpp src/files/presentation/FileExplorerView.cpp src/files/presentation/FileExplorerIcons.cpp src/files/presentation/MarkdownPreview.cpp src/files/presentation/MermaidRenderer.cpp
    src/git/presentation/ProjectGitWidget.cpp
    src/git/infrastructure/ProjectGitClient.cpp
    src/files/presentation/SourceEditor.cpp
    src/files/presentation/SourceHighlighter.cpp
    src/observability/presentation/DockerObservabilityWidget.cpp
    src/observability/presentation/DockerRemoteMonitoring.cpp
    src/observability/presentation/DockerResourceInspector.cpp
    src/observability/presentation/ResourceCanvas.cpp
    src/observability/presentation/ResourceLinkItem.cpp
    src/execution/presentation/ContextRowWidget.cpp
    src/execution/presentation/EditContextDialog.cpp
    src/shell/infrastructure/SetupTerminal.cpp
    src/remote/infrastructure/RemoteCommand.cpp
    src/remote/presentation/RemoteWidget.cpp
    src/remote/presentation/HardwareKeyDialog.cpp
    src/execution/presentation/RemoteRunDialog.cpp
    src/execution/presentation/ActivityWidget.cpp
    src/observability/presentation/ExecutionExplorer.cpp
    src/observability/presentation/ExecutionMetricChart.cpp
    src/observability/presentation/SystemResourceOverview.cpp
    src/shell/presentation/MainWindow.cpp
    src/shell/presentation/MainWindowWorkflows.cpp
    src/shell/presentation/MainWindowDashboards.cpp
    src/shell/presentation/MainWindowSurfaces.cpp
    src/packages/presentation/PackageSurfacePage.cpp
    src/dashboards/presentation/DashboardPage.cpp
    src/dashboards/presentation/DashboardSettingsDialog.cpp
    src/dashboards/presentation/DashboardTourDialog.cpp
    src/dashboards/presentation/DashboardAppsDialog.cpp
    src/dashboards/presentation/DashboardWidgetFactory.cpp
    src/dashboards/presentation/DashboardWidgetViews.cpp
    src/dashboards/presentation/DashboardLogWidget.cpp
    dockpipe-launcher.qrc
)

if(WIN32)
    list(APPEND DOCKPIPE_APP_SOURCES resources/windows/dockpipe-launcher.rc)
    qt_add_executable(dockpipe-launcher WIN32 ${DOCKPIPE_APP_SOURCES})
else()
    qt_add_executable(dockpipe-launcher ${DOCKPIPE_APP_SOURCES})
endif()

dockpipe_target_desktop_permissions(dockpipe-launcher)

if(APPLE)
    set_target_properties(dockpipe-launcher PROPERTIES
        MACOSX_BUNDLE TRUE
        OUTPUT_NAME DockPipe
        MACOSX_BUNDLE_GUI_IDENTIFIER com.dockpipe.launcher
        MACOSX_BUNDLE_BUNDLE_NAME Dockpipe
        MACOSX_BUNDLE_BUNDLE_VERSION "${DOCKPIPE_BUNDLE_VERSION}"
        MACOSX_BUNDLE_SHORT_VERSION_STRING "${DOCKPIPE_BUNDLE_VERSION}"
        MACOSX_BUNDLE_ICON_FILE DockPipe.icns
    )
endif()

if(DOCKPIPE_ENABLE_QUALIFICATION)
    target_sources(dockpipe-launcher PRIVATE src/qualification/application/PackageDisplayQualification.cpp src/qualification/application/QualificationWorkflowLaunch.cpp)
    target_compile_definitions(dockpipe-launcher PRIVATE DOCKPIPE_ENABLE_QUALIFICATION)
endif()

target_include_directories(dockpipe-launcher PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
add_dependencies(dockpipe-launcher dockpipe-build-version)
target_include_directories(dockpipe-launcher PRIVATE "${CMAKE_CURRENT_BINARY_DIR}/generated")
target_link_libraries(dockpipe-launcher PRIVATE Qt6::Widgets Qt6::Network Qt6::Concurrent)

if(MSVC)
    target_compile_definitions(dockpipe-launcher PRIVATE _CRT_SECURE_NO_WARNINGS)
endif()

include(CTest)
if(BUILD_TESTING)
    add_test(NAME launcher-version COMMAND dockpipe-launcher --version)
    if(DOCKPIPE_RELEASE_VERSION)
        set_tests_properties(launcher-version PROPERTIES PASS_REGULAR_EXPRESSION "${DOCKPIPE_RELEASE_VERSION}")
    else()
        set_tests_properties(launcher-version PROPERTIES PASS_REGULAR_EXPRESSION "^dev[+]")
    endif()
    add_test(NAME launcher-version-generation COMMAND "${CMAKE_COMMAND}"
        "-DVERSION_SCRIPT=${CMAKE_CURRENT_SOURCE_DIR}/cmake/BuildVersion.cmake"
        "-DTEST_OUTPUT=${CMAKE_CURRENT_BINARY_DIR}/version-test"
        -P "${CMAKE_CURRENT_SOURCE_DIR}/tests/BuildVersionTest.cmake")
endif()
if(BUILD_TESTING AND UNIX)
    add_executable(launcher-environment-test
        tests/LauncherEnvironmentTest.cpp
        src/shell/infrastructure/LauncherEnvironment.cpp
    )
    target_include_directories(launcher-environment-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(launcher-environment-test PRIVATE Qt6::Core)
    add_test(NAME launcher-environment COMMAND launcher-environment-test)
    add_executable(package-command-test tests/PackageCommandTest.cpp src/packages/infrastructure/PackageCommand.cpp)
    target_include_directories(package-command-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(package-command-test PRIVATE Qt6::Core)
    add_test(NAME package-command COMMAND package-command-test)
    add_executable(global-root-test tests/GlobalRootTest.cpp src/shell/infrastructure/LauncherEnvironment.cpp)
    target_include_directories(global-root-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(global-root-test PRIVATE Qt6::Core)
    add_test(NAME global-root-default COMMAND global-root-test)
    add_test(NAME global-root-inherited COMMAND global-root-test --inherited)
    add_executable(package-manager-test
        tests/PackageManagerTest.cpp src/packages/presentation/PackageManagerDialog.cpp src/packages/infrastructure/PackageCommand.cpp
        src/catalog/infrastructure/DockpipeChoices.cpp src/catalog/infrastructure/OperatorDockpipeBinary.cpp src/catalog/infrastructure/WorkflowCatalog.cpp src/catalog/domain/WorkflowCatalogData.cpp src/shell/infrastructure/LauncherSettings.cpp src/execution/infrastructure/ContextStore.cpp src/execution/domain/Context.cpp
        src/execution/infrastructure/ContextStore.cpp src/execution/domain/Context.cpp)
    target_include_directories(package-manager-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(package-manager-test PRIVATE Qt6::Widgets)
    add_test(NAME package-manager COMMAND package-manager-test)
    set_tests_properties(package-manager PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
        add_executable(flatpak-terminal-test
            tests/FlatpakTerminalTest.cpp src/shell/infrastructure/FlatpakTerminal.cpp)
        target_include_directories(flatpak-terminal-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
        target_link_libraries(flatpak-terminal-test PRIVATE Qt6::Core)
        add_dependencies(flatpak-terminal-test dockpipe-launcher)
        add_test(NAME flatpak-terminal COMMAND flatpak-terminal-test $<TARGET_FILE:dockpipe-launcher>)
        set_tests_properties(flatpak-terminal PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    endif()
    add_executable(launcher-diagnostics-test tests/LauncherDiagnosticsTest.cpp src/shell/infrastructure/LauncherDiagnostics.cpp)
    target_include_directories(launcher-diagnostics-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(launcher-diagnostics-test PRIVATE Qt6::Core)
    add_test(NAME launcher-diagnostics COMMAND launcher-diagnostics-test)
    add_test(NAME launcher-diagnostics-opt-in COMMAND launcher-diagnostics-test --diagnostics)
    add_test(NAME launcher-diagnostics-native COMMAND launcher-diagnostics-test --native)
endif()

if(BUILD_TESTING AND UNIX)
    set(REMOTE_LAUNCHER_TEST_SOURCES ${DOCKPIPE_APP_SOURCES})
    list(REMOVE_ITEM REMOTE_LAUNCHER_TEST_SOURCES src/bootstrap/composition/main.cpp)
    set(REMOTE_LAUNCHER_UI_TEST_SOURCES ${REMOTE_LAUNCHER_TEST_SOURCES})
    if(APPLE)
        # macOS uses an absolute osascript path, so a PATH fixture cannot isolate
        # Terminal.app. Keep UI command assertions without opening a real app.
        list(REMOVE_ITEM REMOTE_LAUNCHER_UI_TEST_SOURCES src/shell/infrastructure/SetupTerminal.cpp)
        list(APPEND REMOTE_LAUNCHER_UI_TEST_SOURCES tests/SetupTerminalFixture.cpp)
    endif()
    add_executable(remote-launcher-test tests/RemoteLauncherTest.cpp tests/SystemMonitoringTest.cpp ${REMOTE_LAUNCHER_UI_TEST_SOURCES})
    target_include_directories(remote-launcher-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(remote-launcher-test PRIVATE Qt6::Widgets Qt6::Network Qt6::Concurrent)
    dockpipe_target_desktop_permissions(remote-launcher-test)
    add_executable(workflow-workspace-test tests/WorkflowWorkspaceTest.cpp ${REMOTE_LAUNCHER_TEST_SOURCES})
    target_include_directories(workflow-workspace-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(workflow-workspace-test PRIVATE Qt6::Widgets Qt6::Network Qt6::Concurrent)
    dockpipe_target_desktop_permissions(workflow-workspace-test)
    add_test(NAME workflow-workspace COMMAND workflow-workspace-test)
    add_test(NAME workflow-surface-navigation COMMAND workflow-workspace-test --surfaces)
    set_tests_properties(workflow-surface-navigation PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
    set_tests_properties(workflow-workspace PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
    add_test(NAME remote-launcher COMMAND remote-launcher-test)
    set_tests_properties(remote-launcher PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(workflow-studio-test
        tests/WorkflowStudioTest.cpp src/shared-ui/presentation/CanvasIcons.cpp src/shared-ui/presentation/CanvasPanel.cpp src/workflows/presentation/WorkflowCanvas.cpp src/workflows/presentation/WorkflowEditorWidget.cpp src/workflows/application/WorkflowScriptPath.cpp src/files/presentation/ProjectFilesWidget.cpp src/files/presentation/FileExplorerView.cpp src/files/presentation/FileExplorerIcons.cpp src/files/presentation/MarkdownPreview.cpp src/files/presentation/MermaidRenderer.cpp src/files/presentation/SourceEditor.cpp src/files/presentation/SourceHighlighter.cpp
        src/shared-ui/presentation/Theme.cpp src/shared-ui/presentation/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(workflow-studio-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(workflow-studio-test PRIVATE Qt6::Widgets)
    add_test(NAME workflow-studio COMMAND workflow-studio-test)
    set_tests_properties(workflow-studio PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
    add_executable(workflow-script-path-test tests/WorkflowScriptPathTest.cpp src/workflows/application/WorkflowScriptPath.cpp)
    target_include_directories(workflow-script-path-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(workflow-script-path-test PRIVATE Qt6::Core)
    add_test(NAME workflow-script-path COMMAND workflow-script-path-test)
    add_executable(project-files-test tests/ProjectFilesTest.cpp src/files/presentation/ProjectFilesWidget.cpp src/files/presentation/FileExplorerView.cpp src/files/presentation/FileExplorerIcons.cpp src/files/presentation/MarkdownPreview.cpp src/files/presentation/MermaidRenderer.cpp src/shared-ui/presentation/CanvasIcons.cpp src/files/presentation/SourceEditor.cpp src/files/presentation/SourceHighlighter.cpp
        src/shared-ui/presentation/Theme.cpp src/shared-ui/presentation/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(project-files-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(project-files-test PRIVATE Qt6::Widgets)
    add_test(NAME project-files COMMAND project-files-test)
    set_tests_properties(project-files PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(project-git-test tests/ProjectGitTest.cpp src/git/presentation/ProjectGitWidget.cpp src/git/infrastructure/ProjectGitClient.cpp src/files/presentation/SourceEditor.cpp src/files/presentation/SourceHighlighter.cpp
        src/remote/infrastructure/RemoteCommand.cpp src/catalog/infrastructure/DockpipeChoices.cpp src/catalog/infrastructure/OperatorDockpipeBinary.cpp src/catalog/infrastructure/WorkflowCatalog.cpp src/catalog/domain/WorkflowCatalogData.cpp src/shell/infrastructure/LauncherSettings.cpp src/execution/infrastructure/ContextStore.cpp src/execution/domain/Context.cpp
        src/shared-ui/presentation/CanvasIcons.cpp src/shared-ui/presentation/Theme.cpp src/shared-ui/presentation/WindowChrome.cpp src/shared-ui/presentation/WindowControlButton.cpp dockpipe-launcher.qrc)
    target_include_directories(project-git-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(project-git-test PRIVATE Qt6::Widgets)
    add_test(NAME project-git COMMAND project-git-test)
    set_tests_properties(project-git PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45 SKIP_RETURN_CODE 77)
endif()

if(BUILD_TESTING)
    add_executable(execution-explorer-test
        tests/ExecutionExplorerTest.cpp src/observability/presentation/ExecutionExplorer.cpp src/observability/presentation/ExecutionMetricChart.cpp src/observability/presentation/SystemResourceOverview.cpp
        src/remote/infrastructure/RemoteCommand.cpp src/catalog/infrastructure/DockpipeChoices.cpp src/catalog/infrastructure/OperatorDockpipeBinary.cpp src/catalog/infrastructure/WorkflowCatalog.cpp src/catalog/domain/WorkflowCatalogData.cpp src/shell/infrastructure/LauncherSettings.cpp src/execution/infrastructure/ContextStore.cpp src/execution/domain/Context.cpp
        src/observability/presentation/ResourceCanvas.cpp src/observability/presentation/ResourceLinkItem.cpp src/shared-ui/presentation/CanvasPanel.cpp src/shared-ui/presentation/CanvasIcons.cpp
        src/shared-ui/presentation/Theme.cpp src/shared-ui/presentation/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(execution-explorer-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(execution-explorer-test PRIVATE Qt6::Widgets)
    add_test(NAME execution-explorer COMMAND execution-explorer-test)
    set_tests_properties(execution-explorer PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(source-editor-test tests/SourceEditorTest.cpp src/files/presentation/SourceEditor.cpp src/files/presentation/SourceHighlighter.cpp
        src/shared-ui/presentation/Theme.cpp src/shared-ui/presentation/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(source-editor-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(source-editor-test PRIVATE Qt6::Widgets dockpipe-syntax)
    add_test(NAME source-editor COMMAND source-editor-test)
    set_tests_properties(source-editor PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

# Native editor dependency; no web runtime or language servers.
foreach(editor_target dockpipe-launcher remote-launcher-test workflow-workspace-test workflow-studio-test project-files-test project-git-test)
    if(TARGET ${editor_target})
        target_link_libraries(${editor_target} PRIVATE dockpipe-syntax)
        if(APPLE AND editor_target STREQUAL "dockpipe-launcher")
            add_custom_command(TARGET ${editor_target} POST_BUILD
                COMMAND ${CMAKE_COMMAND} -E copy_if_different "${DOCKPIPE_SYNTAX_RESOURCE}" "$<TARGET_FILE_DIR:${editor_target}>/dockpipe-syntax.rcc"
                COMMAND ${CMAKE_COMMAND} -E copy_directory "${CMAKE_BINARY_DIR}/syntax-licenses" "$<TARGET_FILE_DIR:${editor_target}>/syntax-licenses")
        endif()
    endif()
endforeach()


# Shared host UI SDK. Optional packages reuse file editors, layout primitives,
# app context and native desktop capture without a spatial or browser dependency.
set(DOCKPIPE_BROWSER_ABI_PROFILE "native" CACHE STRING "Qualified Browser SDK ABI profile")
set_property(CACHE DOCKPIPE_BROWSER_ABI_PROFILE PROPERTY STRINGS native flatpak-kde-6.10)
if(DOCKPIPE_BROWSER_ABI_PROFILE STREQUAL "flatpak-kde-6.10")
    find_package(SpatialWebFlatpakBrowser 0.1.0 EXACT CONFIG REQUIRED)
elseif(DOCKPIPE_BROWSER_ABI_PROFILE STREQUAL "native")
    find_package(SpatialWebBrowser 0.1.0 EXACT CONFIG REQUIRED)
else()
    message(FATAL_ERROR "Unsupported Browser SDK ABI profile: ${DOCKPIPE_BROWSER_ABI_PROFILE}")
endif()
set(DOCKPIPE_UI_SOURCES
    src/workspace-graph/infrastructure/WorkspaceGraphPage.cpp src/workspace-graph/infrastructure/WorkspaceGraphReader.cpp src/workspace-graph/presentation/WorkspaceGraphNode.cpp
    src/workspace-graph/presentation/WorkspaceGraphView.cpp src/workspace-graph/presentation/WorkspaceGraphWidget.cpp
    src/git/presentation/GitCommitQueueModel.cpp src/git/presentation/GitHistoryWidget.cpp src/git/presentation/GitRepositoryWidget.cpp src/git/presentation/GitConflictWidget.cpp
    src/shared-ui/presentation/LauncherStatusBar.cpp
    src/git/infrastructure/ProjectGitClient.cpp src/remote/infrastructure/RemoteCommand.cpp
    src/files/presentation/ProjectFilesWidget.cpp src/files/presentation/FileExplorerView.cpp src/files/presentation/FileExplorerIcons.cpp src/files/presentation/MarkdownPreview.cpp src/files/presentation/MermaidRenderer.cpp src/files/presentation/SourceEditor.cpp src/files/presentation/SourceHighlighter.cpp
    src/shared-ui/presentation/CanvasIcons.cpp src/shared-ui/presentation/Theme.cpp src/shared-ui/presentation/WindowChrome.cpp src/shared-ui/presentation/WindowControlButton.cpp src/execution/domain/Context.cpp
    src/execution/infrastructure/ContextStore.cpp src/catalog/infrastructure/DockpipeChoices.cpp src/catalog/infrastructure/OperatorDockpipeBinary.cpp src/catalog/infrastructure/WorkflowCatalog.cpp src/catalog/domain/WorkflowCatalogData.cpp
    src/shell/infrastructure/LauncherSettings.cpp src/browser/integration/LauncherBrowserServices.cpp
    src/shared-ui/presentation/CanvasPanel.cpp src/dashboards/presentation/DashboardWorkspace.cpp src/dashboards/infrastructure/DashboardStore.cpp src/dashboards/presentation/DashboardDock.cpp src/dashboards/presentation/DashboardFilesWidget.cpp
    src/native-desktop/presentation/DesktopWindowWidget.cpp src/packages/presentation/PackageDisplayWidget.cpp src/native-desktop/infrastructure/DesktopApplicationCatalog.cpp src/native-desktop/infrastructure/DesktopBackground.cpp
    src/packages/infrastructure/PackageCommand.cpp src/packages/infrastructure/WorkspacePackageManager.cpp src/packages/application/SurfaceLaunchEnvironment.cpp)
if(WIN32)
    list(APPEND DOCKPIPE_UI_SOURCES src/native-desktop/infrastructure/DesktopAppWindowWin.cpp)
else()
    list(APPEND DOCKPIPE_UI_SOURCES src/native-desktop/infrastructure/DesktopAppWindow.cpp)
endif()
add_executable(dockpipe-workspace-reader src/bootstrap/composition/WorkspaceReaderMain.cpp src/workspace-graph/infrastructure/WorkspaceGraphPage.cpp)
target_link_libraries(dockpipe-workspace-reader PRIVATE Qt6::Core)
add_library(dockpipe-launcher-ui SHARED ${DOCKPIPE_UI_SOURCES})
target_compile_definitions(dockpipe-launcher-ui
    PRIVATE DOCKPIPE_LAUNCHER_UI_BUILD
        DOCKPIPE_DISPLAY_COMPILER_ID="${CMAKE_CXX_COMPILER_ID}"
        DOCKPIPE_DISPLAY_COMPILER_VERSION="${CMAKE_CXX_COMPILER_VERSION}"
    INTERFACE DOCKPIPE_LAUNCHER_UI_USE)
set_target_properties(dockpipe-launcher-ui PROPERTIES WINDOWS_EXPORT_ALL_SYMBOLS ON VERSION 1.0 SOVERSION 1
    BUILD_RPATH_USE_ORIGIN TRUE)
target_include_directories(dockpipe-launcher-ui PUBLIC
    "$<BUILD_INTERFACE:${DOCKPIPE_APP_INCLUDE_DIRECTORIES}>"
    "$<BUILD_INTERFACE:$<TARGET_PROPERTY:dockpipe-syntax,INTERFACE_INCLUDE_DIRECTORIES>>"
    "$<INSTALL_INTERFACE:include/dockpipe-launcher>"
    "$<INSTALL_INTERFACE:include>"
    "$<INSTALL_INTERFACE:include/dockpipe-launcher/syntax>")
target_link_libraries(dockpipe-launcher-ui PUBLIC Qt6::Widgets Qt6::Network SpatialWeb::Browser PRIVATE dockpipe-syntax Qt6::Concurrent Qt6::OpenGLWidgets)
if(WIN32)
    target_link_libraries(dockpipe-launcher-ui PRIVATE user32 gdi32 advapi32 shell32 ole32)
elseif(APPLE)
    enable_language(OBJCXX)
    target_sources(dockpipe-launcher-ui PRIVATE src/native-desktop/infrastructure/DesktopBackgroundMac.mm)
    set_target_properties(dockpipe-launcher-ui PROPERTIES OBJCXX_STANDARD 17 OBJCXX_STANDARD_REQUIRED ON)
    set_source_files_properties(src/native-desktop/infrastructure/DesktopBackgroundMac.mm PROPERTIES COMPILE_OPTIONS "-fobjc-arc")
    target_link_libraries(dockpipe-launcher-ui PRIVATE "-framework AppKit")
    find_library(DOCKPIPE_SCREEN_CAPTURE_KIT ScreenCaptureKit)
    if(DOCKPIPE_SCREEN_CAPTURE_KIT)
        enable_language(OBJCXX)
        target_sources(dockpipe-launcher-ui PRIVATE src/native-desktop/infrastructure/MacWindowCapture.h src/native-desktop/infrastructure/MacWindowCapture.mm)
        set_target_properties(dockpipe-launcher-ui PROPERTIES OBJCXX_STANDARD 17 OBJCXX_STANDARD_REQUIRED ON)
        set_source_files_properties(src/native-desktop/infrastructure/MacWindowCapture.mm PROPERTIES COMPILE_OPTIONS "-fobjc-arc")
        target_compile_definitions(dockpipe-launcher-ui PRIVATE DOCKPIPE_MAC_WINDOW_CAPTURE)
        target_link_libraries(dockpipe-launcher-ui PRIVATE "-framework AppKit" "-framework CoreMedia"
            "-framework CoreVideo")
        # Older supported desktop builds must reach the runtime availability
        # message even on an OS that does not ship ScreenCaptureKit at all.
        target_link_options(dockpipe-launcher-ui PRIVATE "SHELL:-weak_framework ScreenCaptureKit")
    endif()
elseif(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(X11 QUIET)
    find_library(DOCKPIPE_DESKTOP_XCB_LIBRARY xcb)
    if(X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND X11_Xext_FOUND AND DOCKPIPE_DESKTOP_XCB_LIBRARY)
        target_compile_definitions(dockpipe-launcher-ui PUBLIC DOCKPIPE_X11_BILLBOARDS)
        target_link_libraries(dockpipe-launcher-ui PRIVATE X11::X11 X11::Xcomposite X11::Xrender X11::Xext ${DOCKPIPE_DESKTOP_XCB_LIBRARY})
        if(X11_XRes_FOUND)
            target_compile_definitions(dockpipe-launcher-ui PUBLIC DOCKPIPE_X11_RESOURCE_IDENTITIES)
            target_link_libraries(dockpipe-launcher-ui PRIVATE X11::XRes)
        endif()
    endif()
endif()
option(DOCKPIPE_WAYLAND_CAPTURE "Enable consent-based Wayland window sharing through the desktop portal" ON)
if(DOCKPIPE_WAYLAND_CAPTURE AND CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(Qt6 QUIET COMPONENTS DBus)
    if(TARGET Qt6::DBus)
        target_sources(dockpipe-launcher-ui PRIVATE src/native-desktop/infrastructure/PortalWindowCapture.cpp src/native-desktop/infrastructure/DesktopCaptureFrameBuffer.cpp)
        target_compile_definitions(dockpipe-launcher-ui PRIVATE DOCKPIPE_PORTAL_WINDOW_CAPTURE)
        target_link_libraries(dockpipe-launcher-ui PRIVATE Qt6::DBus)
    else()
        message(STATUS "Qt DBus unavailable: Wayland window sharing will report its missing dependency")
    endif()
endif()
if(CMAKE_SYSTEM_NAME STREQUAL "Linux" AND Qt6_VERSION VERSION_LESS "6.4")
    # Qt 6.2 attaches its GL compositor only when a window's backing store is
    # created. QtOpenGLWidgets registers that factory at library load. Loading it
    # later through an optional UI plugin leaves that
    # existing window unable to composite any GL widgets. Retain this small Qt
    # support library at process startup, even without direct symbol references.
    # All workspace implementation remains optional.
    find_package(Qt6 REQUIRED COMPONENTS OpenGLWidgets)
    target_link_libraries(dockpipe-launcher-ui PRIVATE
        "-Wl,--push-state,--no-as-needed" Qt6::OpenGLWidgets "-Wl,--pop-state")
endif()
get_property(launcher_targets DIRECTORY PROPERTY BUILDSYSTEM_TARGETS)
foreach(ui_target IN LISTS launcher_targets)
    if(ui_target STREQUAL "dockpipe-launcher-ui" OR ui_target STREQUAL "dockpipe-build-version"
            OR ui_target STREQUAL "dockpipe-workspace-reader")
        continue()
    endif()
    get_target_property(ui_sources ${ui_target} SOURCES)
    if(ui_sources)
        list(REMOVE_ITEM ui_sources ${DOCKPIPE_UI_SOURCES})
        set_property(TARGET ${ui_target} PROPERTY SOURCES "${ui_sources}")
        target_link_libraries(${ui_target} PRIVATE dockpipe-launcher-ui)
    endif()
endforeach()
# Ordinary desktop browsing has no dependency on any optional workspace package.
# Compile the shared renderer policy into its consumers without another shipped ABI.
set(DOCKPIPE_TERMINAL_SOURCES src/terminal/infrastructure/TerminalSession.cpp src/terminal/infrastructure/TerminalHostLaunch.cpp)
if(WIN32)
    list(APPEND DOCKPIPE_TERMINAL_SOURCES src/terminal/infrastructure/TerminalSessionWin.cpp)
endif()
add_library(dockpipe-browser STATIC
    src/browser/integration/WebMermaidRenderer.cpp src/browser/integration/WebMermaidRenderer.h mermaid.qrc
    ${DOCKPIPE_TERMINAL_SOURCES} src/terminal/infrastructure/TerminalShellCatalog.cpp src/terminal/application/TerminalHub.cpp src/terminal/presentation/TerminalWidget.cpp src/terminal/presentation/TerminalBridge.h src/terminal/infrastructure/TerminalPreferences.cpp src/terminal/presentation/TerminalSettingsDialog.cpp
    src/terminal/presentation/SendInputWidget.cpp terminal.qrc)
target_include_directories(dockpipe-browser PUBLIC
    "$<BUILD_INTERFACE:${DOCKPIPE_APP_INCLUDE_DIRECTORIES}>"
    "$<INSTALL_INTERFACE:include/dockpipe-launcher>"
    "$<INSTALL_INTERFACE:include>")
target_link_libraries(dockpipe-browser PUBLIC Qt6::WebEngineWidgets Qt6::PrintSupport Qt6::WebChannel dockpipe-launcher-ui)
include(cmake/LauncherSDK.cmake)
if(APPLE)
    target_link_libraries(dockpipe-browser PRIVATE proc)
endif()
foreach(browser_host dockpipe-launcher remote-launcher-test workflow-workspace-test)
    if(TARGET ${browser_host})
        target_link_libraries(${browser_host} PRIVATE dockpipe-browser)
    endif()
endforeach()

if(BUILD_TESTING)
    find_package(Qt6 REQUIRED COMPONENTS Test)
    qt_add_executable(desktop-permissions-widget-test
        tests/DesktopPermissionsWidgetTest.cpp src/native-desktop/presentation/DesktopPermissionsWidget.h)
    target_include_directories(desktop-permissions-widget-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(desktop-permissions-widget-test PRIVATE Qt6::Widgets Qt6::Test)
    dockpipe_target_desktop_permissions(desktop-permissions-widget-test)
    add_test(NAME desktop-permissions-widget COMMAND desktop-permissions-widget-test)
    set_tests_properties(desktop-permissions-widget PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 15)

    add_executable(qualification-workflow-launch-test tests/QualificationWorkflowLaunchTest.cpp
        src/qualification/application/QualificationWorkflowLaunch.cpp src/catalog/presentation/WorkflowLaunchDialog.cpp)
    target_include_directories(qualification-workflow-launch-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(qualification-workflow-launch-test PRIVATE dockpipe-launcher-ui Qt6::Widgets Qt6::Test)
    add_test(NAME qualification-workflow-launch COMMAND qualification-workflow-launch-test)
    set_tests_properties(qualification-workflow-launch PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(package-native-display-harness tests/PackageDisplayHarness.cpp)
    target_link_libraries(package-native-display-harness PRIVATE dockpipe-launcher-ui Qt6::Widgets)

    add_executable(package-native-display-test tests/PackageDisplayTest.cpp src/execution/infrastructure/SessionManager.cpp)
    target_link_libraries(package-native-display-test PRIVATE dockpipe-launcher-ui Qt6::Widgets Qt6::Test)
    add_test(NAME package-native-display COMMAND package-native-display-test)
    set_tests_properties(package-native-display PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(package-surface-test tests/PackageSurfaceTest.cpp src/packages/presentation/PackageSurfacePage.cpp src/execution/infrastructure/SessionManager.cpp)
    target_include_directories(package-surface-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(package-surface-test PRIVATE dockpipe-launcher-ui Qt6::Widgets Qt6::Test)
    if(WIN32)
        add_test(NAME package-surface COMMAND package-surface-test
            -o "${CMAKE_CURRENT_BINARY_DIR}/native-package-surface-results.txt,txt")
    else()
        add_test(NAME package-surface COMMAND package-surface-test)
    endif()
    set_tests_properties(package-surface PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)

    add_executable(terminal-preferences-test tests/TerminalPreferencesTest.cpp)
    target_link_libraries(terminal-preferences-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME terminal-preferences COMMAND terminal-preferences-test)
    set_tests_properties(terminal-preferences PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(terminal-shell-catalog-test tests/TerminalShellCatalogTest.cpp src/terminal/infrastructure/TerminalShellCatalog.cpp)
    target_include_directories(terminal-shell-catalog-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(terminal-shell-catalog-test PRIVATE Qt6::Test Qt6::Core)
    add_test(NAME terminal-shell-catalog COMMAND terminal-shell-catalog-test)
    set_tests_properties(terminal-shell-catalog PROPERTIES TIMEOUT 30)
    add_executable(terminal-host-launch-test tests/TerminalHostLaunchTest.cpp src/terminal/infrastructure/TerminalHostLaunch.cpp)
    target_include_directories(terminal-host-launch-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(terminal-host-launch-test PRIVATE Qt6::Test Qt6::Core)
    add_test(NAME terminal-host-launch COMMAND terminal-host-launch-test)
    set_tests_properties(terminal-host-launch PROPERTIES TIMEOUT 45)
    add_executable(terminal-session-test tests/TerminalSessionTest.cpp ${DOCKPIPE_TERMINAL_SOURCES})
    target_include_directories(terminal-session-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(terminal-session-test PRIVATE Qt6::Test Qt6::Core)
    if(APPLE)
        target_link_libraries(terminal-session-test PRIVATE proc)
    endif()
    add_test(NAME terminal-session COMMAND terminal-session-test)
    set_tests_properties(terminal-session PROPERTIES TIMEOUT 60)
    if(WIN32)
        add_executable(terminal-session-win-test tests/TerminalSessionWinTest.cpp ${DOCKPIPE_TERMINAL_SOURCES})
        target_include_directories(terminal-session-win-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
        target_link_libraries(terminal-session-win-test PRIVATE Qt6::Test Qt6::Core)
        add_test(NAME terminal-session-win COMMAND terminal-session-win-test
            -o "${CMAKE_CURRENT_BINARY_DIR}/native-terminal-session-results.txt,txt")
        set_tests_properties(terminal-session-win PROPERTIES TIMEOUT 60)
    endif()
    add_executable(terminal-widget-test tests/TerminalWidgetTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(terminal-widget-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME terminal-widget COMMAND terminal-widget-test)
    set_tests_properties(terminal-widget PROPERTIES TIMEOUT 45)
    add_executable(window-controls-test tests/WindowControlsTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(window-controls-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME window-controls COMMAND window-controls-test)
    set_tests_properties(window-controls PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(dashboard-workspace-test tests/DashboardWorkspaceTest.cpp)
    target_link_libraries(dashboard-workspace-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-workspace COMMAND dashboard-workspace-test)
    set_tests_properties(dashboard-workspace PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(dashboard-dock-test tests/DashboardDockTest.cpp)
    target_link_libraries(dashboard-dock-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-dock COMMAND dashboard-dock-test)
    set_tests_properties(dashboard-dock PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(dashboard-performance-test tests/DashboardPerformanceTest.cpp)
    target_link_libraries(dashboard-performance-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-performance COMMAND dashboard-performance-test)
    set_tests_properties(dashboard-performance PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
    add_executable(desktop-application-catalog-test tests/DesktopApplicationCatalogTest.cpp)
    target_link_libraries(desktop-application-catalog-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME desktop-application-catalog COMMAND desktop-application-catalog-test)
    set_tests_properties(desktop-application-catalog PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(dashboard-store-test tests/DashboardStoreTest.cpp)
    target_link_libraries(dashboard-store-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-store COMMAND dashboard-store-test)
    set_tests_properties(dashboard-store PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30 SKIP_RETURN_CODE 77)
    add_executable(dashboard-widgets-test tests/DashboardWidgetFactoryTest.cpp
        src/dashboards/presentation/DashboardWidgetFactory.cpp src/dashboards/presentation/DashboardWidgetViews.cpp src/dashboards/presentation/DashboardLogWidget.cpp
        src/execution/infrastructure/SessionManager.cpp src/observability/presentation/SystemResourceOverview.cpp src/observability/presentation/ExecutionMetricChart.cpp)
    target_link_libraries(dashboard-widgets-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-widgets COMMAND dashboard-widgets-test)
    set_tests_properties(dashboard-widgets PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(launcher-status-bar-test tests/LauncherStatusBarTest.cpp)
    target_link_libraries(launcher-status-bar-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME launcher-status-bar COMMAND launcher-status-bar-test)
    set_tests_properties(launcher-status-bar PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 15)
    add_executable(dashboard-files-test tests/DashboardFilesWidgetTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-files-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-files COMMAND dashboard-files-test)
    set_tests_properties(dashboard-files PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(desktop-background-test tests/DesktopBackgroundTest.cpp)
    target_link_libraries(desktop-background-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME desktop-background COMMAND desktop-background-test)
    set_tests_properties(desktop-background PROPERTIES TIMEOUT 15)

    add_executable(dashboard-tour-test tests/DashboardTourDialogTest.cpp src/dashboards/presentation/DashboardTourDialog.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-tour-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-tour COMMAND dashboard-tour-test)
    set_tests_properties(dashboard-tour PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(dashboard-page-test tests/DashboardPageTest.cpp
        src/dashboards/presentation/DashboardPage.cpp src/dashboards/presentation/DashboardSettingsDialog.cpp src/dashboards/presentation/DashboardTourDialog.cpp src/dashboards/presentation/DashboardAppsDialog.cpp
        src/dashboards/presentation/DashboardWidgetFactory.cpp src/dashboards/presentation/DashboardWidgetViews.cpp src/dashboards/presentation/DashboardLogWidget.cpp
        src/execution/infrastructure/SessionManager.cpp src/observability/presentation/SystemResourceOverview.cpp src/observability/presentation/ExecutionMetricChart.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-page-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test Qt6::Concurrent)
    if(CMAKE_SYSTEM_NAME STREQUAL "Linux" AND X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_DESKTOP_XCB_LIBRARY)
        target_link_libraries(dashboard-page-test PRIVATE X11::X11)
    endif()
    add_test(NAME dashboard-file-views COMMAND dashboard-page-test fileScopesAndCanvasProjectPicker)
    set_tests_properties(dashboard-file-views PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_test(NAME package-display-identity COMMAND dashboard-page-test packageDisplayIdentitySelection)
    set_tests_properties(package-display-identity PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_test(NAME dashboard-page COMMAND dashboard-page-test)
    set_tests_properties(dashboard-page PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
    add_executable(dashboard-browser-performance-test tests/DashboardBrowserPerformanceTest.cpp
        src/dashboards/presentation/DashboardWidgetFactory.cpp src/dashboards/presentation/DashboardWidgetViews.cpp src/dashboards/presentation/DashboardLogWidget.cpp
        src/execution/infrastructure/SessionManager.cpp src/observability/presentation/SystemResourceOverview.cpp src/observability/presentation/ExecutionMetricChart.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-browser-performance-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-browser-performance COMMAND dashboard-browser-performance-test)
    set_tests_properties(dashboard-browser-performance PROPERTIES
        ENVIRONMENT "QT_QPA_PLATFORM=offscreen"
        TIMEOUT 45
        RUN_SERIAL TRUE)
    add_executable(desktop-window-widget-test tests/DesktopWindowWidgetTest.cpp)
    target_link_libraries(desktop-window-widget-test PRIVATE dockpipe-launcher-ui)
    add_test(NAME desktop-window-widget COMMAND desktop-window-widget-test)
    set_tests_properties(desktop-window-widget PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    if(CMAKE_SYSTEM_NAME STREQUAL "Linux" AND X11_Xcomposite_FOUND AND X11_Xrender_FOUND
            AND X11_Xext_FOUND AND DOCKPIPE_DESKTOP_XCB_LIBRARY)
        # A host-owned window fixture; optional package test implementations stay external.
        add_executable(desktop-window-fixture tests/DesktopWindowFixture.cpp)
        target_link_libraries(desktop-window-fixture PRIVATE Qt6::Widgets)
        add_dependencies(desktop-window-widget-test desktop-window-fixture)
        add_dependencies(dashboard-page-test desktop-window-fixture)
    endif()
    if(WIN32)
        # Hosted Windows runners are administrators. Exercise embedding with the
        # same unelevated identity required of real target applications.
        add_executable(windows-test-launcher tests/WindowsTestLauncher.cpp)
        set_target_properties(windows-test-launcher PROPERTIES AUTOMOC OFF AUTORCC OFF AUTOUIC OFF)
        target_link_libraries(windows-test-launcher PRIVATE advapi32 user32)
        add_test(NAME windows-test-launcher COMMAND windows-test-launcher --self-test)
        set_tests_properties(windows-test-launcher PROPERTIES TIMEOUT 45)
        add_executable(desktop-app-window-win-test tests/DesktopAppWindowWinTest.cpp)
        add_dependencies(desktop-app-window-win-test windows-test-launcher)
        target_link_libraries(desktop-app-window-win-test PRIVATE dockpipe-launcher-ui user32 advapi32)
        add_test(NAME desktop-app-window-win
            COMMAND windows-test-launcher $<TARGET_FILE:desktop-app-window-win-test>)
        # Preserve the test's 45-second execution budget and allow bounded cleanup.
        set_tests_properties(desktop-app-window-win PROPERTIES TIMEOUT 50 SKIP_RETURN_CODE 77)
    elseif(DOCKPIPE_WAYLAND_CAPTURE AND CMAKE_SYSTEM_NAME STREQUAL "Linux" AND TARGET Qt6::DBus)
        add_executable(desktop-capture-frame-buffer-test tests/DesktopCaptureFrameBufferTest.cpp)
        target_link_libraries(desktop-capture-frame-buffer-test PRIVATE dockpipe-launcher-ui Qt6::Test)
        add_test(NAME desktop-capture-frame-buffer COMMAND desktop-capture-frame-buffer-test)
        set_tests_properties(desktop-capture-frame-buffer PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
        add_executable(portal-window-capture-test tests/PortalWindowCaptureTest.cpp)
        target_link_libraries(portal-window-capture-test PRIVATE dockpipe-launcher-ui Qt6::DBus Qt6::Test)
        add_test(NAME portal-window-capture COMMAND portal-window-capture-test)
        set_tests_properties(portal-window-capture PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45 SKIP_RETURN_CODE 77)
    endif()
    add_executable(browser-preferences-test tests/BrowserPreferencesTest.cpp)
    target_link_libraries(browser-preferences-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME browser-preferences COMMAND browser-preferences-test)
    set_tests_properties(browser-preferences PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(browser-link-router-test tests/BrowserLinkRouterTest.cpp)
    target_link_libraries(browser-link-router-test PRIVATE dockpipe-launcher-ui)
    add_test(NAME browser-link-router COMMAND browser-link-router-test)
    set_tests_properties(browser-link-router PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30 SKIP_RETURN_CODE 77)
    add_executable(browser-settings-dialog-test tests/BrowserSettingsDialogTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(browser-settings-dialog-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME browser-settings-dialog COMMAND browser-settings-dialog-test)
    set_tests_properties(browser-settings-dialog PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(browser-test tests/BrowserTest.cpp)
    target_link_libraries(browser-test PRIVATE dockpipe-browser Qt6::Network Qt6::Test)
    add_executable(browser-annotations-test tests/BrowserAnnotationDialogTest.cpp)
    target_link_libraries(browser-annotations-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME browser-annotations COMMAND browser-annotations-test)
    set_tests_properties(browser-annotations PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(browser-extensions-test tests/BrowserExtensionsTest.cpp)
    target_link_libraries(browser-extensions-test PRIVATE dockpipe-browser)
    add_test(NAME browser-extensions COMMAND browser-extensions-test)
    set_tests_properties(browser-extensions PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
    add_test(NAME browser COMMAND browser-test)
    # The offscreen platform has no native GL context. Keep these lifecycle
    # tests on the software renderer without changing the product renderer.
    set_tests_properties(browser PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;QT_QUICK_BACKEND=software;QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu" TIMEOUT 60)
endif()

set_target_properties(dockpipe-launcher PROPERTIES BUILD_RPATH_USE_ORIGIN TRUE INSTALL_RPATH "$ORIGIN;$ORIGIN/../lib/dockpipe")
if(UNIX AND NOT APPLE)
    set_property(TARGET dockpipe-launcher APPEND PROPERTY BUILD_RPATH "$ORIGIN/../lib/dockpipe")
endif()

if(BUILD_TESTING)
    add_library(workspace-package-fixture MODULE tests/WorkspacePackageFixture.cpp)
    target_link_libraries(workspace-package-fixture PRIVATE dockpipe-launcher-ui)
    set_target_properties(workspace-package-fixture PROPERTIES OUTPUT_NAME dockpipe-cockpit
        SUFFIX "${CMAKE_SHARED_LIBRARY_SUFFIX}"
        LIBRARY_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture/ui"
        RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture/ui")
    add_executable(workspace-package-test tests/WorkspacePackageTest.cpp src/shell/presentation/SettingsDialog.cpp)
    target_link_libraries(workspace-package-test PRIVATE dockpipe-launcher-ui)
    if(APPLE)
        dockpipe_target_desktop_permissions(workspace-package-test)
    endif()
    add_dependencies(workspace-package-test workspace-package-fixture)
    add_test(NAME workspace-package COMMAND workspace-package-test)
    set_tests_properties(workspace-package PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture"
        TIMEOUT 30 SKIP_RETURN_CODE 77)
    add_executable(workspace-package-manager-test tests/WorkspacePackageManagerTest.cpp)
    target_link_libraries(workspace-package-manager-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_dependencies(workspace-package-manager-test workspace-package-fixture)
    add_test(NAME workspace-package-manager COMMAND workspace-package-manager-test)
    set_tests_properties(workspace-package-manager PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture"
        TIMEOUT 30)
    add_executable(web-mermaid-renderer-test tests/WebMermaidRendererTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(web-mermaid-renderer-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME web-mermaid-renderer COMMAND web-mermaid-renderer-test)
    set_tests_properties(web-mermaid-renderer PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu" TIMEOUT 120)
    add_executable(project-files-markdown-test tests/ProjectFilesMarkdownTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(project-files-markdown-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    if(WIN32)
        add_test(NAME project-files-markdown COMMAND project-files-markdown-test
            -o "${CMAKE_CURRENT_BINARY_DIR}/native-project-files-markdown-results.txt,txt")
    else()
        add_test(NAME project-files-markdown COMMAND project-files-markdown-test)
    endif()
    set_tests_properties(project-files-markdown PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(project-files-graph-test tests/ProjectFilesGraphTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(project-files-graph-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_dependencies(project-files-graph-test dockpipe-workspace-reader)
    add_test(NAME project-files-graph COMMAND project-files-graph-test)
    set_tests_properties(project-files-graph PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_READER=$<TARGET_FILE:dockpipe-workspace-reader>"
        TIMEOUT 45)
    add_executable(launcher-settings-test tests/LauncherSettingsTest.cpp src/shell/presentation/SettingsDialog.cpp)
    target_link_libraries(launcher-settings-test PRIVATE dockpipe-launcher-ui)
    if(APPLE)
        dockpipe_target_desktop_permissions(launcher-settings-test)
    endif()
    add_test(NAME launcher-settings COMMAND launcher-settings-test)
    add_dependencies(launcher-settings-test workspace-package-fixture)
    set_tests_properties(launcher-settings PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture" TIMEOUT 30)
endif()

# Optional package source belongs to SpatialPipe. Host qualification uses only
# already-built assets through the same loader used by installed applications.
if(DOCKPIPE_COCKPIT_SOURCE)
    message(FATAL_ERROR "DOCKPIPE_COCKPIT_SOURCE was removed; use DOCKPIPE_COCKPIT_ASSETS")
endif()
set(DOCKPIPE_COCKPIT_ASSETS "" CACHE PATH "Installed Cockpit assets for host loader/rendering qualification")
if(DOCKPIPE_COCKPIT_ASSETS)
    if(NOT BUILD_TESTING)
        message(FATAL_ERROR "DOCKPIPE_COCKPIT_ASSETS requires BUILD_TESTING=ON")
    endif()
    include(cmake/InstalledCockpitTest.cmake)
endif()

if(BUILD_TESTING)
    add_executable(git-commit-queue-test tests/GitCommitQueueModelTest.cpp)
    target_link_libraries(git-commit-queue-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    target_include_directories(git-commit-queue-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    add_test(NAME git-commit-queue COMMAND git-commit-queue-test)
    set_tests_properties(git-commit-queue PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(git-repository-conflict-test tests/GitRepositoryConflictTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(git-repository-conflict-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    target_include_directories(git-repository-conflict-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    if(WIN32)
        add_test(NAME git-repository-conflict COMMAND git-repository-conflict-test
            -o "${CMAKE_CURRENT_BINARY_DIR}/native-git-repository-conflict-results.txt,txt")
    else()
        add_test(NAME git-repository-conflict COMMAND git-repository-conflict-test)
    endif()
    set_tests_properties(git-repository-conflict PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
endif()

if(BUILD_TESTING)
    add_executable(git-history-widget-test tests/GitHistoryWidgetTest.cpp dockpipe-launcher.qrc)
    target_link_libraries(git-history-widget-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    target_include_directories(git-history-widget-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    add_test(NAME git-history-widget COMMAND git-history-widget-test)
    set_tests_properties(git-history-widget PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
endif()

if(BUILD_TESTING)
    add_executable(browser-extension-package-test tests/BrowserExtensionPackageTest.cpp)
    target_link_libraries(browser-extension-package-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME browser-extension-package COMMAND browser-extension-package-test)
    set_tests_properties(browser-extension-package PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(browser-extension-store-test tests/BrowserExtensionStoreTest.cpp)
    target_link_libraries(browser-extension-store-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME browser-extension-store COMMAND browser-extension-store-test)
    set_tests_properties(browser-extension-store PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
endif()

if(BUILD_TESTING)
    add_executable(browser-extension-crx-test tests/BrowserExtensionCrxTest.cpp)
    target_link_libraries(browser-extension-crx-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME browser-extension-crx COMMAND browser-extension-crx-test)
    set_tests_properties(browser-extension-crx PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(browser-extension-compatibility-test tests/BrowserExtensionCompatibilityTest.cpp)
    target_link_libraries(browser-extension-compatibility-test PRIVATE dockpipe-browser)
    add_test(NAME browser-extension-compatibility COMMAND browser-extension-compatibility-test)
    set_tests_properties(browser-extension-compatibility PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 100 SKIP_RETURN_CODE 77)
endif()

if(BUILD_TESTING)
    add_executable(canvas-widget-presentation-test tests/CanvasWidgetPresentationTest.cpp dockpipe-launcher.qrc
        src/dashboards/presentation/DashboardWidgetFactory.cpp src/dashboards/presentation/DashboardWidgetViews.cpp src/dashboards/presentation/DashboardLogWidget.cpp
        src/execution/infrastructure/SessionManager.cpp src/observability/presentation/SystemResourceOverview.cpp src/observability/presentation/ExecutionMetricChart.cpp)
    target_link_libraries(canvas-widget-presentation-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test)
    add_test(NAME canvas-widget-presentation COMMAND canvas-widget-presentation-test)
    set_tests_properties(canvas-widget-presentation PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_test(NAME browser-canvas-mode COMMAND browser-test canvasViewportIsContextScoped
        tearOutRetainsLivePageAndSessionAfterSourceCloses tabTearOutGestureIsCanvasOnly)
    set_tests_properties(browser-canvas-mode PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;QT_QUICK_BACKEND=software;QTWEBENGINE_CHROMIUM_FLAGS=--disable-gpu" TIMEOUT 45)
    add_test(NAME dashboard-canvas-widgets COMMAND dashboard-page-test canvasGadgetsAndBrowserTearOut)
    set_tests_properties(dashboard-canvas-widgets PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
endif()

include("${CMAKE_CURRENT_LIST_DIR}/cmake/WindowsOffscreenTestFonts.cmake")
dockpipe_configure_windows_offscreen_test_fonts("$ENV{SystemRoot}/Fonts")

include(cmake/AppRegressionTests.cmake)

if(BUILD_TESTING)
    find_package(Python3 COMPONENTS Interpreter QUIET)
    if(Python3_Interpreter_FOUND)
        add_test(NAME app-source-layout
            COMMAND "${Python3_EXECUTABLE}" "${CMAKE_CURRENT_SOURCE_DIR}/tests/test_source_layout.py")
    endif()
endif()

if(BUILD_TESTING)
    add_executable(hardware-key-transport-test tests/HardwareKeyTransportTest.cpp
        src/remote/infrastructure/RemoteCommand.cpp)
    target_include_directories(hardware-key-transport-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(hardware-key-transport-test PRIVATE Qt6::Core)
    add_test(NAME hardware-key-transport COMMAND hardware-key-transport-test)
    set_tests_properties(hardware-key-transport PROPERTIES TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(hardware-key-dialog-test tests/HardwareKeyDialogTest.cpp
        src/remote/presentation/HardwareKeyDialog.cpp)
    target_include_directories(hardware-key-dialog-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(hardware-key-dialog-test PRIVATE Qt6::Widgets)
    add_test(NAME hardware-key-dialog COMMAND hardware-key-dialog-test)
    set_tests_properties(hardware-key-dialog PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(operator-dockpipe-binary-test tests/OperatorDockpipeBinaryTest.cpp
        src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
    target_include_directories(operator-dockpipe-binary-test PRIVATE ${DOCKPIPE_APP_INCLUDE_DIRECTORIES})
    target_link_libraries(operator-dockpipe-binary-test PRIVATE Qt6::Core)
    add_test(NAME operator-dockpipe-binary COMMAND operator-dockpipe-binary-test)
endif()
