cmake_minimum_required(VERSION 3.21.1)
project(dockpipe-launcher VERSION 0.1.0 LANGUAGES CXX)
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

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

# Qt6Gui depends on WrapOpenGL; without Mesa dev packages, find_package(Qt6) fails obscurely on Linux.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(OpenGL REQUIRED)
endif()

find_package(Qt6 REQUIRED COMPONENTS Widgets Network Concurrent OpenGLWidgets)
add_subdirectory(third_party/ksyntaxhighlighting)

set(DOCKPIPE_LAUNCHER_SOURCES
    src/main.cpp
    src/LauncherEnvironment.cpp
    src/LauncherDiagnostics.cpp
    src/FlatpakTerminal.cpp
    src/SingleInstanceGuard.cpp
    src/Theme.cpp
    src/WindowChrome.cpp
    src/Context.cpp
    src/ContextStore.cpp
    src/GitHelper.cpp
    src/SessionManager.cpp
    src/DockpipeChoices.cpp
    src/ContextDiscovery.cpp
    src/WorkflowCatalog.cpp
    src/LauncherSettings.cpp
    src/AppCardWidget.cpp
    src/BasicModeWidget.cpp
    src/PackageCommand.cpp
    src/PackageManagerDialog.cpp
    src/SettingsDialog.cpp
    src/LogViewerDialog.cpp
    src/PromptDialog.cpp
    src/WorkflowLaunchDialog.cpp
    src/CanvasIcons.cpp
    src/CanvasPanel.cpp
    src/WorkflowCanvas.cpp
    src/WorkflowEditorWidget.cpp
    src/WorkflowScriptPath.cpp
    src/ProjectFilesWidget.cpp
    src/ProjectGitWidget.cpp
    src/SourceEditor.cpp
    src/SourceHighlighter.cpp
    src/DockerObservabilityWidget.cpp
    src/DockerRemoteMonitoring.cpp
    src/DockerResourceInspector.cpp
    src/ResourceCanvas.cpp
    src/ResourceLinkItem.cpp
    src/ContextRowWidget.cpp
    src/EditContextDialog.cpp
    src/SetupTerminal.cpp
    src/RemoteCommand.cpp
    src/RemoteWidget.cpp
    src/RemoteRunDialog.cpp
    src/ActivityWidget.cpp
    src/ExecutionExplorer.cpp
    src/ExecutionMetricChart.cpp
    src/SystemResourceOverview.cpp
    src/MainWindow.cpp
    src/MainWindowWorkflows.cpp
    dockpipe-launcher.qrc
)

if(WIN32)
    list(APPEND DOCKPIPE_LAUNCHER_SOURCES resources/windows/dockpipe-launcher.rc)
    qt_add_executable(dockpipe-launcher WIN32 ${DOCKPIPE_LAUNCHER_SOURCES})
else()
    qt_add_executable(dockpipe-launcher ${DOCKPIPE_LAUNCHER_SOURCES})
endif()

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

