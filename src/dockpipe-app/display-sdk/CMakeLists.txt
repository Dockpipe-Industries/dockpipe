cmake_minimum_required(VERSION 3.22)
project(DockPipeDisplaySDK VERSION 1.0.0 LANGUAGES CXX)
find_package(Qt6 6.10.3 EXACT REQUIRED COMPONENTS Widgets)
include(CMakePackageConfigHelpers)

# This leaf SDK exports only the core-owned native QWidget interface. It neither
# configures the desktop host nor weakens the full LauncherSDK dependencies.
add_library(dockpipe-display-headers INTERFACE)
set_target_properties(dockpipe-display-headers PROPERTIES EXPORT_NAME DisplayHeaders)
target_compile_features(dockpipe-display-headers INTERFACE cxx_std_17)
target_link_libraries(dockpipe-display-headers INTERFACE Qt6::Widgets)
target_include_directories(dockpipe-display-headers INTERFACE
    "$<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/../src/host-sdk/contracts>"
    "$<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/../src/packages/infrastructure>"
    "$<INSTALL_INTERFACE:include/dockpipe-launcher>")
set(interface_header "${CMAKE_CURRENT_SOURCE_DIR}/../src/host-sdk/contracts/LauncherDisplayPackage.h")
file(SHA256 "${interface_header}" interface_sha256)
set(native_library_header "${CMAKE_CURRENT_SOURCE_DIR}/../src/packages/infrastructure/LauncherNativeLibraryScope.h")
file(SHA256 "${native_library_header}" native_library_sha256)
install(TARGETS dockpipe-display-headers EXPORT DockPipeDisplaySDKTargets COMPONENT DisplaySDK)
install(FILES "${interface_header}" "${native_library_header}" DESTINATION include/dockpipe-launcher COMPONENT DisplaySDK)
set(config_directory "lib/cmake/DockPipeDisplaySDK")
configure_package_config_file(DockPipeDisplaySDKConfig.cmake.in
    "${CMAKE_CURRENT_BINARY_DIR}/DockPipeDisplaySDKConfig.cmake"
    INSTALL_DESTINATION "${config_directory}")
write_basic_package_version_file("${CMAKE_CURRENT_BINARY_DIR}/DockPipeDisplaySDKConfigVersion.cmake"
    VERSION "${PROJECT_VERSION}" COMPATIBILITY ExactVersion)
install(FILES "${CMAKE_CURRENT_BINARY_DIR}/DockPipeDisplaySDKConfig.cmake"
    "${CMAKE_CURRENT_BINARY_DIR}/DockPipeDisplaySDKConfigVersion.cmake"
    DESTINATION "${config_directory}" COMPONENT DisplaySDK)
install(EXPORT DockPipeDisplaySDKTargets NAMESPACE DockPipe::
    DESTINATION "${config_directory}" COMPONENT DisplaySDK)
configure_file(display-sdk-candidate.json.in display-sdk-candidate.json @ONLY)
install(FILES "${CMAKE_CURRENT_BINARY_DIR}/display-sdk-candidate.json"
    DESTINATION share/DockPipeDisplaySDK COMPONENT DisplaySDK)
