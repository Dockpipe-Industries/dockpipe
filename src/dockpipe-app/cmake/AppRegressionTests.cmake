# Focused state/lifecycle regressions require only Qt Core, not the GUI host SDK.
if(NOT BUILD_TESTING)
    return()
endif()

add_executable(app-workspace-files-test
    tests/WorkspaceFilesClientTest.cpp
    src/files/infrastructure/WorkspaceFilesClient.cpp
    src/files/presentation/WorkspaceFileModel.cpp
    src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
target_link_libraries(app-workspace-files-test PRIVATE Qt6::Core Qt6::Test)
add_test(NAME app-workspace-files COMMAND app-workspace-files-test)
set_tests_properties(app-workspace-files PROPERTIES TIMEOUT 30)

add_executable(app-session-start-failure-test
    tests/SessionManagerStartFailureTest.cpp
    src/execution/infrastructure/SessionManager.cpp
    src/execution/domain/Context.cpp)
target_include_directories(app-session-start-failure-test PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}/src/execution/infrastructure"
    "${CMAKE_CURRENT_SOURCE_DIR}/src/execution/domain"
    "${CMAKE_CURRENT_SOURCE_DIR}/src/catalog/infrastructure")
target_link_libraries(app-session-start-failure-test PRIVATE Qt6::Core)
add_test(NAME app-session-start-failure COMMAND app-session-start-failure-test)

add_executable(app-session-async-test
    tests/SessionManagerTest.cpp
    src/execution/infrastructure/SessionManager.cpp
    src/execution/domain/Context.cpp)
target_include_directories(app-session-async-test PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}/src/execution/infrastructure"
    "${CMAKE_CURRENT_SOURCE_DIR}/src/execution/domain"
    "${CMAKE_CURRENT_SOURCE_DIR}/src/catalog/infrastructure")
target_link_libraries(app-session-async-test PRIVATE Qt6::Core Qt6::Test)
add_test(NAME app-session-async COMMAND app-session-async-test)
set_tests_properties(app-session-async PROPERTIES TIMEOUT 30)

add_executable(app-remote-command-lifecycle-test
    tests/RemoteCommandLifecycleTest.cpp
    src/remote/infrastructure/RemoteCommand.cpp)
target_include_directories(app-remote-command-lifecycle-test PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}/src/remote/infrastructure"
    "${CMAKE_CURRENT_SOURCE_DIR}/src/host-sdk/contracts")
target_link_libraries(app-remote-command-lifecycle-test PRIVATE Qt6::Core)
add_test(NAME app-remote-command-lifecycle COMMAND app-remote-command-lifecycle-test)
set_tests_properties(app-remote-command-lifecycle PROPERTIES TIMEOUT 10 SKIP_RETURN_CODE 77)

# Linux's XDG_CONFIG_HOME provides a deterministic temporary settings location.
# This permission-failure fixture must run as an unprivileged user.
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    add_executable(app-context-store-persistence-test
        tests/ContextStorePersistenceTest.cpp
        src/execution/infrastructure/ContextStore.cpp
        src/execution/domain/Context.cpp)
    target_include_directories(app-context-store-persistence-test PRIVATE
        "${CMAKE_CURRENT_SOURCE_DIR}/src/execution/infrastructure"
        "${CMAKE_CURRENT_SOURCE_DIR}/src/execution/domain")
    target_link_libraries(app-context-store-persistence-test PRIVATE Qt6::Core)
    add_test(NAME app-context-store-persistence COMMAND app-context-store-persistence-test)
endif()

