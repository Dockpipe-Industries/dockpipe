cmake_minimum_required(VERSION 3.21.1)
project(dockpipe-launcher VERSION 0.1.0 LANGUAGES CXX)
set(DOCKPIPE_RELEASE_VERSION "${PROJECT_VERSION}" CACHE STRING "Desktop release version")

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

# Qt6Gui depends on WrapOpenGL; without Mesa dev packages, find_package(Qt6) fails obscurely on Linux.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    find_package(OpenGL REQUIRED)
endif()

find_package(Qt6 REQUIRED COMPONENTS Widgets Network Concurrent)

set(DOCKPIPE_LAUNCHER_SOURCES
    src/main.cpp
    src/LauncherEnvironment.cpp
    src/SingleInstanceGuard.cpp
    src/Theme.cpp
    src/Context.cpp
    src/ContextStore.cpp
    src/GitHelper.cpp
    src/SessionManager.cpp
    src/DockpipeChoices.cpp
    src/ContextDiscovery.cpp
    src/WorkflowCatalog.cpp
    src/LauncherSettings.cpp
    src/BasicModeWidget.cpp
    src/PackageCommand.cpp
    src/PackageManagerDialog.cpp
    src/SettingsDialog.cpp
    src/LogViewerDialog.cpp
    src/PromptDialog.cpp
    src/WorkflowLaunchDialog.cpp
    src/DockerObservabilityWidget.cpp
    src/ContextRowWidget.cpp
    src/EditContextDialog.cpp
    src/MainWindow.cpp
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
        MACOSX_BUNDLE_BUNDLE_NAME DockPipe
        MACOSX_BUNDLE_BUNDLE_VERSION "${DOCKPIPE_RELEASE_VERSION}"
        MACOSX_BUNDLE_SHORT_VERSION_STRING "${DOCKPIPE_RELEASE_VERSION}"
        MACOSX_BUNDLE_ICON_FILE DockPipe.icns
    )
endif()

target_include_directories(dockpipe-launcher PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
target_link_libraries(dockpipe-launcher PRIVATE Qt6::Widgets Qt6::Network Qt6::Concurrent)

if(MSVC)
    target_compile_definitions(dockpipe-launcher PRIVATE _CRT_SECURE_NO_WARNINGS)
endif()

include(CTest)
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
        src/DockpipeChoices.cpp src/WorkflowCatalog.cpp src/LauncherSettings.cpp
        src/ContextStore.cpp src/Context.cpp)
    target_include_directories(package-manager-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/src)
    target_link_libraries(package-manager-test PRIVATE Qt6::Widgets)
    add_test(NAME package-manager COMMAND package-manager-test)
    set_tests_properties(package-manager PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen")
endif()
