cmake_minimum_required(VERSION 3.21.1)
project(dockpipe-launcher VERSION 0.1.0 LANGUAGES CXX)
# Interactive rendering needs optimization even in a plain local CMake build.
# Preserve explicitly selected Debug builds and multi-configuration generators.
if(NOT CMAKE_CONFIGURATION_TYPES AND NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release CACHE STRING "Build type" FORCE)
endif()
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

# Qt6Gui depends on WrapOpenGL; without Mesa dev packages, find_package(Qt6) fails obscurely on Linux.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(OpenGL REQUIRED)
endif()

find_package(Qt6 REQUIRED COMPONENTS Widgets Network Concurrent WebEngineWidgets PrintSupport WebChannel)
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
    src/ProjectGitClient.cpp
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
    src/MainWindowDashboards.cpp
    src/DashboardPage.cpp
    src/DashboardSettingsDialog.cpp
    src/DashboardTourDialog.cpp
    src/DashboardAppsDialog.cpp
    src/DashboardWidgetFactory.cpp
    src/DashboardWidgetViews.cpp
    src/DashboardLogWidget.cpp
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
    add_executable(project-git-test tests/ProjectGitTest.cpp src/ProjectGitWidget.cpp src/ProjectGitClient.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp
        src/RemoteCommand.cpp src/DockpipeChoices.cpp src/WorkflowCatalog.cpp src/LauncherSettings.cpp src/ContextStore.cpp src/Context.cpp
        src/CanvasIcons.cpp src/Theme.cpp src/WindowChrome.cpp src/WindowControlButton.cpp dockpipe-launcher.qrc)
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


# Shared host UI SDK. Optional packages reuse file editors, layout primitives,
# app context and native desktop capture without a spatial or browser dependency.
set(DOCKPIPE_UI_SOURCES
    src/ProjectGitClient.cpp src/RemoteCommand.cpp
    src/ProjectFilesWidget.cpp src/SourceEditor.cpp src/SourceHighlighter.cpp
    src/CanvasIcons.cpp src/Theme.cpp src/WindowChrome.cpp src/WindowControlButton.cpp src/Context.cpp
    src/ContextStore.cpp src/DockpipeChoices.cpp src/WorkflowCatalog.cpp
    src/LauncherSettings.cpp src/BrowserPreferences.cpp src/BrowserLinkRouter.cpp
    src/CanvasPanel.cpp src/DashboardWorkspace.cpp src/DashboardStore.cpp src/DashboardDock.cpp src/DashboardFilesWidget.cpp
    src/DesktopWindowWidget.cpp src/DesktopApplicationCatalog.cpp src/DesktopBackground.cpp
    src/PackageCommand.cpp src/WorkspacePackageManager.cpp)
if(WIN32)
    list(APPEND DOCKPIPE_UI_SOURCES src/DesktopAppWindowWin.cpp)
else()
    list(APPEND DOCKPIPE_UI_SOURCES src/DesktopAppWindow.cpp)
endif()
add_library(dockpipe-launcher-ui SHARED ${DOCKPIPE_UI_SOURCES})
set_target_properties(dockpipe-launcher-ui PROPERTIES WINDOWS_EXPORT_ALL_SYMBOLS ON VERSION 1.0 SOVERSION 1
    BUILD_RPATH_USE_ORIGIN TRUE)
target_include_directories(dockpipe-launcher-ui PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}/src"
    $<TARGET_PROPERTY:dockpipe-syntax,INTERFACE_INCLUDE_DIRECTORIES>)
target_link_libraries(dockpipe-launcher-ui PUBLIC Qt6::Widgets Qt6::Network PRIVATE dockpipe-syntax Qt6::Concurrent)
if(WIN32)
    target_link_libraries(dockpipe-launcher-ui PRIVATE user32 gdi32 advapi32 shell32 ole32)
elseif(APPLE)
    enable_language(OBJCXX)
    target_sources(dockpipe-launcher-ui PRIVATE src/DesktopBackgroundMac.mm)
    set_target_properties(dockpipe-launcher-ui PROPERTIES OBJCXX_STANDARD 17 OBJCXX_STANDARD_REQUIRED ON)
    set_source_files_properties(src/DesktopBackgroundMac.mm PROPERTIES COMPILE_OPTIONS "-fobjc-arc")
    target_link_libraries(dockpipe-launcher-ui PRIVATE "-framework AppKit")
    find_library(DOCKPIPE_SCREEN_CAPTURE_KIT ScreenCaptureKit)
    if(DOCKPIPE_SCREEN_CAPTURE_KIT)
        enable_language(OBJCXX)
        target_sources(dockpipe-launcher-ui PRIVATE src/MacWindowCapture.h src/MacWindowCapture.mm)
        set_target_properties(dockpipe-launcher-ui PROPERTIES OBJCXX_STANDARD 17 OBJCXX_STANDARD_REQUIRED ON)
        set_source_files_properties(src/MacWindowCapture.mm PROPERTIES COMPILE_OPTIONS "-fobjc-arc")
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
    if(X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_DESKTOP_XCB_LIBRARY)
        target_compile_definitions(dockpipe-launcher-ui PUBLIC DOCKPIPE_X11_BILLBOARDS)
        target_link_libraries(dockpipe-launcher-ui PRIVATE X11::X11 X11::Xcomposite X11::Xrender ${DOCKPIPE_DESKTOP_XCB_LIBRARY})
    endif()