target_include_directories(dockpipe-launcher PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
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
        src/LauncherEnvironment.cpp
    )
    target_include_directories(launcher-environment-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(launcher-environment-test PRIVATE Qt6::Core)
    add_test(NAME launcher-environment COMMAND launcher-environment-test)
    add_executable(package-command-test tests/PackageCommandTest.cpp src/PackageCommand.cpp)
    target_include_directories(package-command-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(package-command-test PRIVATE Qt6::Core)
    add_test(NAME package-command COMMAND package-command-test)
    add_executable(global-root-test tests/GlobalRootTest.cpp src/LauncherEnvironment.cpp)
    target_include_directories(global-root-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(global-root-test PRIVATE Qt6::Core)
    add_test(NAME global-root-default COMMAND global-root-test)
    add_test(NAME global-root-inherited COMMAND global-root-test --inherited)
    add_executable(package-manager-test
        tests/PackageManagerTest.cpp src/PackageManagerDialog.cpp src/PackageCommand.cpp
        src/DockpipeChoices.cpp src/WorkflowCatalog.cpp src/LauncherSettings.cpp src/ContextStore.cpp src/Context.cpp
        src/ContextStore.cpp src/Context.cpp)
    target_include_directories(package-manager-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(package-manager-test PRIVATE Qt6::Widgets)
    add_test(NAME package-manager COMMAND package-manager-test)
    set_tests_properties(package-manager PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
        add_executable(flatpak-terminal-test
            tests/FlatpakTerminalTest.cpp src/FlatpakTerminal.cpp)
        target_include_directories(flatpak-terminal-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
        target_link_libraries(flatpak-terminal-test PRIVATE Qt6::Core)
        add_dependencies(flatpak-terminal-test dockpipe-launcher)
        add_test(NAME flatpak-terminal COMMAND flatpak-terminal-test $<TARGET_FILE:dockpipe-launcher>)
        set_tests_properties(flatpak-terminal PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
    endif()
    add_executable(launcher-diagnostics-test tests/LauncherDiagnosticsTest.cpp src/LauncherDiagnostics.cpp)
    target_include_directories(launcher-diagnostics-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(launcher-diagnostics-test PRIVATE Qt6::Core)
    add_test(NAME launcher-diagnostics COMMAND launcher-diagnostics-test)
    add_test(NAME launcher-diagnostics-opt-in COMMAND launcher-diagnostics-test --diagnostics)
    add_test(NAME launcher-diagnostics-native COMMAND launcher-diagnostics-test --native)
endif()

if(BUILD_TESTING AND UNIX)
    set(REMOTE_LAUNCHER_TEST_SOURCES ${DOCKPIPE_LAUNCHER_SOURCES})
    list(REMOVE_ITEM REMOTE_LAUNCHER_TEST_SOURCES src/main.cpp)
    set(REMOTE_LAUNCHER_UI_TEST_SOURCES ${REMOTE_LAUNCHER_TEST_SOURCES})
    if(APPLE)
        # macOS uses an absolute osascript path, so a PATH fixture cannot isolate
        # Terminal.app. Keep UI command assertions without opening a real app.
        list(REMOVE_ITEM REMOTE_LAUNCHER_UI_TEST_SOURCES src/SetupTerminal.cpp)
        list(APPEND REMOTE_LAUNCHER_UI_TEST_SOURCES tests/SetupTerminalFixture.cpp)
    endif()
    add_executable(remote-launcher-test tests/RemoteLauncherTest.cpp tests/SystemMonitoringTest.cpp ${REMOTE_LAUNCHER_UI_TEST_SOURCES})
    target_include_directories(remote-launcher-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(remote-launcher-test PRIVATE Qt6::Widgets Qt6::Network Qt6::Concurrent)
    add_executable(workflow-workspace-test tests/WorkflowWorkspaceTest.cpp ${REMOTE_LAUNCHER_TEST_SOURCES})
    target_include_directories(workflow-workspace-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(workflow-workspace-test PRIVATE Qt6::Widgets Qt6::Network Qt6::Concurrent)
    add_test(NAME workflow-workspace COMMAND workflow-workspace-test)
    set_tests_properties(workflow-workspace PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
    add_test(NAME remote-launcher COMMAND remote-launcher-test)
    set_tests_properties(remote-launcher PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(workflow-studio-test
        tests/WorkflowStudioTest.cpp src/CanvasIcons.cpp src/CanvasPanel.cpp src/WorkflowCanvas.cpp src/WorkflowEditorWidget.cpp src/WorkflowScriptPath.cpp src/ProjectFilesWidget.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp
        src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(workflow-studio-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(workflow-studio-test PRIVATE Qt6::Widgets)
    add_test(NAME workflow-studio COMMAND workflow-studio-test)
    set_tests_properties(workflow-studio PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
    add_executable(workflow-script-path-test tests/WorkflowScriptPathTest.cpp src/WorkflowScriptPath.cpp)
    target_include_directories(workflow-script-path-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(workflow-script-path-test PRIVATE Qt6::Core)
    add_test(NAME workflow-script-path COMMAND workflow-script-path-test)
    add_executable(project-files-test tests/ProjectFilesTest.cpp src/ProjectFilesWidget.cpp src/CanvasIcons.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp
        src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(project-files-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(project-files-test PRIVATE Qt6::Widgets)
    add_test(NAME project-files COMMAND project-files-test)
    set_tests_properties(project-files PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(project-git-test tests/ProjectGitTest.cpp src/ProjectGitWidget.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp
        src/RemoteCommand.cpp src/DockpipeChoices.cpp src/WorkflowCatalog.cpp src/LauncherSettings.cpp src/ContextStore.cpp src/Context.cpp
        src/CanvasIcons.cpp src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(project-git-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(project-git-test PRIVATE Qt6::Widgets)
    add_test(NAME project-git COMMAND project-git-test)
    set_tests_properties(project-git PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45 SKIP_RETURN_CODE 77)
endif()

if(BUILD_TESTING)
    add_executable(execution-explorer-test
        tests/ExecutionExplorerTest.cpp src/ExecutionExplorer.cpp src/ExecutionMetricChart.cpp src/SystemResourceOverview.cpp
        src/RemoteCommand.cpp src/DockpipeChoices.cpp src/WorkflowCatalog.cpp src/LauncherSettings.cpp src/ContextStore.cpp src/Context.cpp
        src/ResourceCanvas.cpp src/ResourceLinkItem.cpp src/CanvasPanel.cpp src/CanvasIcons.cpp
        src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(execution-explorer-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(execution-explorer-test PRIVATE Qt6::Widgets)
    add_test(NAME execution-explorer COMMAND execution-explorer-test)
    set_tests_properties(execution-explorer PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
endif()

if(BUILD_TESTING)
    add_executable(source-editor-test tests/SourceEditorTest.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp
        src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(source-editor-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
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

# Workspace graph is a lazily created view over the existing Files scope.
set(WORKSPACE_GRAPH_SOURCES
    src/SpatialWorkspace.cpp src/SpatialWorkspaceStore.cpp src/SpatialWorkspaceEditor.cpp
    src/CockpitWorkspaces.cpp src/ContextStore.cpp src/Context.cpp
    src/SpatialInteriorGeometry.cpp src/WorkspaceFlightInterior.cpp src/SpatialDisplayDialog.cpp
    src/WorkspaceGraphWidget.cpp src/WorkspaceGraphNode.cpp
    src/WorkspaceGraphReader.cpp src/WorkspaceGraphPage.cpp src/WorkspaceGraphView.cpp
    src/WorkspaceFlightView.cpp src/WorkspaceFlightOrbit.cpp src/WorkspaceFlightCockpit.cpp src/FlightPlanetArt.cpp
    src/CockpitWidget.cpp src/CockpitCommandDeck.cpp src/CockpitHudPanel.cpp src/CockpitVoice.cpp src/CockpitController.cpp src/DesktopAppCatalog.cpp)
foreach(graph_target dockpipe-launcher remote-launcher-test workflow-workspace-test workflow-studio-test project-files-test)
    if(TARGET ${graph_target})
        target_sources(${graph_target} PRIVATE ${WORKSPACE_GRAPH_SOURCES})
        target_link_libraries(${graph_target} PRIVATE Qt6::OpenGLWidgets)
    endif()
endforeach()

if(BUILD_TESTING)
    add_executable(workspace-graph-test tests/WorkspaceGraphTest.cpp ${WORKSPACE_GRAPH_SOURCES}
        src/ProjectFilesWidget.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp src/CanvasIcons.cpp
        src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(workspace-graph-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(workspace-graph-test PRIVATE Qt6::Widgets Qt6::OpenGLWidgets dockpipe-syntax)
    add_test(NAME workspace-graph COMMAND workspace-graph-test)
    set_tests_properties(workspace-graph PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 90)
endif()

# Optional native app streaming; other platforms retain an explanatory UI stub.
option(DOCKPIPE_APP_BILLBOARDS "Build the experimental X11 app billboard backend" ON)
if(DOCKPIPE_APP_BILLBOARDS AND CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(X11 QUIET)
    find_library(DOCKPIPE_XCB_LIBRARY xcb)
endif()
foreach(graph_target dockpipe-launcher remote-launcher-test workflow-workspace-test workflow-studio-test project-files-test workspace-graph-test)
    if(TARGET ${graph_target})
        target_sources(${graph_target} PRIVATE src/DesktopAppWindow.cpp src/FlightAppBillboards.cpp)
        if(DOCKPIPE_APP_BILLBOARDS AND X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_XCB_LIBRARY)
            target_compile_definitions(${graph_target} PRIVATE DOCKPIPE_X11_BILLBOARDS)
            target_link_libraries(${graph_target} PRIVATE X11::X11 X11::Xcomposite X11::Xrender ${DOCKPIPE_XCB_LIBRARY})
        endif()
    endif()
endforeach()

if(BUILD_TESTING)
    add_executable(desktop-app-window-test tests/DesktopAppWindowTest.cpp src/DesktopAppWindow.cpp)
    target_include_directories(desktop-app-window-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(desktop-app-window-test PRIVATE Qt6::Widgets)
    if(DOCKPIPE_APP_BILLBOARDS AND X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_XCB_LIBRARY)
        target_compile_definitions(desktop-app-window-test PRIVATE DOCKPIPE_X11_BILLBOARDS)
        target_link_libraries(desktop-app-window-test PRIVATE X11::X11 X11::Xcomposite X11::Xrender ${DOCKPIPE_XCB_LIBRARY})
    endif()
    add_test(NAME desktop-app-window COMMAND desktop-app-window-test)
    set_tests_properties(desktop-app-window PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45 SKIP_RETURN_CODE 77)
endif()

if(BUILD_TESTING)
    add_executable(launcher-settings-test tests/LauncherSettingsTest.cpp
        src/LauncherSettings.cpp src/SettingsDialog.cpp src/ContextStore.cpp src/Context.cpp)
    target_include_directories(launcher-settings-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(launcher-settings-test PRIVATE Qt6::Widgets)
    add_test(NAME launcher-settings COMMAND launcher-settings-test)
    set_tests_properties(launcher-settings PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 15)
    add_executable(cockpit-test tests/CockpitTest.cpp tests/SpatialCockpitTest.cpp tests/NativeHandoffTest.cpp ${WORKSPACE_GRAPH_SOURCES}
        src/ProjectFilesWidget.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp src/CanvasIcons.cpp
        src/DesktopAppWindow.cpp src/FlightAppBillboards.cpp src/Theme.cpp src/WindowChrome.cpp dockpipe-launcher.qrc)
    target_include_directories(cockpit-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(cockpit-test PRIVATE Qt6::Widgets Qt6::OpenGLWidgets dockpipe-syntax)
    if(DOCKPIPE_APP_BILLBOARDS AND X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_XCB_LIBRARY)
        target_compile_definitions(cockpit-test PRIVATE DOCKPIPE_X11_BILLBOARDS)
        target_link_libraries(cockpit-test PRIVATE X11::X11 X11::Xcomposite X11::Xrender ${DOCKPIPE_XCB_LIBRARY})
    endif()
    add_test(NAME cockpit COMMAND cockpit-test)
    set_tests_properties(cockpit PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
    add_test(NAME native-handoff COMMAND cockpit-test --native-handoff)
    set_tests_properties(native-handoff PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60 SKIP_RETURN_CODE 77)
endif()

if(BUILD_TESTING)
    add_executable(spatial-workspace-test tests/SpatialWorkspaceTest.cpp
        src/SpatialWorkspace.cpp src/SpatialWorkspaceStore.cpp src/ContextStore.cpp src/Context.cpp)
    target_include_directories(spatial-workspace-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(spatial-workspace-test PRIVATE Qt6::Core)
    add_test(NAME spatial-workspace COMMAND spatial-workspace-test)
endif()

if(TARGET cockpit-test)
    add_test(NAME spatial-cockpit COMMAND cockpit-test --spatial)
    set_tests_properties(spatial-cockpit PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
endif()
