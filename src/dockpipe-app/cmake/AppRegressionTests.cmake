# Focused state/lifecycle regressions require only Qt Core, not the GUI host SDK.
if(NOT BUILD_TESTING)
    return()
endif()

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