endif()
option(DOCKPIPE_WAYLAND_CAPTURE "Enable consent-based Wayland window sharing through the desktop portal" ON)
if(DOCKPIPE_WAYLAND_CAPTURE AND CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(Qt6 QUIET COMPONENTS DBus)
    if(TARGET Qt6::DBus)
        target_sources(dockpipe-launcher-ui PRIVATE src/PortalWindowCapture.cpp src/DesktopCaptureFrameBuffer.cpp)
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
    if(ui_target STREQUAL "dockpipe-launcher-ui" OR ui_target STREQUAL "dockpipe-build-version")
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
add_library(dockpipe-browser STATIC
    src/BrowserRuntime.cpp src/BrowserWidget.cpp src/BrowserWidgetTools.cpp src/BrowserWidgetPermissions.cpp
    src/BrowserSettingsDialog.cpp
    src/BrowserDownloads.cpp src/BrowserAnnotationDialog.cpp src/BrowserExtensionsDialog.cpp
    src/TerminalSession.cpp src/TerminalHub.cpp src/TerminalWidget.cpp src/TerminalBridge.h src/TerminalPreferences.cpp src/TerminalSettingsDialog.cpp
    src/SendInputWidget.cpp terminal.qrc)
target_include_directories(dockpipe-browser PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}/src")
target_link_libraries(dockpipe-browser PUBLIC Qt6::WebEngineWidgets Qt6::PrintSupport Qt6::WebChannel dockpipe-launcher-ui)
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
    add_executable(terminal-preferences-test tests/TerminalPreferencesTest.cpp)
    target_link_libraries(terminal-preferences-test PRIVATE dockpipe-browser Qt6::Test)
    add_test(NAME terminal-preferences COMMAND terminal-preferences-test)
    set_tests_properties(terminal-preferences PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(terminal-session-test tests/TerminalSessionTest.cpp src/TerminalSession.cpp)
    target_include_directories(terminal-session-test PRIVATE src)
    target_link_libraries(terminal-session-test PRIVATE Qt6::Test Qt6::Core)
    if(APPLE)
        target_link_libraries(terminal-session-test PRIVATE proc)
    endif()
    add_test(NAME terminal-session COMMAND terminal-session-test)
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
        src/DashboardWidgetFactory.cpp src/DashboardWidgetViews.cpp src/DashboardLogWidget.cpp
        src/SessionManager.cpp src/SystemResourceOverview.cpp src/ExecutionMetricChart.cpp)
    target_link_libraries(dashboard-widgets-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-widgets COMMAND dashboard-widgets-test)
    set_tests_properties(dashboard-widgets PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    add_executable(dashboard-files-test tests/DashboardFilesWidgetTest.cpp)
    target_link_libraries(dashboard-files-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-files COMMAND dashboard-files-test)
    set_tests_properties(dashboard-files PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(desktop-background-test tests/DesktopBackgroundTest.cpp)
    target_link_libraries(desktop-background-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME desktop-background COMMAND desktop-background-test)
    set_tests_properties(desktop-background PROPERTIES TIMEOUT 15)

    add_executable(dashboard-tour-test tests/DashboardTourDialogTest.cpp src/DashboardTourDialog.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-tour-test PRIVATE dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-tour COMMAND dashboard-tour-test)
    set_tests_properties(dashboard-tour PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

    add_executable(dashboard-page-test tests/DashboardPageTest.cpp
        src/DashboardPage.cpp src/DashboardSettingsDialog.cpp src/DashboardTourDialog.cpp src/DashboardAppsDialog.cpp
        src/DashboardWidgetFactory.cpp src/DashboardWidgetViews.cpp src/DashboardLogWidget.cpp
        src/SessionManager.cpp src/SystemResourceOverview.cpp src/ExecutionMetricChart.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-page-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test Qt6::Concurrent)
    if(CMAKE_SYSTEM_NAME STREQUAL "Linux" AND X11_Xcomposite_FOUND AND X11_Xrender_FOUND AND DOCKPIPE_DESKTOP_XCB_LIBRARY)
        target_link_libraries(dashboard-page-test PRIVATE X11::X11)
    endif()
    add_test(NAME dashboard-page COMMAND dashboard-page-test)
    set_tests_properties(dashboard-page PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
    add_executable(dashboard-browser-performance-test tests/DashboardBrowserPerformanceTest.cpp
        src/DashboardWidgetFactory.cpp src/DashboardWidgetViews.cpp src/DashboardLogWidget.cpp
        src/SessionManager.cpp src/SystemResourceOverview.cpp src/ExecutionMetricChart.cpp dockpipe-launcher.qrc)
    target_link_libraries(dashboard-browser-performance-test PRIVATE dockpipe-browser dockpipe-launcher-ui Qt6::Test)
    add_test(NAME dashboard-browser-performance COMMAND dashboard-browser-performance-test)
    set_tests_properties(dashboard-browser-performance PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)
    add_executable(desktop-window-widget-test tests/DesktopWindowWidgetTest.cpp)
    target_link_libraries(desktop-window-widget-test PRIVATE dockpipe-launcher-ui)
    add_test(NAME desktop-window-widget COMMAND desktop-window-widget-test)
    set_tests_properties(desktop-window-widget PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)
    if(WIN32)
        add_executable(desktop-app-window-win-test tests/DesktopAppWindowWinTest.cpp)
        target_link_libraries(desktop-app-window-win-test PRIVATE dockpipe-launcher-ui user32)
        add_test(NAME desktop-app-window-win COMMAND desktop-app-window-win-test)
        set_tests_properties(desktop-app-window-win PROPERTIES TIMEOUT 45 SKIP_RETURN_CODE 77)
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
    set_tests_properties(browser PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 60)
endif()

set_target_properties(dockpipe-launcher PROPERTIES BUILD_RPATH_USE_ORIGIN TRUE INSTALL_RPATH "$ORIGIN;$ORIGIN/../lib/dockpipe")
if(UNIX AND NOT APPLE)
    set_property(TARGET dockpipe-launcher APPEND PROPERTY BUILD_RPATH "$ORIGIN/../lib/dockpipe")
endif()

if(BUILD_TESTING)
    add_library(workspace-package-fixture MODULE tests/WorkspacePackageFixture.cpp)
    target_link_libraries(workspace-package-fixture PRIVATE dockpipe-launcher-ui)
    set_target_properties(workspace-package-fixture PROPERTIES OUTPUT_NAME dockpipe-cockpit
        LIBRARY_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture/ui"
        RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture/ui")
    add_executable(workspace-package-test tests/WorkspacePackageTest.cpp src/SettingsDialog.cpp)
    target_link_libraries(workspace-package-test PRIVATE dockpipe-launcher-ui)
    add_dependencies(workspace-package-test workspace-package-fixture)
    add_test(NAME workspace-package COMMAND workspace-package-test)
    set_tests_properties(workspace-package PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture"
        TIMEOUT 30 SKIP_RETURN_CODE 77)
    add_executable(launcher-settings-test tests/LauncherSettingsTest.cpp src/SettingsDialog.cpp)
    target_link_libraries(launcher-settings-test PRIVATE dockpipe-launcher-ui)
    add_test(NAME launcher-settings COMMAND launcher-settings-test)
    add_dependencies(launcher-settings-test workspace-package-fixture)
    set_tests_properties(launcher-settings PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/workspace-fixture" TIMEOUT 30)
endif()

# Explicit developer integration only. The normal launcher has no dependency on
# the package source tree. Installed builds discover
# the package through Dockpipe's resolver store when the user enables the feature.
set(DOCKPIPE_COCKPIT_SOURCE "" CACHE PATH "Optional Cockpit package source for development/testing")
if(DOCKPIPE_COCKPIT_SOURCE)
    set(DOCKPIPE_LAUNCHER_SOURCE_DIR "${CMAKE_CURRENT_SOURCE_DIR}")
    add_subdirectory("${DOCKPIPE_COCKPIT_SOURCE}" "${CMAKE_CURRENT_BINARY_DIR}/cockpit-package")
    if(BUILD_TESTING)
        set_tests_properties(remote-launcher launcher-settings PROPERTIES ENVIRONMENT
            "QT_QPA_PLATFORM=offscreen;DOCKPIPE_WORKSPACE_PACKAGE=${CMAKE_CURRENT_BINARY_DIR}/cockpit-package/assets;DOCKPIPE_COCKPIT_READER=$<TARGET_FILE:cockpit-reader>")
    endif()
endif()