add_executable(app-remote-feature-test
    tests/RemoteFeatureClientTest.cpp
    src/remote/infrastructure/RemoteFeatureClient.cpp
    src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
target_link_libraries(app-remote-feature-test PRIVATE Qt6::Core Qt6::Test)
add_test(NAME app-remote-feature COMMAND app-remote-feature-test)
set_tests_properties(app-remote-feature PROPERTIES TIMEOUT 30)

add_executable(app-project-git-remote-test
    tests/ProjectGitClientRemoteTest.cpp
    src/git/infrastructure/ProjectGitClient.cpp
    src/git/infrastructure/RemoteGitContext.cpp
    src/remote/infrastructure/RemoteFeatureClient.cpp
    src/remote/infrastructure/RemoteCommand.cpp
    src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
target_link_libraries(app-project-git-remote-test PRIVATE Qt6::Core Qt6::Test)
add_test(NAME app-project-git-remote COMMAND app-project-git-remote-test)
set_tests_properties(app-project-git-remote PROPERTIES TIMEOUT 45)

add_executable(app-remote-git-context-test
    tests/RemoteGitContextTest.cpp
    src/git/presentation/ProjectGitWidget.cpp
    src/git/infrastructure/RemoteGitContextWidget.cpp)
target_link_libraries(app-remote-git-context-test PRIVATE dockpipe-launcher-ui Qt6::Widgets Qt6::Test)
add_test(NAME app-remote-git-context COMMAND app-remote-git-context-test)
set_tests_properties(app-remote-git-context PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)

add_executable(app-workspace-machine-menu-test
    tests/WorkspaceMachineMenuTest.cpp
    src/remote/presentation/WorkspaceMachineMenu.cpp)
target_link_libraries(app-workspace-machine-menu-test PRIVATE Qt6::Widgets Qt6::Test)
add_test(NAME app-workspace-machine-menu COMMAND app-workspace-machine-menu-test)
set_tests_properties(app-workspace-machine-menu PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

add_executable(app-remote-terminal-session-test
    tests/RemoteTerminalSessionTest.cpp
    src/terminal/application/TerminalHub.cpp
    src/terminal/infrastructure/TerminalShellCatalog.cpp
    ${DOCKPIPE_TERMINAL_SOURCES}
    src/terminal/infrastructure/ManagedTerminalSession.cpp
    src/terminal/infrastructure/RemoteTerminalSession.cpp
    src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
target_link_libraries(app-remote-terminal-session-test PRIVATE Qt6::Core Qt6::Test)
if(APPLE)
    target_link_libraries(app-remote-terminal-session-test PRIVATE proc)
endif()
add_test(NAME app-remote-terminal-session COMMAND app-remote-terminal-session-test)
set_tests_properties(app-remote-terminal-session PROPERTIES TIMEOUT 30)

add_executable(app-remote-terminal-recovery-test
    tests/RemoteTerminalRecoveryDialogTest.cpp)
target_link_libraries(app-remote-terminal-recovery-test PRIVATE dockpipe-launcher-ui Qt6::Widgets Qt6::Test)
add_test(NAME app-remote-terminal-recovery COMMAND app-remote-terminal-recovery-test)
set_tests_properties(app-remote-terminal-recovery PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 30)

add_executable(app-docker-remote-feature-test
    tests/DockerRemoteFeatureTest.cpp
    src/observability/presentation/DockerObservabilityWidget.cpp
    src/observability/presentation/DockerRemoteMonitoring.cpp
    src/observability/presentation/DockerResourceInspector.cpp
    src/observability/presentation/ResourceCanvas.cpp
    src/observability/presentation/ResourceLinkItem.cpp
    src/shell/infrastructure/SetupTerminal.cpp)
target_link_libraries(app-docker-remote-feature-test PRIVATE dockpipe-launcher-ui Qt6::Widgets Qt6::Concurrent Qt6::Test)
add_test(NAME app-docker-remote-feature COMMAND app-docker-remote-feature-test)
set_tests_properties(app-docker-remote-feature PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen" TIMEOUT 45)

add_executable(app-native-terminal-provider-test
    tests/NativeTerminalProviderTest.cpp
    src/terminal/infrastructure/NativeTerminalLaunch.cpp
    src/terminal/infrastructure/TerminalHostLaunch.cpp)
target_compile_definitions(app-native-terminal-provider-test PRIVATE
    DOCKPIPE_TERMINAL_HOST_TEST_BINARY="$<TARGET_FILE:dockpipe-terminal-host>")
target_link_libraries(app-native-terminal-provider-test PRIVATE Qt6::Core Qt6::Test)
add_dependencies(app-native-terminal-provider-test dockpipe-terminal-host)
add_test(NAME app-native-terminal-provider COMMAND app-native-terminal-provider-test)
set_tests_properties(app-native-terminal-provider PROPERTIES TIMEOUT 45)

add_executable(app-managed-terminal-session-test
    tests/ManagedTerminalSessionTest.cpp
    src/terminal/infrastructure/ManagedTerminalSession.cpp
    src/terminal/infrastructure/NativeTerminalLaunch.cpp
    src/terminal/infrastructure/TerminalHostLaunch.cpp
    src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
target_link_libraries(app-managed-terminal-session-test PRIVATE Qt6::Core Qt6::Test)
add_test(NAME app-managed-terminal-session COMMAND app-managed-terminal-session-test)
set_tests_properties(app-managed-terminal-session PROPERTIES TIMEOUT 30)

add_executable(app-remote-workspace-files-test
    tests/RemoteWorkspaceFilesDispatchTest.cpp
    src/files/infrastructure/WorkspaceFilesClient.cpp
    src/remote/infrastructure/RemoteWorkspaceFilesDispatch.cpp
    src/remote/infrastructure/RemoteFeatureClient.cpp
    src/catalog/infrastructure/OperatorDockpipeBinary.cpp)
target_link_libraries(app-remote-workspace-files-test PRIVATE Qt6::Core Qt6::Test)
add_test(NAME app-remote-workspace-files COMMAND app-remote-workspace-files-test)
set_tests_properties(app-remote-workspace-files PROPERTIES TIMEOUT 30)
